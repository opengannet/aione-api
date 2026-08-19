package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	projecti18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/flyte2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateModelDeploymentSettingsAPIKeySemantics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(request.URL.Path, "/context") {
			_, _ = w.Write([]byte(`{"status":200,"data":{"org":"org","project":"aione","domain":"development"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":200,"data":{"items":[],"total":0,"page":1,"pageSize":1}}`))
	}))
	defer server.Close()
	for _, test := range []struct {
		name        string
		extra       string
		enabled     bool
		expectedKey string
	}{
		{name: "preserve omitted key", expectedKey: "saved-key"},
		{name: "preserve empty key", extra: `,"api_key":""`, expectedKey: "saved-key"},
		{name: "replace key", enabled: true, extra: `,"api_key":"replacement-key"`, expectedKey: "replacement-key"},
		{name: "explicitly clear key", extra: `,"clear_api_key":true`, expectedKey: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			setupDeploymentControllerTest(t, flyteDeploymentSettings{
				BaseURL: server.URL + "/v2", Project: "aione", APIKey: "saved-key", Configured: true,
			})
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			payload := fmt.Sprintf(`{"enabled":%t,"base_url":%q,"project":"aione"%s}`, test.enabled, server.URL+"/v2", test.extra)
			context.Request = httptest.NewRequest(http.MethodPut, "/api/deployments/settings", strings.NewReader(payload))
			context.Request.Header.Set("Content-Type", "application/json")

			UpdateModelDeploymentSettings(context)

			var body struct {
				Success bool `json:"success"`
				Data    struct {
					Configured bool `json:"configured"`
				} `json:"data"`
			}
			require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
			assert.True(t, body.Success)
			assert.Equal(t, test.expectedKey != "", body.Data.Configured)
			assert.NotContains(t, response.Body.String(), "saved-key")
			assert.NotContains(t, response.Body.String(), "replacement-key")

			settings := readFlyteDeploymentSettings()
			assert.Equal(t, test.expectedKey, settings.APIKey)
			var option model.Option
			require.NoError(t, model.DB.First(&option, "key = ?", flyteAPIKeyKey).Error)
			assert.Equal(t, test.expectedKey, option.Value)
		})
	}
}

func TestFlyte2ConnectionAcceptsZeroModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "aione", request.URL.Query().Get("project"))
		assert.Contains(t, flyteDeploymentDomains, request.URL.Query().Get("domain"))
		assert.Equal(t, "connection-key", request.Header.Get("X-API-Key"))
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(request.URL.Path, "/context") {
			_, _ = w.Write([]byte(`{"status":200,"data":{"org":"org","project":"aione","domain":"development"}}`))
		} else {
			assert.Equal(t, "/v2/api/aione/models", request.URL.Path)
			_, _ = w.Write([]byte(`{"status":200,"data":{"items":[],"total":0,"page":1,"pageSize":1}}`))
		}
	}))
	defer server.Close()
	setupDeploymentControllerTest(t, flyteDeploymentSettings{
		BaseURL: server.URL + "/v2", Project: "aione", APIKey: "connection-key", Configured: true,
	})
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/deployments/settings/test-connection", strings.NewReader(`{}`))

	TestFlyte2Connection(context)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Connected   bool           `json:"connected"`
			ModelCounts map[string]int `json:"model_counts"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.True(t, body.Data.Connected)
	assert.Equal(t, map[string]int{"development": 0, "production": 0, "staging": 0}, body.Data.ModelCounts)
	assert.NotContains(t, response.Body.String(), "connection-key")
}

func TestDeploymentProxyLifecycle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "proxy-key", request.Header.Get("X-API-Key"))
		w.Header().Set("Content-Type", "application/json")
		if request.URL.Path != "/v2/api/aione/model/run" {
			assert.Equal(t, "aione", request.URL.Query().Get("project"))
			assert.Equal(t, "development", request.URL.Query().Get("domain"))
		}
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/v2/api/aione/models":
			assert.Equal(t, "qwen", request.URL.Query().Get("keyword"))
			assert.Equal(t, "ACTIVE", request.URL.Query().Get("status"))
			_, _ = w.Write([]byte(`{"status":200,"data":{"items":[],"total":0,"page":2,"pageSize":10}}`))
		case request.Method == http.MethodPost && request.URL.Path == "/v2/api/aione/model/run":
			var payload flyte2.CreateModelRequest
			require.NoError(t, common.DecodeJson(request.Body, &payload))
			assert.Equal(t, "aione", payload.Project)
			assert.Equal(t, "development", payload.Domain)
			assert.Equal(t, "VLLM", payload.Profile)
			assert.Equal(t, "source-token", payload.Codes[0].Token)
			_, _ = w.Write([]byte(`{"status":200,"data":{"id":"model-a","type":"VLLM"}}`))
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/log"):
			assert.Equal(t, "2", request.URL.Query().Get("page"))
			assert.Equal(t, "50", request.URL.Query().Get("size"))
			_, _ = w.Write([]byte(`{"status":200,"data":{"items":[],"total":0,"page":2,"size":50}}`))
		case request.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"status":200,"data":{"id":"model-a","config":{"codes":[{"id":"repo","tokenConfigured":true}]}}}`))
		case request.Method == http.MethodDelete:
			_, _ = w.Write([]byte(`{"status":200,"data":{}}`))
		default:
			_, _ = w.Write([]byte(`{"status":200,"data":{"id":"model-a","type":"VLLM"}}`))
		}
	}))
	defer server.Close()
	setupDeploymentControllerTest(t, flyteDeploymentSettings{
		Enabled: true, BaseURL: server.URL + "/v2", Project: "aione", APIKey: "proxy-key", Configured: true,
	})

	engine := gin.New()
	engine.GET("/api/deployments/", GetAllDeployments)
	engine.POST("/api/deployments/", CreateDeployment)
	engine.GET("/api/deployments/:id", GetDeployment)
	engine.PUT("/api/deployments/:id", UpdateDeployment)
	engine.DELETE("/api/deployments/:id", DeleteDeployment)
	engine.POST("/api/deployments/:id/start", StartDeployment)
	engine.POST("/api/deployments/:id/stop", StopDeployment)
	engine.GET("/api/deployments/:id/logs", GetDeploymentLogs)

	requests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/deployments/?domain=development&p=2&page_size=10&keyword=qwen&status=ACTIVE"},
		{method: http.MethodPost, path: "/api/deployments/", body: `{"name":"Model A","id":"model-a","code":"qwen","project":"wrong","domain":"development","modelCacheSize":"1Gi","codes":[{"id":"repo","token":"source-token"}]}`},
		{method: http.MethodGet, path: "/api/deployments/model-a?domain=development"},
		{method: http.MethodPut, path: "/api/deployments/model-a?domain=development", body: `{"name":"Edited","image":"vllm","modelCacheSize":"2Gi","resourceDefinition":{"cpu":"1","memory":"1Gi","gpu":0}}`},
		{method: http.MethodPost, path: "/api/deployments/model-a/start?domain=development"},
		{method: http.MethodPost, path: "/api/deployments/model-a/stop?domain=development"},
		{method: http.MethodGet, path: "/api/deployments/model-a/logs?domain=development&page=2&size=50"},
		{method: http.MethodDelete, path: "/api/deployments/model-a?domain=development"},
	}
	for _, item := range requests {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(item.method, item.path, strings.NewReader(item.body))
		if item.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		engine.ServeHTTP(response, request)
		assert.Contains(t, response.Body.String(), `"success":true`, item.method+" "+item.path)
		assert.NotContains(t, response.Body.String(), "proxy-key")
		assert.NotContains(t, response.Body.String(), "source-token")
	}
}

func TestDeploymentProxyUnavailableDoesNotLeakAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := server.URL + "/v2"
	server.Close()
	setupDeploymentControllerTest(t, flyteDeploymentSettings{
		Enabled: true, BaseURL: baseURL, Project: "aione", APIKey: "unavailable-key", Configured: true,
	})
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/deployments/?domain=development", nil)

	GetAllDeployments(context)

	assert.Contains(t, response.Body.String(), `"success":false`)
	assert.Contains(t, response.Body.String(), "Flyte2 is unavailable")
	assert.NotContains(t, response.Body.String(), "unavailable-key")
}

func TestDeploymentDomainValidation(t *testing.T) {
	require.NoError(t, projecti18n.Init())
	setupDeploymentControllerTest(t, flyteDeploymentSettings{
		Enabled: true, BaseURL: "https://flyte.example/v2", Project: "aione", APIKey: "domain-key", Configured: true,
	})
	engine := gin.New()
	engine.GET("/api/deployments/", GetAllDeployments)
	engine.POST("/api/deployments/", CreateDeployment)

	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/api/deployments/", nil),
		httptest.NewRequest(http.MethodGet, "/api/deployments/?domain=custom", nil),
		httptest.NewRequest(http.MethodPost, "/api/deployments/", strings.NewReader(`{"id":"model-a"}`)),
		httptest.NewRequest(http.MethodPost, "/api/deployments/", strings.NewReader(`{"id":"model-a","domain":"custom"}`)),
	} {
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), `"success":false`)
		assert.Contains(t, response.Body.String(), "development")
	}
}

func TestDeploymentDomainsAreForwardedToFlyte2(t *testing.T) {
	forwarded := make([]string, 0, len(flyteDeploymentDomains))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		forwarded = append(forwarded, request.URL.Query().Get("domain"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":200,"data":{"items":[],"total":0,"page":1,"pageSize":10}}`))
	}))
	defer server.Close()
	setupDeploymentControllerTest(t, flyteDeploymentSettings{
		Enabled: true, BaseURL: server.URL + "/v2", Project: "aione", APIKey: "domain-key", Configured: true,
	})
	engine := gin.New()
	engine.GET("/api/deployments/", GetAllDeployments)

	for _, domain := range flyteDeploymentDomains {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/deployments/?domain="+domain, nil)
		engine.ServeHTTP(response, request)
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"success":true`)
	}
	assert.Equal(t, flyteDeploymentDomains, forwarded)
}

func setupDeploymentControllerTest(t *testing.T, settings flyteDeploymentSettings) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	database, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&model.Option{}))
	previousDB := model.DB
	common.OptionMapRWMutex.Lock()
	previousOptions := common.OptionMap
	common.OptionMap = map[string]string{
		flyteEnabledKey:                  strconv.FormatBool(settings.Enabled),
		flyteBaseURLKey:                  settings.BaseURL,
		flyteProjectKey:                  settings.Project,
		"model_deployment.flyte2.domain": "development",
		flyteAPIKeyKey:                   settings.APIKey,
		flytePublicationEnabledKey:       strconv.FormatBool(settings.PublicationEnabled),
	}
	common.OptionMapRWMutex.Unlock()
	model.DB = database
	t.Cleanup(func() {
		model.DB = previousDB
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
	})
}
