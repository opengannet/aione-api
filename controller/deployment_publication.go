package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
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
	AccessGroup        string          `json:"access_group"`
	IdempotencyKey     string          `json:"idempotency_key"`
	UpstreamModel      string          `json:"upstream_model"`
	DeprecatedTokenIDs json.RawMessage `json:"token_ids"`
	DeprecatedNewToken json.RawMessage `json:"new_token"`
}

type updateUpstreamModelRequest struct {
	UpstreamModel string `json:"upstream_model"`
}

func GetDeploymentPublication(c *gin.Context) {
	_, settings, domain, deploymentID, ok := deploymentRequest(c)
	if !ok {
		return
	}
	publication, err := model.GetFlytePublication(settings.BaseURL, settings.Project, domain, deploymentID)
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
	if len(request.DeprecatedTokenIDs) > 0 || len(request.DeprecatedNewToken) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "API keys must be created and managed separately from deployment publication"})
		return
	}
	if len(strings.TrimSpace(request.IdempotencyKey)) == 0 || len(request.IdempotencyKey) > 128 {
		common.ApiErrorMsg(c, "idempotency_key is required and must not exceed 128 characters")
		return
	}
	client, settings, domain, deploymentID, ok := deploymentRequest(c)
	if !ok {
		return
	}
	detail, err := client.GetModel(c.Request.Context(), deploymentID, settings.Project, domain)
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
	contextResult, err := client.GetContext(c.Request.Context(), settings.Project, domain)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	phase, reason, endpoint, upstreamModel, verifyError := evaluateFlytePublication(c.Request.Context(), detail, request.UpstreamModel)
	publication, err := model.PublishFlyteDeployment(model.FlytePublicationMutation{
		BaseURL: settings.BaseURL, Organization: contextResult.Org, Project: settings.Project, Domain: domain,
		AccessGroup: strings.TrimSpace(request.AccessGroup), DeploymentID: deploymentID, ModelCode: modelCode,
		Endpoint: endpoint, UpstreamModel: upstreamModel, Phase: phase, ReasonCode: reason,
		LastError: errorText(verifyError), CreatedBy: c.GetInt("id"), IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
		RequestedUpstreamModel: strings.TrimSpace(request.UpstreamModel),
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
	recordManageAudit(c, "deployment.publication.publish", map[string]any{
		"deployment_id":  deploymentID,
		"domain":         domain,
		"publication_id": publication.ID,
	})
	common.ApiSuccess(c, response)
}

func DeleteDeploymentPublication(c *gin.Context) {
	_, settings, domain, deploymentID, ok := deploymentRequest(c)
	if !ok {
		return
	}
	publication, err := model.GetFlytePublication(settings.BaseURL, settings.Project, domain, deploymentID)
	if err == nil {
		err = model.UnpublishFlyteDeployment(publication.ID)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "deployment.publication.delete", map[string]any{"deployment_id": deploymentID, "domain": domain})
	common.ApiSuccess(c, gin.H{})
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
	client, settings, domain, deploymentID, ok := deploymentRequest(c)
	if !ok {
		return nil
	}
	publication, err := model.GetFlytePublication(settings.BaseURL, settings.Project, domain, deploymentID)
	if err != nil {
		return err
	}
	contextResult, err := client.GetContext(c.Request.Context(), settings.Project, domain)
	if err != nil {
		return err
	}
	if contextResult.Org != publication.Gateway.Organization {
		return fmt.Errorf("Flyte2 organization changed; publication reconciliation is frozen")
	}
	detail, err := client.GetModel(c.Request.Context(), deploymentID, settings.Project, domain)
	if err != nil {
		return err
	}
	phase, reason, endpoint, upstream, verifyErr := evaluateFlytePublication(c.Request.Context(), detail, firstNonEmpty(requestedUpstream, publication.UpstreamModel))
	updated, err := model.UpdateFlytePublicationRoute(publication.ID, phase, reason, endpoint, upstream, errorText(verifyErr))
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

func reconcileAllFlytePublications(ctx context.Context, force bool) (int, int, error) {
	settings := readFlyteDeploymentSettings()
	if settings.BaseURL == "" || settings.Project == "" || settings.APIKey == "" {
		return 0, 0, nil
	}
	publications, err := model.ListFlytePublications()
	if err != nil || len(publications) == 0 {
		return 0, 0, err
	}
	type reconciliationScope struct {
		BaseURL      string
		Organization string
		Project      string
		Domain       string
	}
	byScope := make(map[reconciliationScope][]model.FlytePublicationView)
	scopes := make([]reconciliationScope, 0)
	for _, publication := range publications {
		scope := reconciliationScope{
			BaseURL:      publication.Gateway.BaseURL,
			Organization: publication.Gateway.Organization,
			Project:      publication.Gateway.Project,
			Domain:       publication.Gateway.Domain,
		}
		if _, exists := byScope[scope]; !exists {
			scopes = append(scopes, scope)
		}
		byScope[scope] = append(byScope[scope], publication)
	}
	sort.Slice(scopes, func(left, right int) bool {
		leftKey := scopes[left].BaseURL + "\x00" + scopes[left].Organization + "\x00" + scopes[left].Project + "\x00" + scopes[left].Domain
		rightKey := scopes[right].BaseURL + "\x00" + scopes[right].Organization + "\x00" + scopes[right].Project + "\x00" + scopes[right].Domain
		return leftKey < rightKey
	})
	reconciled, unpublished := 0, 0
	now := common.GetTimestamp()
	reconciliationErrors := make([]error, 0)
	for _, scope := range scopes {
		scopePublications := byScope[scope]
		gateway := scopePublications[0].Gateway
		client, clientErr := flyte2.NewClient(gateway.BaseURL, settings.APIKey)
		if clientErr != nil {
			reconciliationErrors = append(reconciliationErrors, fmt.Errorf("Flyte2 %s domain client initialization failed: %w", gateway.Domain, clientErr))
			continue
		}
		contextResult, contextErr := client.GetContext(ctx, gateway.Project, gateway.Domain)
		if contextErr != nil {
			reconciliationErrors = append(reconciliationErrors, fmt.Errorf("Flyte2 %s domain context failed: %w", gateway.Domain, contextErr))
			continue
		}
		if gateway.Organization != contextResult.Org {
			reconciliationErrors = append(reconciliationErrors, fmt.Errorf("Flyte2 %s domain organization changed; automatic reconciliation is frozen", gateway.Domain))
			continue
		}

		allModels := make(map[string]flyte2.ModelSummary)
		page := 1
		expectedTotal := -1
		listFailed := false
		for {
			result, listErr := client.ListModels(ctx, gateway.Project, gateway.Domain, "", "", page, 100)
			if listErr != nil {
				reconciliationErrors = append(reconciliationErrors, fmt.Errorf("Flyte2 %s domain model listing failed: %w", gateway.Domain, listErr))
				listFailed = true
				break
			}
			if expectedTotal < 0 {
				expectedTotal = result.Total
			} else if result.Total != expectedTotal {
				reconciliationErrors = append(reconciliationErrors, fmt.Errorf("Flyte2 %s domain model list changed during pagination", gateway.Domain))
				listFailed = true
				break
			}
			for _, item := range result.Items {
				if _, duplicate := allModels[item.ID]; duplicate {
					reconciliationErrors = append(reconciliationErrors, fmt.Errorf("Flyte2 %s domain returned duplicate models during pagination", gateway.Domain))
					listFailed = true
					break
				}
				allModels[item.ID] = item
			}
			if listFailed || len(allModels) == expectedTotal {
				break
			}
			if len(result.Items) == 0 || len(allModels) > expectedTotal {
				reconciliationErrors = append(reconciliationErrors, fmt.Errorf("Flyte2 %s domain returned an incomplete model list", gateway.Domain))
				listFailed = true
				break
			}
			page++
		}
		if listFailed {
			continue
		}

		for _, publication := range scopePublications {
			if !force && publication.NextRetryAt > now {
				continue
			}
			if _, exists := allModels[publication.DeploymentID]; !exists {
				removed, markErr := model.MarkFlytePublicationMissing(publication.ID)
				if markErr != nil {
					reconciliationErrors = append(reconciliationErrors, fmt.Errorf("Flyte2 %s domain publication %s missing-state update failed: %w", gateway.Domain, publication.DeploymentID, markErr))
					continue
				}
				if removed {
					unpublished++
				}
				continue
			}
			detail, detailErr := client.GetModel(ctx, publication.DeploymentID, gateway.Project, gateway.Domain)
			if detailErr != nil {
				reconciliationErrors = append(reconciliationErrors, fmt.Errorf("Flyte2 %s domain deployment %s lookup failed: %w", gateway.Domain, publication.DeploymentID, detailErr))
				continue
			}
			phase, reason, endpoint, upstream, verifyErr := evaluateFlytePublication(ctx, detail, publication.UpstreamModel)
			if _, updateErr := model.UpdateFlytePublicationRoute(publication.ID, phase, reason, endpoint, upstream, errorText(verifyErr)); updateErr != nil {
				reconciliationErrors = append(reconciliationErrors, fmt.Errorf("Flyte2 %s domain deployment %s reconciliation failed: %w", gateway.Domain, publication.DeploymentID, updateErr))
				continue
			}
			reconciled++
		}
	}
	return reconciled, unpublished, errors.Join(reconciliationErrors...)
}

func publicationResponse(publication *model.FlytePublicationView) gin.H {
	return gin.H{"id": publication.ID, "deployment_id": publication.DeploymentID, "model_code": publication.ModelCode, "endpoint": publication.Endpoint, "upstream_model": publication.UpstreamModel, "phase": publication.Phase, "reason_code": publication.ReasonCode, "last_error": publication.LastError, "access_group": publication.Gateway.AccessGroup, "organization": publication.Gateway.Organization, "project": publication.Gateway.Project, "domain": publication.Gateway.Domain, "channel_id": publication.Gateway.ChannelID, "pricing_configured": relayhelper.HasModelBillingConfig(publication.ModelCode)}
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
