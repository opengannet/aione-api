package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/flyte2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
