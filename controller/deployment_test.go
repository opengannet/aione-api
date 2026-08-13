package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
				BaseURL: server.URL + "/v2", Project: "aione", Domain: "development", APIKey: "saved-key", Configured: true,
			})
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)
			payload := fmt.Sprintf(`{"enabled":%t,"base_url":%q,"project":"aione","domain":"development"%s}`, test.enabled, server.URL+"/v2", test.extra)
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
		assert.Equal(t, "development", request.URL.Query().Get("domain"))
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
		BaseURL: server.URL + "/v2", Project: "aione", Domain: "development", APIKey: "connection-key", Configured: true,
	})
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/deployments/settings/test-connection", strings.NewReader(`{}`))

	TestFlyte2Connection(context)

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Connected  bool `json:"connected"`
			ModelCount int  `json:"model_count"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &body))
	assert.True(t, body.Success)
	assert.True(t, body.Data.Connected)
	assert.Zero(t, body.Data.ModelCount)
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
		Enabled: true, BaseURL: server.URL + "/v2", Project: "aione", Domain: "development", APIKey: "proxy-key", Configured: true,
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
		{method: http.MethodGet, path: "/api/deployments/?p=2&page_size=10&keyword=qwen&status=ACTIVE"},
		{method: http.MethodPost, path: "/api/deployments/", body: `{"name":"Model A","id":"model-a","code":"qwen","project":"wrong","domain":"wrong","modelCacheSize":"1Gi","codes":[{"id":"repo","token":"source-token"}]}`},
		{method: http.MethodGet, path: "/api/deployments/model-a"},
		{method: http.MethodPut, path: "/api/deployments/model-a", body: `{"name":"Edited","image":"vllm","modelCacheSize":"2Gi","resourceDefinition":{"cpu":"1","memory":"1Gi","gpu":0}}`},
		{method: http.MethodPost, path: "/api/deployments/model-a/start"},
		{method: http.MethodPost, path: "/api/deployments/model-a/stop"},
		{method: http.MethodGet, path: "/api/deployments/model-a/logs?page=2&size=50"},
		{method: http.MethodDelete, path: "/api/deployments/model-a"},
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
		Enabled: true, BaseURL: baseURL, Project: "aione", Domain: "development", APIKey: "unavailable-key", Configured: true,
	})
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/deployments/", nil)

	GetAllDeployments(context)

	assert.Contains(t, response.Body.String(), `"success":false`)
	assert.Contains(t, response.Body.String(), "Flyte2 is unavailable")
	assert.NotContains(t, response.Body.String(), "unavailable-key")
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
		flyteEnabledKey:            strconv.FormatBool(settings.Enabled),
		flyteBaseURLKey:            settings.BaseURL,
		flyteProjectKey:            settings.Project,
		flyteDomainKey:             settings.Domain,
		flyteAPIKeyKey:             settings.APIKey,
		flytePublicationEnabledKey: strconv.FormatBool(settings.PublicationEnabled),
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
