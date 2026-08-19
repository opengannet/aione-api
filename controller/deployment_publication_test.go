package controller

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/flyte2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReconcileAllFlytePublicationsIsolatesDeploymentDomains(t *testing.T) {
	listPages := map[string][]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		domain := request.URL.Query().Get("domain")
		if domain == "production" {
			http.Error(w, "production unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(request.URL.Path, "/context"):
			_, _ = fmt.Fprintf(w, `{"status":200,"data":{"org":"org","project":"aione","domain":%q}}`, domain)
		case strings.HasSuffix(request.URL.Path, "/models"):
			page := request.URL.Query().Get("p")
			listPages[domain] = append(listPages[domain], page)
			deploymentID := domain + "-other-" + page
			if domain == "development" && page == "1" {
				deploymentID = "shared-id"
			}
			_, _ = fmt.Fprintf(w, `{"status":200,"data":{"items":[{"id":%q,"project":"aione","domain":%q,"code":%q,"type":"VLLM","deploymentStatus":4,"currentReplicas":0}],"total":2,"page":%s,"pageSize":100}}`, deploymentID, domain, domain+"-model", page)
		default:
			_, _ = w.Write([]byte(`{"status":200,"data":{"id":"shared-id","project":"aione","domain":"development","code":"development-model","type":"VLLM","deploymentStatus":4,"currentReplicas":0,"config":{}}}`))
		}
	}))
	defer server.Close()
	setupDeploymentControllerTest(t, flyteDeploymentSettings{
		Enabled: true, BaseURL: server.URL + "/v2", Project: "aione", APIKey: "reconcile-key", Configured: true,
	})
	require.NoError(t, model.DB.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Token{}, &model.FlyteGateway{}, &model.FlytePublication{}, &model.FlytePublicationBinding{}))
	previousCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = previousCache })
	token := model.Token{UserId: 1, Key: "reconcile-token", Name: "reconcile", Group: "aione", ModelLimitsEnabled: true}
	require.NoError(t, model.DB.Create(&token).Error)
	for _, publication := range []struct {
		domain    string
		modelCode string
	}{
		{domain: "development", modelCode: "development-model"},
		{domain: "production", modelCode: "production-model"},
		{domain: "staging", modelCode: "staging-model"},
	} {
		_, _, err := model.PublishFlyteDeployment(model.FlytePublicationMutation{
			BaseURL: server.URL + "/v2", Organization: "org", Project: "aione", Domain: publication.domain,
			AccessGroup: "aione", DeploymentID: "shared-id", ModelCode: publication.modelCode,
			Phase: model.FlytePublicationPhasePending, TokenIDs: []int{token.Id}, IdempotencyKey: publication.domain,
		})
		require.NoError(t, err)
	}

	reconciled, unpublished, err := reconcileAllFlytePublications(context.Background(), true)
	assert.Equal(t, 1, reconciled)
	assert.Zero(t, unpublished)
	require.ErrorContains(t, err, "production")
	assert.Equal(t, []string{"1", "2"}, listPages["development"])
	assert.Equal(t, []string{"1", "2"}, listPages["staging"])
	development, lookupErr := model.GetFlytePublication(server.URL+"/v2", "aione", "development", "shared-id")
	require.NoError(t, lookupErr)
	assert.Equal(t, "deployment_not_running", development.ReasonCode)
	production, lookupErr := model.GetFlytePublication(server.URL+"/v2", "aione", "production", "shared-id")
	require.NoError(t, lookupErr)
	assert.Empty(t, production.ReasonCode)
	staging, lookupErr := model.GetFlytePublication(server.URL+"/v2", "aione", "staging", "shared-id")
	require.NoError(t, lookupErr)
	assert.Equal(t, "deployment_missing", staging.ReasonCode)
}

func TestEvaluateFlytePublicationSelectsExactAndSingleUpstreamModelsWithoutForwardingSecrets(t *testing.T) {
	models := []string{"alias", "served"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/v1/models", request.URL.Path)
		assert.Empty(t, request.Header.Get("Authorization"))
		assert.Empty(t, request.Header.Get("X-API-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"` + models[0] + `"},{"id":"` + models[1] + `"}]}`))
	}))
	defer server.Close()

	detail := &flyte2.ModelDetail{ModelSummary: flyte2.ModelSummary{Code: "alias", URL: server.URL, DeploymentStatus: 7}}
	phase, reason, endpoint, upstream, err := evaluateFlytePublication(context.Background(), detail, "")
	require.NoError(t, err)
	assert.Equal(t, "published", phase)
	assert.Empty(t, reason)
	assert.Equal(t, server.URL, endpoint)
	assert.Equal(t, "alias", upstream)

	detail.Code = "public-alias"
	phase, reason, _, upstream, err = evaluateFlytePublication(context.Background(), detail, "")
	require.ErrorContains(t, err, "available upstream models: alias, served")
	assert.Equal(t, "pending", phase)
	assert.Equal(t, "upstream_model_required", reason)
	assert.Empty(t, upstream)

	models = []string{"only-served-model", ""}
	phase, _, _, upstream, err = evaluateFlytePublication(context.Background(), detail, "")
	require.NoError(t, err)
	assert.Equal(t, "published", phase)
	assert.Equal(t, "only-served-model", upstream)
}

func TestEvaluateFlytePublicationRequiresActiveDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"alias"}]}`))
	}))
	defer server.Close()

	for _, status := range []int{0, 1, 2, 3, 4, 5, 6, 8, 9, 10} {
		detail := &flyte2.ModelDetail{ModelSummary: flyte2.ModelSummary{Code: "alias", URL: server.URL, DeploymentStatus: status}}
		phase, reason, endpoint, upstream, err := evaluateFlytePublication(context.Background(), detail, "")
		require.NoError(t, err)
		assert.Equal(t, model.FlytePublicationPhasePending, phase)
		assert.Equal(t, "deployment_not_running", reason)
		assert.Empty(t, endpoint)
		assert.Empty(t, upstream)
	}
}

func TestNormalizeModelEndpointRejectsCredentialsQueryAndFragment(t *testing.T) {
	for _, value := range []string{"ftp://model.example", "https://user:model@example.com", "https://model.example?key=secret", "https://model.example#fragment"} {
		_, err := normalizeModelEndpoint(value)
		assert.Error(t, err)
	}
}
