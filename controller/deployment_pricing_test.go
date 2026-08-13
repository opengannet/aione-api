package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	projecti18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeploymentPricingLifecycle(t *testing.T) {
	require.NoError(t, projecti18n.Init())
	server := deploymentPricingFlyteServer(t, "flyte-price-model")
	defer server.Close()
	setupDeploymentControllerTest(t, flyteDeploymentSettings{
		Enabled: true, BaseURL: server.URL + "/v2", Project: "aione", Domain: "development", APIKey: "pricing-key", Configured: true,
	})
	restoreDeploymentPricingAfterTest(t)

	modelPrice := ratio_setting.GetModelPriceCopy()
	modelRatio := ratio_setting.GetModelRatioCopy()
	completionRatio := ratio_setting.GetCompletionRatioCopy()
	modelPrice["unrelated-model"] = 9.5
	values, err := marshalDeploymentPricingOptions(
		modelPrice, modelRatio, completionRatio,
		ratio_setting.GetCacheRatioCopy(), ratio_setting.GetCreateCacheRatioCopy(),
		ratio_setting.GetImageRatioCopy(), ratio_setting.GetAudioRatioCopy(),
		ratio_setting.GetAudioCompletionRatioCopy(), billing_setting.GetBillingModeCopy(), billing_setting.GetBillingExprCopy(),
	)
	require.NoError(t, err)
	require.NoError(t, model.UpdateOptionsBulk(values))

	engine := gin.New()
	engine.GET("/api/deployments/:id/pricing", GetDeploymentPricing)
	engine.PUT("/api/deployments/:id/pricing", UpdateDeploymentPricing)

	response := performDeploymentPricingRequest(engine, http.MethodGet, "")
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"configured":false`)
	assert.Contains(t, response.Body.String(), `"model_code":"flyte-price-model"`)

	for _, test := range []struct {
		name           string
		payload        string
		expectedMode   string
		expectedPrice  *float64
		expectedRatio  *float64
		expectedOutput *float64
	}{
		{name: "free", payload: `{"mode":" free "}`, expectedMode: "free", expectedPrice: float64Pointer(0)},
		{name: "per request", payload: `{"mode":"per_request","request_price":0.02}`, expectedMode: "per_request", expectedPrice: float64Pointer(0.02)},
		{name: "per token", payload: `{"mode":"per_token","input_price":2,"output_price":8}`, expectedMode: "per_token", expectedRatio: float64Pointer(1), expectedOutput: float64Pointer(4)},
	} {
		t.Run(test.name, func(t *testing.T) {
			response = performDeploymentPricingRequest(engine, http.MethodPut, test.payload)
			assert.Equal(t, http.StatusOK, response.Code)
			assert.Contains(t, response.Body.String(), `"success":true`)
			assert.Contains(t, response.Body.String(), `"mode":"`+test.expectedMode+`"`)

			price, hasPrice := ratio_setting.GetModelPriceCopy()["flyte-price-model"]
			if test.expectedPrice == nil {
				assert.False(t, hasPrice)
			} else {
				assert.True(t, hasPrice)
				assert.Equal(t, *test.expectedPrice, price)
			}
			ratio, hasRatio := ratio_setting.GetModelRatioCopy()["flyte-price-model"]
			completion, hasCompletion := ratio_setting.GetCompletionRatioCopy()["flyte-price-model"]
			if test.expectedRatio == nil {
				assert.False(t, hasRatio)
				assert.False(t, hasCompletion)
			} else {
				assert.True(t, hasRatio)
				assert.True(t, hasCompletion)
				assert.Equal(t, *test.expectedRatio, ratio)
				assert.Equal(t, *test.expectedOutput, completion)
			}
			assert.Equal(t, 9.5, ratio_setting.GetModelPriceCopy()["unrelated-model"])
			assert.True(t, relayhelper.HasModelBillingConfig("flyte-price-model"))
		})
	}
}

func TestDeploymentPricingRejectsInvalidAndAdvancedConfiguration(t *testing.T) {
	require.NoError(t, projecti18n.Init())
	server := deploymentPricingFlyteServer(t, "flyte-advanced-model")
	defer server.Close()
	setupDeploymentControllerTest(t, flyteDeploymentSettings{
		Enabled: true, BaseURL: server.URL + "/v2", Project: "aione", Domain: "development", APIKey: "pricing-key", Configured: true,
	})
	restoreDeploymentPricingAfterTest(t)

	engine := gin.New()
	engine.PUT("/api/deployments/:id/pricing", UpdateDeploymentPricing)
	for _, payload := range []string{
		`{"mode":"per_token","input_price":0,"output_price":1}`,
		`{"mode":"per_token","input_price":1,"output_price":-1}`,
		`{"mode":"per_request","request_price":-1}`,
		`{"mode":"unknown"}`,
	} {
		response := performDeploymentPricingRequest(engine, http.MethodPut, payload)
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"success":false`)
	}

	cacheRatio := ratio_setting.GetCacheRatioCopy()
	cacheRatio["flyte-advanced-model"] = 0.5
	values, err := marshalDeploymentPricingOptions(
		ratio_setting.GetModelPriceCopy(), ratio_setting.GetModelRatioCopy(), ratio_setting.GetCompletionRatioCopy(),
		cacheRatio, ratio_setting.GetCreateCacheRatioCopy(), ratio_setting.GetImageRatioCopy(),
		ratio_setting.GetAudioRatioCopy(), ratio_setting.GetAudioCompletionRatioCopy(),
		billing_setting.GetBillingModeCopy(), billing_setting.GetBillingExprCopy(),
	)
	require.NoError(t, err)
	require.NoError(t, model.UpdateOptionsBulk(values))

	response := performDeploymentPricingRequest(engine, http.MethodPut, `{"mode":"free"}`)
	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Contains(t, response.Body.String(), `"success":false`)
	assert.Equal(t, 0.5, ratio_setting.GetCacheRatioCopy()["flyte-advanced-model"])
}

func deploymentPricingFlyteServer(t *testing.T, modelCode string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "pricing-key", request.Header.Get("X-API-Key"))
		assert.Equal(t, "aione", request.URL.Query().Get("project"))
		assert.Equal(t, "development", request.URL.Query().Get("domain"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":200,"data":{"id":"deployment-price","code":"` + modelCode + `","config":{"code":"` + modelCode + `"}}}`))
	}))
}

func performDeploymentPricingRequest(engine *gin.Engine, method string, payload string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(method, "/api/deployments/deployment-price/pricing", strings.NewReader(payload))
	if payload != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	engine.ServeHTTP(response, request)
	return response
}

func restoreDeploymentPricingAfterTest(t *testing.T) {
	t.Helper()
	values, err := marshalDeploymentPricingOptions(
		ratio_setting.GetModelPriceCopy(), ratio_setting.GetModelRatioCopy(), ratio_setting.GetCompletionRatioCopy(),
		ratio_setting.GetCacheRatioCopy(), ratio_setting.GetCreateCacheRatioCopy(), ratio_setting.GetImageRatioCopy(),
		ratio_setting.GetAudioRatioCopy(), ratio_setting.GetAudioCompletionRatioCopy(),
		billing_setting.GetBillingModeCopy(), billing_setting.GetBillingExprCopy(),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, model.UpdateOptionsBulk(values))
	})
}

func float64Pointer(value float64) *float64 {
	return &value
}
