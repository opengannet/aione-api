package controller

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/flyte2"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	flyteEnabledKey            = "model_deployment.flyte2.enabled"
	flyteBaseURLKey            = "model_deployment.flyte2.base_url"
	flyteProjectKey            = "model_deployment.flyte2.project"
	flyteDomainKey             = "model_deployment.flyte2.domain"
	flyteAPIKeyKey             = "model_deployment.flyte2.api_key"
	flytePublicationEnabledKey = "model_deployment.flyte2.publication_enabled"
)

type flyteDeploymentSettings struct {
	Enabled            bool   `json:"enabled"`
	BaseURL            string `json:"base_url"`
	Project            string `json:"project"`
	Domain             string `json:"domain"`
	APIKey             string `json:"-"`
	Configured         bool   `json:"configured"`
	PublicationEnabled bool   `json:"publication_enabled"`
}

type updateFlyteDeploymentSettingsRequest struct {
	Enabled            *bool  `json:"enabled"`
	BaseURL            string `json:"base_url"`
	Project            string `json:"project"`
	Domain             string `json:"domain"`
	APIKey             string `json:"api_key"`
	ClearAPIKey        bool   `json:"clear_api_key"`
	PublicationEnabled *bool  `json:"publication_enabled"`
}

func readFlyteDeploymentSettings() flyteDeploymentSettings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	settings := flyteDeploymentSettings{
		Enabled:            common.OptionMap[flyteEnabledKey] == "true",
		BaseURL:            strings.TrimSpace(common.OptionMap[flyteBaseURLKey]),
		Project:            strings.TrimSpace(common.OptionMap[flyteProjectKey]),
		Domain:             strings.TrimSpace(common.OptionMap[flyteDomainKey]),
		APIKey:             strings.TrimSpace(common.OptionMap[flyteAPIKeyKey]),
		PublicationEnabled: common.OptionMap[flytePublicationEnabledKey] == "true",
	}
	settings.Configured = settings.APIKey != ""
	return settings
}

func GetModelDeploymentSettings(c *gin.Context) {
	settings := readFlyteDeploymentSettings()
	common.ApiSuccess(c, gin.H{
		"provider":            "flyte2",
		"enabled":             settings.Enabled,
		"base_url":            settings.BaseURL,
		"project":             settings.Project,
		"domain":              settings.Domain,
		"configured":          settings.Configured,
		"publication_enabled": settings.PublicationEnabled,
	})
}

func UpdateModelDeploymentSettings(c *gin.Context) {
	var request updateFlyteDeploymentSettingsRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgDeploymentInvalidPayload)
		return
	}
	current := readFlyteDeploymentSettings()
	enabled := current.Enabled
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	baseURL := strings.TrimSpace(request.BaseURL)
	project := strings.TrimSpace(request.Project)
	domain := strings.TrimSpace(request.Domain)
	apiKey := current.APIKey
	publicationEnabled := current.PublicationEnabled
	if request.PublicationEnabled != nil {
		publicationEnabled = *request.PublicationEnabled
	}
	if request.ClearAPIKey {
		apiKey = ""
	} else if strings.TrimSpace(request.APIKey) != "" {
		apiKey = strings.TrimSpace(request.APIKey)
	}
	if baseURL != "" {
		normalizedBaseURL, err := flyte2.NormalizeBaseURL(baseURL)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		baseURL = normalizedBaseURL
	}
	if enabled && (baseURL == "" || project == "" || domain == "" || apiKey == "") {
		common.ApiErrorI18n(c, i18n.MsgDeploymentSettingsRequired)
		return
	}
	if current.BaseURL != "" && (baseURL != current.BaseURL || project != current.Project || domain != current.Domain) {
		hasRecords, err := model.HasFlytePublicationRecords()
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if hasRecords {
			common.ApiErrorI18n(c, i18n.MsgDeploymentScopeLocked)
			return
		}
	}
	if request.ClearAPIKey && enabled {
		common.ApiErrorI18n(c, i18n.MsgDeploymentKeyClearDisabled)
		return
	}
	if apiKey != current.APIKey && apiKey != "" {
		candidate, err := flyte2.NewClient(baseURL, apiKey)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if _, err = candidate.GetContext(c.Request.Context(), project, domain); err != nil {
			common.ApiError(c, err)
			return
		}
		if _, err = candidate.ListModels(c.Request.Context(), project, domain, "", "", 1, 1); err != nil {
			common.ApiError(c, err)
			return
		}
	}
	if err := model.UpdateOptionsBulk(map[string]string{
		flyteEnabledKey:            strconv.FormatBool(enabled),
		flyteBaseURLKey:            baseURL,
		flyteProjectKey:            project,
		flyteDomainKey:             domain,
		flyteAPIKeyKey:             apiKey,
		flytePublicationEnabledKey: strconv.FormatBool(publicationEnabled),
	}); err != nil {
		common.ApiErrorI18n(c, i18n.MsgDeploymentSaveFailed)
		return
	}
	GetModelDeploymentSettings(c)
}

func TestFlyte2Connection(c *gin.Context) {
	var request updateFlyteDeploymentSettingsRequest
	if c.Request.ContentLength != 0 {
		if err := common.DecodeJson(c.Request.Body, &request); err != nil {
			common.ApiErrorI18n(c, i18n.MsgDeploymentInvalidPayload)
			return
		}
	}
	settings := readFlyteDeploymentSettings()
	if value := strings.TrimSpace(request.BaseURL); value != "" {
		settings.BaseURL = value
	}
	if value := strings.TrimSpace(request.Project); value != "" {
		settings.Project = value
	}
	if value := strings.TrimSpace(request.Domain); value != "" {
		settings.Domain = value
	}
	if value := strings.TrimSpace(request.APIKey); value != "" {
		settings.APIKey = value
	}
	if settings.BaseURL == "" || settings.Project == "" || settings.Domain == "" || settings.APIKey == "" {
		common.ApiErrorI18n(c, i18n.MsgDeploymentSettingsMissing)
		return
	}
	client, err := newFlyteClient(settings, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	contextResult, err := client.GetContext(c.Request.Context(), settings.Project, settings.Domain)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := client.ListModels(c.Request.Context(), settings.Project, settings.Domain, "", "", 1, 1)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"connected": true, "model_count": result.Total, "org": contextResult.Org})
}

func GetAllDeployments(c *gin.Context) {
	client, settings, ok := deploymentClient(c)
	if !ok {
		return
	}
	page, ok := positiveQueryInt(c, "p", 1, 0)
	if !ok {
		return
	}
	pageSize, ok := positiveQueryInt(c, "page_size", 20, 100)
	if !ok {
		return
	}
	result, err := client.ListModels(
		c.Request.Context(), settings.Project, settings.Domain,
		strings.TrimSpace(c.Query("keyword")), strings.TrimSpace(c.Query("status")), page, pageSize,
	)
	if err == nil {
		for index := range result.Items {
			attachDeploymentPublication(&result.Items[index])
		}
	}
	respondDeployment(c, result, err)
}

func CreateDeployment(c *gin.Context) {
	client, settings, ok := deploymentClient(c)
	if !ok {
		return
	}
	var request flyte2.CreateModelRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgDeploymentInvalidPayload)
		return
	}
	request.Project = settings.Project
	request.Domain = settings.Domain
	request.Profile = "VLLM"
	result, err := client.CreateModel(c.Request.Context(), request)
	respondDeployment(c, result, err)
}

func GetDeployment(c *gin.Context) {
	client, settings, id, ok := deploymentRequest(c)
	if !ok {
		return
	}
	result, err := client.GetModel(c.Request.Context(), id, settings.Project, settings.Domain)
	if err == nil {
		attachDeploymentPublication(&result.ModelSummary)
	}
	respondDeployment(c, result, err)
}

func UpdateDeployment(c *gin.Context) {
	client, settings, id, ok := deploymentRequest(c)
	if !ok {
		return
	}
	var request flyte2.UpdateModelRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiErrorI18n(c, i18n.MsgDeploymentInvalidPayload)
		return
	}
	result, err := client.UpdateModel(c.Request.Context(), id, settings.Project, settings.Domain, request)
	respondDeployment(c, result, err)
}

func StartDeployment(c *gin.Context) {
	client, settings, id, ok := deploymentRequest(c)
	if !ok {
		return
	}
	result, err := client.StartModel(c.Request.Context(), id, settings.Project, settings.Domain)
	respondDeployment(c, result, err)
}

func StopDeployment(c *gin.Context) {
	client, settings, id, ok := deploymentRequest(c)
	if !ok {
		return
	}
	result, err := client.StopModel(c.Request.Context(), id, settings.Project, settings.Domain)
	respondDeployment(c, result, err)
}

func DeleteDeployment(c *gin.Context) {
	client, settings, id, ok := deploymentRequest(c)
	if !ok {
		return
	}
	if err := client.DeleteModel(c.Request.Context(), id, settings.Project, settings.Domain); err != nil {
		common.ApiError(c, err)
		return
	}
	cleanupPending := false
	if _, err := model.GetFlytePublication(id); err == nil {
		if err := model.UnpublishFlyteDeployment(id); err != nil {
			cleanupPending = true
			if markErr := model.MarkFlytePublicationCleanupPending(id, "local publication cleanup failed"); markErr != nil {
				common.ApiError(c, markErr)
				return
			}
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"cleanup_pending": cleanupPending})
}

func GetDeploymentLogs(c *gin.Context) {
	client, settings, id, ok := deploymentRequest(c)
	if !ok {
		return
	}
	page, ok := positiveQueryInt(c, "page", 1, 0)
	if !ok {
		return
	}
	size, ok := positiveQueryInt(c, "size", 200, 1000)
	if !ok {
		return
	}
	result, err := client.GetModelLogs(c.Request.Context(), id, settings.Project, settings.Domain, page, size)
	respondDeployment(c, result, err)
}

func deploymentClient(c *gin.Context) (*flyte2.Client, flyteDeploymentSettings, bool) {
	settings := readFlyteDeploymentSettings()
	if !settings.Enabled {
		common.ApiErrorI18n(c, i18n.MsgDeploymentNotEnabled)
		return nil, settings, false
	}
	if settings.BaseURL == "" || settings.Project == "" || settings.Domain == "" || settings.APIKey == "" {
		common.ApiErrorI18n(c, i18n.MsgDeploymentSettingsMissing)
		return nil, settings, false
	}
	client, err := newFlyteClient(settings, true)
	if err != nil {
		common.ApiError(c, err)
		return nil, settings, false
	}
	return client, settings, true
}

func deploymentRequest(c *gin.Context) (*flyte2.Client, flyteDeploymentSettings, string, bool) {
	client, settings, ok := deploymentClient(c)
	if !ok {
		return nil, settings, "", false
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		common.ApiErrorI18n(c, i18n.MsgDeploymentIdRequired)
		return nil, settings, "", false
	}
	return client, settings, id, true
}

func newFlyteClient(settings flyteDeploymentSettings, requireEnabled bool) (*flyte2.Client, error) {
	if requireEnabled && !settings.Enabled {
		return nil, errors.New("Flyte2 model deployment is not enabled")
	}
	if settings.BaseURL == "" || settings.Project == "" || settings.Domain == "" || settings.APIKey == "" {
		return nil, errors.New("Flyte2 deployment settings are incomplete")
	}
	return flyte2.NewClient(settings.BaseURL, settings.APIKey)
}

func positiveQueryInt(c *gin.Context, key string, fallback, maximum int) (int, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || (maximum > 0 && value > maximum) {
		common.ApiErrorMsg(c, key+" must be a valid positive integer")
		return 0, false
	}
	return value, true
}

func respondDeployment(c *gin.Context, data any, err error) {
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, data)
}

func attachDeploymentPublication(summary *flyte2.ModelSummary) {
	publication, err := model.GetFlytePublication(summary.ID)
	if err == nil {
		summary.Publication = publicationResponse(publication)
	}
}
