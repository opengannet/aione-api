package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/flyte2"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const upstreamModelsResponseLimit = 1 << 20

const flyteDeploymentStatusActive = 7

type publishDeploymentRequest struct {
	AccessGroup    string               `json:"access_group"`
	TokenIDs       []int                `json:"token_ids"`
	NewToken       *newPublicationToken `json:"new_token"`
	IdempotencyKey string               `json:"idempotency_key"`
	UpstreamModel  string               `json:"upstream_model"`
}

type newPublicationToken struct {
	UserID             int     `json:"user_id"`
	Name               string  `json:"name"`
	ExpiredTime        int64   `json:"expired_time"`
	RemainQuota        int     `json:"remain_quota"`
	UnlimitedQuota     bool    `json:"unlimited_quota"`
	ModelLimitsEnabled bool    `json:"model_limits_enabled"`
	AllowIPs           *string `json:"allow_ips"`
	CrossGroupRetry    bool    `json:"cross_group_retry"`
}

type addPublicationBindingsRequest struct {
	TokenIDs       []int  `json:"token_ids"`
	IdempotencyKey string `json:"idempotency_key"`
}

type updateUpstreamModelRequest struct {
	UpstreamModel string `json:"upstream_model"`
}

func GetDeploymentPublication(c *gin.Context) {
	publication, err := model.GetFlytePublication(strings.TrimSpace(c.Param("id")))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiSuccess(c, nil)
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, publicationResponse(publication))
}

func PublishDeployment(c *gin.Context) {
	settings := readFlyteDeploymentSettings()
	if !settings.PublicationEnabled {
		common.ApiErrorI18n(c, i18n.MsgDeploymentPublicationDisabled)
		return
	}
	var request publishDeploymentRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(strings.TrimSpace(request.IdempotencyKey)) == 0 || len(request.IdempotencyKey) > 128 {
		common.ApiErrorMsg(c, "idempotency_key is required and must not exceed 128 characters")
		return
	}
	client, settings, deploymentID, ok := deploymentRequest(c)
	if !ok {
		return
	}
	detail, err := client.GetModel(c.Request.Context(), deploymentID, settings.Project, settings.Domain)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	modelCode, err := model.ValidateFlyteModelCode(detail.Code)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !relayhelper.HasModelBillingConfig(modelCode) {
		common.ApiErrorI18n(c, i18n.MsgDeploymentPricingRequired)
		return
	}
	contextResult, err := client.GetContext(c.Request.Context(), settings.Project, settings.Domain)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	phase, reason, endpoint, upstreamModel, verifyError := evaluateFlytePublication(c.Request.Context(), detail, request.UpstreamModel)
	var newToken *model.Token
	if request.NewToken != nil {
		if request.NewToken.UserID <= 0 || strings.TrimSpace(request.NewToken.Name) == "" {
			common.ApiErrorMsg(c, "new_token.user_id and name are required")
			return
		}
		if len(strings.TrimSpace(request.NewToken.Name)) > 50 {
			common.ApiErrorMsg(c, "new_token.name must not exceed 50 characters")
			return
		}
		if !request.NewToken.UnlimitedQuota && request.NewToken.RemainQuota < 0 {
			common.ApiErrorMsg(c, "new_token.remain_quota must not be negative")
			return
		}
		maxQuotaValue := int(1000000000 * common.QuotaPerUnit)
		if !request.NewToken.UnlimitedQuota && request.NewToken.RemainQuota > maxQuotaValue {
			common.ApiErrorMsg(c, fmt.Sprintf("new_token.remain_quota must not exceed %d", maxQuotaValue))
			return
		}
		var owner model.User
		if err := model.DB.First(&owner, request.NewToken.UserID).Error; err != nil {
			common.ApiErrorMsg(c, "new_token.user_id does not identify an active user")
			return
		}
		key, keyErr := common.GenerateKey()
		if keyErr != nil {
			common.ApiError(c, keyErr)
			return
		}
		newToken = &model.Token{UserId: request.NewToken.UserID, Name: strings.TrimSpace(request.NewToken.Name), Key: key, Status: common.TokenStatusEnabled, ExpiredTime: request.NewToken.ExpiredTime, RemainQuota: request.NewToken.RemainQuota, UnlimitedQuota: request.NewToken.UnlimitedQuota, ModelLimitsEnabled: request.NewToken.ModelLimitsEnabled, AllowIps: request.NewToken.AllowIPs, CrossGroupRetry: request.NewToken.CrossGroupRetry}
	}
	publication, createdToken, err := model.PublishFlyteDeployment(model.FlytePublicationMutation{
		BaseURL: settings.BaseURL, Organization: contextResult.Org, Project: settings.Project, Domain: settings.Domain,
		AccessGroup: strings.TrimSpace(request.AccessGroup), DeploymentID: deploymentID, ModelCode: modelCode,
		Endpoint: endpoint, UpstreamModel: upstreamModel, Phase: phase, ReasonCode: reason,
		LastError: errorText(verifyError), CreatedBy: c.GetInt("id"), TokenIDs: request.TokenIDs,
		NewToken: newToken, IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
	})
	if err != nil {
		if errors.Is(err, model.ErrFlytePublicationConflict) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
			return
		}
		common.ApiError(c, err)
		return
	}
	response := publicationResponse(publication)
	if createdToken != nil {
		response["created_token"] = gin.H{"id": createdToken.Id, "key": createdToken.GetFullKey(), "name": createdToken.Name}
	}
	if hasUnrestrictedBinding(publication) {
		response["warning"] = "Unrestricted keys in this fixed group can access every published model."
	}
	recordManageAudit(c, "deployment.publication.publish", map[string]any{
		"deployment_id":  deploymentID,
		"publication_id": publication.ID,
		"binding_count":  len(publication.Bindings),
	})
	common.ApiSuccess(c, response)
}

func DeleteDeploymentPublication(c *gin.Context) {
	err := model.UnpublishFlyteDeployment(strings.TrimSpace(c.Param("id")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "deployment.publication.delete", map[string]any{"deployment_id": strings.TrimSpace(c.Param("id"))})
	common.ApiSuccess(c, gin.H{})
}

func AddDeploymentPublicationBindings(c *gin.Context) {
	settings := readFlyteDeploymentSettings()
	if !settings.PublicationEnabled {
		common.ApiErrorI18n(c, i18n.MsgDeploymentPublicationDisabled)
		return
	}
	var request addPublicationBindingsRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(request.TokenIDs) == 0 {
		common.ApiErrorMsg(c, "token_ids is required")
		return
	}
	publication, err := model.AddFlytePublicationBindings(strings.TrimSpace(c.Param("id")), request.TokenIDs, strings.TrimSpace(request.IdempotencyKey))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "deployment.publication.bind", map[string]any{
		"deployment_id": strings.TrimSpace(c.Param("id")),
		"token_ids":     request.TokenIDs,
	})
	common.ApiSuccess(c, publicationResponse(publication))
}

func DeleteDeploymentPublicationBinding(c *gin.Context) {
	tokenID, ok := positivePathInt(c, "token_id")
	if !ok {
		return
	}
	unpublished, err := model.RemoveFlytePublicationBinding(strings.TrimSpace(c.Param("id")), tokenID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "deployment.publication.unbind", map[string]any{
		"deployment_id": strings.TrimSpace(c.Param("id")),
		"token_id":      tokenID,
	})
	common.ApiSuccess(c, gin.H{"unpublished": unpublished})
}

func UpdateDeploymentPublicationUpstreamModel(c *gin.Context) {
	var request updateUpstreamModelRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := reconcileDeploymentPublication(c, strings.TrimSpace(request.UpstreamModel)); err != nil {
		common.ApiError(c, err)
		return
	}
}

func ReconcileDeploymentPublication(c *gin.Context) {
	if err := reconcileDeploymentPublication(c, ""); err != nil {
		common.ApiError(c, err)
		return
	}
}

func ReconcileAllDeploymentPublications(c *gin.Context) {
	if _, _, err := reconcileAllFlytePublications(c.Request.Context(), true); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"reconciled": true})
}

func reconcileDeploymentPublication(c *gin.Context, requestedUpstream string) error {
	client, settings, deploymentID, ok := deploymentRequest(c)
	if !ok {
		return nil
	}
	publication, err := model.GetFlytePublication(deploymentID)
	if err != nil {
		return err
	}
	contextResult, err := client.GetContext(c.Request.Context(), settings.Project, settings.Domain)
	if err != nil {
		return err
	}
	if contextResult.Org != publication.Gateway.Organization {
		return fmt.Errorf("Flyte2 organization changed; publication reconciliation is frozen")
	}
	detail, err := client.GetModel(c.Request.Context(), deploymentID, settings.Project, settings.Domain)
	if err != nil {
		return err
	}
	phase, reason, endpoint, upstream, verifyErr := evaluateFlytePublication(c.Request.Context(), detail, firstNonEmpty(requestedUpstream, publication.UpstreamModel))
	updated, err := model.UpdateFlytePublicationRoute(deploymentID, phase, reason, endpoint, upstream, errorText(verifyErr))
	if err != nil {
		return err
	}
	common.ApiSuccess(c, publicationResponse(updated))
	return nil
}

func evaluateFlytePublication(ctx context.Context, detail *flyte2.ModelDetail, requestedUpstream string) (phase, reason, endpoint, upstream string, err error) {
	if detail.DeploymentStatus != flyteDeploymentStatusActive {
		return model.FlytePublicationPhasePending, "deployment_not_running", "", "", nil
	}
	endpoint, err = normalizeModelEndpoint(detail.URL)
	if err != nil {
		return model.FlytePublicationPhasePending, "endpoint_invalid", "", "", err
	}
	if endpoint == "" {
		return model.FlytePublicationPhasePending, "endpoint_unavailable", "", "", nil
	}
	models, err := fetchUpstreamModels(ctx, endpoint)
	if err != nil {
		return model.FlytePublicationPhasePending, "endpoint_unhealthy", "", "", err
	}
	requestedUpstream = strings.TrimSpace(requestedUpstream)
	if requestedUpstream != "" {
		for _, candidate := range models {
			if candidate == requestedUpstream {
				return model.FlytePublicationPhasePublished, "", endpoint, candidate, nil
			}
		}
		return model.FlytePublicationPhasePending, "upstream_model_invalid", "", "", fmt.Errorf("selected upstream model is not advertised")
	}
	for _, candidate := range models {
		if candidate == detail.Code {
			return model.FlytePublicationPhasePublished, "", endpoint, candidate, nil
		}
	}
	if len(models) == 1 {
		return model.FlytePublicationPhasePublished, "", endpoint, models[0], nil
	}
	if len(models) > 1 {
		return model.FlytePublicationPhasePending, "upstream_model_required", "", "", fmt.Errorf("available upstream models: %s", strings.Join(models, ", "))
	}
	return model.FlytePublicationPhasePending, "upstream_models_empty", "", "", nil
}

func normalizeModelEndpoint(value string) (string, error) {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("model endpoint must be an HTTP or HTTPS URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("model endpoint must not contain credentials, query, or fragment")
	}
	return parsed.String(), nil
}

func fetchUpstreamModels(ctx context.Context, endpoint string) ([]string, error) {
	base, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		if request.URL.Scheme != "http" && request.URL.Scheme != "https" {
			return fmt.Errorf("invalid redirect scheme")
		}
		if !sameOrigin(base, request.URL) {
			return fmt.Errorf("cross-origin redirect is not allowed")
		}
		return nil
	}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("model endpoint is unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("model endpoint returned HTTP %d", response.StatusCode)
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, upstreamModelsResponseLimit+1))
	if readErr != nil {
		return nil, fmt.Errorf("failed to read model endpoint response")
	}
	if len(data) > upstreamModelsResponseLimit {
		return nil, fmt.Errorf("model endpoint response exceeds 1 MiB")
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := common.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("model endpoint returned invalid JSON")
	}
	models := make([]string, 0, len(payload.Data))
	seen := map[string]struct{}{}
	for _, item := range payload.Data {
		value := strings.TrimSpace(item.ID)
		if value != "" {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				models = append(models, value)
			}
		}
	}
	sort.Strings(models)
	return models, nil
}

func sameOrigin(left, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) && strings.EqualFold(left.Host, right.Host)
}

func positivePathInt(c *gin.Context, key string) (int, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(c.Param(key)))
	if err != nil || value <= 0 {
		common.ApiErrorMsg(c, key+" must be a positive integer")
		return 0, false
	}
	return value, true
}

func reconcileAllFlytePublications(ctx context.Context, force bool) (int, int, error) {
	settings := readFlyteDeploymentSettings()
	if settings.BaseURL == "" || settings.Project == "" || settings.Domain == "" || settings.APIKey == "" {
		return 0, 0, nil
	}
	publications, err := model.ListFlytePublications()
	if err != nil || len(publications) == 0 {
		return 0, 0, err
	}
	client, err := flyte2.NewClient(settings.BaseURL, settings.APIKey)
	if err != nil {
		return 0, 0, err
	}
	contextResult, err := client.GetContext(ctx, settings.Project, settings.Domain)
	if err != nil {
		return 0, 0, err
	}
	for _, publication := range publications {
		if publication.Gateway.Organization != contextResult.Org {
			return 0, 0, fmt.Errorf("Flyte2 organization changed; automatic reconciliation is frozen")
		}
	}
	allModels := make(map[string]flyte2.ModelSummary)
	page := 1
	expectedTotal := -1
	for {
		result, listErr := client.ListModels(ctx, settings.Project, settings.Domain, "", "", page, 100)
		if listErr != nil {
			return 0, 0, listErr
		}
		if expectedTotal < 0 {
			expectedTotal = result.Total
		} else if result.Total != expectedTotal {
			return 0, 0, fmt.Errorf("Flyte2 model list changed during pagination")
		}
		for _, item := range result.Items {
			if _, duplicate := allModels[item.ID]; duplicate {
				return 0, 0, fmt.Errorf("Flyte2 returned duplicate models during pagination")
			}
			allModels[item.ID] = item
		}
		if len(allModels) == expectedTotal {
			break
		}
		if len(result.Items) == 0 || len(allModels) > expectedTotal {
			return 0, 0, fmt.Errorf("Flyte2 returned an incomplete model list")
		}
		page++
	}
	reconciled, unpublished := 0, 0
	now := common.GetTimestamp()
	for _, publication := range publications {
		if !force && publication.NextRetryAt > now {
			continue
		}
		if _, exists := allModels[publication.DeploymentID]; !exists {
			removed, markErr := model.MarkFlytePublicationMissing(publication.DeploymentID)
			if markErr != nil {
				return reconciled, unpublished, markErr
			}
			if removed {
				unpublished++
			}
			continue
		}
		detail, detailErr := client.GetModel(ctx, publication.DeploymentID, settings.Project, settings.Domain)
		if detailErr != nil {
			return reconciled, unpublished, detailErr
		}
		phase, reason, endpoint, upstream, verifyErr := evaluateFlytePublication(ctx, detail, publication.UpstreamModel)
		if _, updateErr := model.UpdateFlytePublicationRoute(publication.DeploymentID, phase, reason, endpoint, upstream, errorText(verifyErr)); updateErr != nil {
			return reconciled, unpublished, updateErr
		}
		reconciled++
	}
	return reconciled, unpublished, nil
}

func publicationResponse(publication *model.FlytePublicationView) gin.H {
	response := gin.H{"id": publication.ID, "deployment_id": publication.DeploymentID, "model_code": publication.ModelCode, "endpoint": publication.Endpoint, "upstream_model": publication.UpstreamModel, "phase": publication.Phase, "reason_code": publication.ReasonCode, "last_error": publication.LastError, "access_group": publication.Gateway.AccessGroup, "organization": publication.Gateway.Organization, "channel_id": publication.Gateway.ChannelID, "bindings": publication.Bindings, "pricing_configured": relayhelper.HasModelBillingConfig(publication.ModelCode)}
	if hasUnrestrictedBinding(publication) {
		response["warning"] = "Unrestricted keys in this fixed group can access every published model."
	}
	return response
}

func hasUnrestrictedBinding(publication *model.FlytePublicationView) bool {
	for _, binding := range publication.Bindings {
		if !binding.Restricted {
			return true
		}
	}
	return false
}
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
