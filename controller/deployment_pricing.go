package controller

import (
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

const (
	deploymentPricingModeFree       = "free"
	deploymentPricingModePerToken   = "per_token"
	deploymentPricingModePerRequest = "per_request"
	deploymentPricingModeAdvanced   = "advanced"
)

var deploymentPricingUpdateMu sync.Mutex

type deploymentPricingResponse struct {
	DeploymentID    string   `json:"deployment_id"`
	ModelCode       string   `json:"model_code"`
	Configured      bool     `json:"configured"`
	Mode            string   `json:"mode,omitempty"`
	InputPrice      *float64 `json:"input_price,omitempty"`
	OutputPrice     *float64 `json:"output_price,omitempty"`
	RequestPrice    *float64 `json:"request_price,omitempty"`
	AdvancedOnly    bool     `json:"advanced_only"`
	AdvancedPageURL string   `json:"advanced_page_url"`
}

type updateDeploymentPricingRequest struct {
	Mode         string   `json:"mode"`
	InputPrice   *float64 `json:"input_price"`
	OutputPrice  *float64 `json:"output_price"`
	RequestPrice *float64 `json:"request_price"`
}

func GetDeploymentPricing(c *gin.Context) {
	deploymentID, modelCode, ok := deploymentPricingModel(c)
	if !ok {
		return
	}
	common.ApiSuccess(c, currentDeploymentPricing(deploymentID, modelCode))
}

func UpdateDeploymentPricing(c *gin.Context) {
	var request updateDeploymentPricingRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgDeploymentInvalidPayload)
		return
	}
	request.Mode = strings.TrimSpace(request.Mode)
	deploymentID, modelCode, ok := deploymentPricingModel(c)
	if !ok {
		return
	}
	if currentDeploymentPricing(deploymentID, modelCode).AdvancedOnly {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": i18n.T(c, i18n.MsgDeploymentPricingAdvanced)})
		return
	}
	if !validateDeploymentPricing(c, request) {
		return
	}

	deploymentPricingUpdateMu.Lock()
	defer deploymentPricingUpdateMu.Unlock()

	modelPrice := ratio_setting.GetModelPriceCopy()
	modelRatio := ratio_setting.GetModelRatioCopy()
	completionRatio := ratio_setting.GetCompletionRatioCopy()
	cacheRatio := ratio_setting.GetCacheRatioCopy()
	createCacheRatio := ratio_setting.GetCreateCacheRatioCopy()
	imageRatio := ratio_setting.GetImageRatioCopy()
	audioRatio := ratio_setting.GetAudioRatioCopy()
	audioCompletionRatio := ratio_setting.GetAudioCompletionRatioCopy()
	billingMode := billing_setting.GetBillingModeCopy()
	billingExpr := billing_setting.GetBillingExprCopy()

	delete(modelPrice, modelCode)
	delete(modelRatio, modelCode)
	delete(completionRatio, modelCode)
	delete(cacheRatio, modelCode)
	delete(createCacheRatio, modelCode)
	delete(imageRatio, modelCode)
	delete(audioRatio, modelCode)
	delete(audioCompletionRatio, modelCode)
	delete(billingMode, modelCode)
	delete(billingExpr, modelCode)

	switch request.Mode {
	case deploymentPricingModeFree:
		modelPrice[modelCode] = 0
	case deploymentPricingModePerRequest:
		modelPrice[modelCode] = *request.RequestPrice
	case deploymentPricingModePerToken:
		modelRatio[modelCode] = *request.InputPrice / 2
		completionRatio[modelCode] = *request.OutputPrice / *request.InputPrice
	}

	values, err := marshalDeploymentPricingOptions(modelPrice, modelRatio, completionRatio, cacheRatio, createCacheRatio, imageRatio, audioRatio, audioCompletionRatio, billingMode, billingExpr)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = model.UpdateOptionsBulk(values); err != nil {
		common.ApiErrorI18n(c, i18n.MsgDeploymentPricingSaveFailed)
		return
	}
	common.ApiSuccess(c, currentDeploymentPricing(deploymentID, modelCode))
}

func deploymentPricingModel(c *gin.Context) (string, string, bool) {
	client, settings, domain, deploymentID, ok := deploymentRequest(c)
	if !ok {
		return "", "", false
	}
	detail, err := client.GetModel(c.Request.Context(), deploymentID, settings.Project, domain)
	if err != nil {
		common.ApiError(c, err)
		return "", "", false
	}
	modelCode, err := model.ValidateFlyteModelCode(detail.Code)
	if err != nil {
		common.ApiError(c, err)
		return "", "", false
	}
	return deploymentID, modelCode, true
}

func currentDeploymentPricing(deploymentID string, modelCode string) deploymentPricingResponse {
	response := deploymentPricingResponse{
		DeploymentID: deploymentID, ModelCode: modelCode, Configured: relayhelper.HasModelBillingConfig(modelCode),
		AdvancedPageURL: "/system-settings/billing/model-pricing",
	}
	if billing_setting.GetBillingModeCopy()[modelCode] == billing_setting.BillingModeTieredExpr || hasAdvancedDeploymentPricing(modelCode) {
		response.Mode = deploymentPricingModeAdvanced
		response.AdvancedOnly = true
		return response
	}
	if price, ok := ratio_setting.GetModelPriceCopy()[modelCode]; ok {
		response.Configured = true
		if price == 0 {
			response.Mode = deploymentPricingModeFree
			return response
		}
		response.Mode = deploymentPricingModePerRequest
		response.RequestPrice = &price
		return response
	}
	if ratio, ok := ratio_setting.GetModelRatioCopy()[modelCode]; ok {
		inputPrice := ratio * 2
		outputPrice := inputPrice * ratio_setting.GetCompletionRatio(modelCode)
		response.Configured = true
		response.Mode = deploymentPricingModePerToken
		response.InputPrice = &inputPrice
		response.OutputPrice = &outputPrice
	}
	return response
}

func hasAdvancedDeploymentPricing(modelCode string) bool {
	_, cache := ratio_setting.GetCacheRatioCopy()[modelCode]
	_, createCache := ratio_setting.GetCreateCacheRatioCopy()[modelCode]
	_, image := ratio_setting.GetImageRatioCopy()[modelCode]
	_, audio := ratio_setting.GetAudioRatioCopy()[modelCode]
	_, audioOutput := ratio_setting.GetAudioCompletionRatioCopy()[modelCode]
	return cache || createCache || image || audio || audioOutput
}

func validateDeploymentPricing(c *gin.Context, request updateDeploymentPricingRequest) bool {
	switch strings.TrimSpace(request.Mode) {
	case deploymentPricingModeFree:
		return true
	case deploymentPricingModePerRequest:
		if request.RequestPrice == nil || !validNonNegativePrice(*request.RequestPrice) {
			common.ApiErrorI18n(c, i18n.MsgDeploymentPricingRequestInvalid)
			return false
		}
		return true
	case deploymentPricingModePerToken:
		if request.InputPrice == nil || !validPositivePrice(*request.InputPrice) {
			common.ApiErrorI18n(c, i18n.MsgDeploymentPricingInputInvalid)
			return false
		}
		if request.OutputPrice == nil || !validNonNegativePrice(*request.OutputPrice) {
			common.ApiErrorI18n(c, i18n.MsgDeploymentPricingOutputInvalid)
			return false
		}
		if !validNonNegativePrice(*request.OutputPrice / *request.InputPrice) {
			common.ApiErrorI18n(c, i18n.MsgDeploymentPricingOutputInvalid)
			return false
		}
		return true
	default:
		common.ApiErrorI18n(c, i18n.MsgDeploymentPricingModeInvalid)
		return false
	}
}

func validPositivePrice(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validNonNegativePrice(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func marshalDeploymentPricingOptions(maps ...any) (map[string]string, error) {
	keys := []string{
		"ModelPrice", "ModelRatio", "CompletionRatio", "CacheRatio", "CreateCacheRatio",
		"ImageRatio", "AudioRatio", "AudioCompletionRatio",
		"billing_setting.billing_mode", "billing_setting.billing_expr",
	}
	values := make(map[string]string, len(keys))
	for index, pricingMap := range maps {
		encoded, err := common.Marshal(pricingMap)
		if err != nil {
			return nil, err
		}
		values[keys[index]] = string(encoded)
	}
	return values, nil
}
