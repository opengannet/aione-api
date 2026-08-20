package model

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"gorm.io/gorm"
)

var (
	ErrFlytePublicationConflict  = errors.New("Flyte publication conflict")
	ErrFlytePublicationNotFound  = errors.New("Flyte publication not found")
	flytePublicationMutationLock sync.Mutex
)

const (
	FlytePublicationPhasePending   = "pending"
	FlytePublicationPhasePublished = "published"
	FlytePublicationPhaseDrifted   = "drifted"
	FlytePublicationPhaseCleanup   = "cleanup_pending"
)

type FlyteGateway struct {
	ID           int64  `json:"id" gorm:"primaryKey"`
	ScopeKey     string `json:"-" gorm:"type:char(64);uniqueIndex"`
	BaseURL      string `json:"base_url" gorm:"type:text;not null"`
	Organization string `json:"organization" gorm:"type:varchar(255);not null"`
	Project      string `json:"project" gorm:"type:varchar(255);not null"`
	Domain       string `json:"domain" gorm:"type:varchar(255);not null"`
	AccessGroup  string `json:"access_group" gorm:"type:varchar(64);not null"`
	ChannelID    int    `json:"channel_id" gorm:"uniqueIndex;not null"`
	CreatedBy    int    `json:"created_by"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

type FlytePublication struct {
	ID                 int64   `json:"id" gorm:"primaryKey"`
	GatewayID          int64   `json:"gateway_id" gorm:"index;uniqueIndex:idx_flyte_gateway_deployment"`
	DeploymentID       string  `json:"deployment_id" gorm:"type:varchar(255);uniqueIndex:idx_flyte_gateway_deployment"`
	ModelCode          string  `json:"model_code" gorm:"type:varchar(255);not null"`
	ModelCodeKey       string  `json:"-" gorm:"type:char(64);uniqueIndex"`
	Endpoint           string  `json:"endpoint" gorm:"type:text"`
	UpstreamModel      string  `json:"upstream_model" gorm:"type:varchar(255)"`
	Phase              string  `json:"phase" gorm:"type:varchar(32);index"`
	ReasonCode         string  `json:"reason_code" gorm:"type:varchar(64)"`
	NextRetryAt        int64   `json:"next_retry_at" gorm:"bigint;index"`
	ConsecutiveMissing int     `json:"consecutive_missing"`
	MissingSince       int64   `json:"missing_since" gorm:"bigint"`
	LastSeenAt         int64   `json:"last_seen_at" gorm:"bigint"`
	LastError          string  `json:"last_error" gorm:"type:text"`
	IdempotencyKey     *string `json:"-" gorm:"type:varchar(128);uniqueIndex"`
	IdempotencyHash    string  `json:"-" gorm:"type:char(64)"`
	CreatedAt          int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64   `json:"updated_at" gorm:"bigint"`
}

type FlytePublicationBinding struct {
	ID                     int64  `json:"id" gorm:"primaryKey"`
	PublicationID          int64  `json:"publication_id" gorm:"uniqueIndex:idx_flyte_publication_token"`
	TokenID                int    `json:"token_id" gorm:"uniqueIndex:idx_flyte_publication_token;index"`
	ManagedPermissionAdded bool   `json:"managed_permission_added"`
	IdempotencyKey         string `json:"-" gorm:"type:varchar(128);index"`
	CreatedAt              int64  `json:"created_at" gorm:"bigint"`
}

type FlytePublicationView struct {
	FlytePublication
	Gateway FlyteGateway `json:"gateway" gorm:"-"`
}

type FlytePublicationMutation struct {
	BaseURL                string
	Organization           string
	Project                string
	Domain                 string
	AccessGroup            string
	DeploymentID           string
	ModelCode              string
	Endpoint               string
	UpstreamModel          string
	Phase                  string
	ReasonCode             string
	LastError              string
	CreatedBy              int
	IdempotencyKey         string
	RequestedUpstreamModel string
}

func FlyteScopeKey(baseURL, organization, project, domain string) string {
	return sha256Hex(strings.Join([]string{strings.TrimRight(strings.TrimSpace(baseURL), "/"), strings.TrimSpace(organization), strings.TrimSpace(project), strings.TrimSpace(domain)}, "\x00"))
}

func FlyteModelCodeKey(modelCode string) string { return sha256Hex(strings.TrimSpace(modelCode)) }

func sha256Hex(value string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(value))) }

func ValidateFlyteModelCode(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) == 0 || len(value) > 255 {
		return "", fmt.Errorf("model code must contain 1 to 255 bytes")
	}
	for _, character := range value {
		if character == ',' || character < 0x20 || character == 0x7f {
			return "", fmt.Errorf("model code must not contain commas or control characters")
		}
	}
	return value, nil
}

func HasFlytePublicationRecords() (bool, error) {
	if !flytePublicationTablesReady(DB) {
		return false, nil
	}
	var count int64
	if err := DB.Model(&FlyteGateway{}).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func GetFlytePublication(baseURL, project, domain, deploymentID string) (*FlytePublicationView, error) {
	if !flytePublicationTablesReady(DB) {
		return nil, gorm.ErrRecordNotFound
	}
	var publication FlytePublication
	if err := DB.Table("flyte_publications").
		Select("flyte_publications.*").
		Joins("JOIN flyte_gateways ON flyte_gateways.id = flyte_publications.gateway_id").
		Where("flyte_gateways.base_url = ? AND flyte_gateways.project = ? AND flyte_gateways.domain = ? AND flyte_publications.deployment_id = ?",
			strings.TrimRight(strings.TrimSpace(baseURL), "/"), strings.TrimSpace(project), strings.TrimSpace(domain), strings.TrimSpace(deploymentID)).
		First(&publication).Error; err != nil {
		return nil, err
	}
	return loadFlytePublicationView(DB, publication)
}

func GetFlytePublicationByModelCode(modelCode string) (*FlytePublicationView, error) {
	modelCode, err := ValidateFlyteModelCode(modelCode)
	if err != nil {
		return nil, err
	}
	if !flytePublicationTablesReady(DB) {
		return nil, ErrFlytePublicationNotFound
	}
	var publication FlytePublication
	if err := DB.Where("model_code_key = ?", FlyteModelCodeKey(modelCode)).First(&publication).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFlytePublicationNotFound
		}
		return nil, err
	}
	return loadFlytePublicationView(DB, publication)
}

func ListFlytePublications() ([]FlytePublicationView, error) {
	if !flytePublicationTablesReady(DB) {
		return nil, nil
	}
	var publications []FlytePublication
	if err := DB.Order("id").Find(&publications).Error; err != nil {
		return nil, err
	}
	views := make([]FlytePublicationView, 0, len(publications))
	for _, publication := range publications {
		view, err := loadFlytePublicationView(DB, publication)
		if err != nil {
			return nil, err
		}
		views = append(views, *view)
	}
	return views, nil
}

func UpdateFlytePublicationRoute(publicationID int64, phase, reasonCode, endpoint, upstreamModel, lastError string) (*FlytePublicationView, error) {
	var publication FlytePublication
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&publication, publicationID).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		delay := int64(120)
		if phase == FlytePublicationPhasePending {
			delay = 15
			if publication.NextRetryAt > publication.UpdatedAt {
				delay = (publication.NextRetryAt - publication.UpdatedAt) * 2
				if delay > 300 {
					delay = 300
				}
			}
		}
		updates := map[string]any{"phase": phase, "reason_code": reasonCode, "last_error": lastError, "updated_at": now, "last_seen_at": now, "next_retry_at": now + delay, "consecutive_missing": 0, "missing_since": 0}
		if phase == FlytePublicationPhasePublished && endpoint != "" && upstreamModel != "" {
			updates["endpoint"] = endpoint
			updates["upstream_model"] = upstreamModel
		} else if publication.Endpoint != "" && publication.UpstreamModel != "" {
			updates["phase"] = FlytePublicationPhaseDrifted
		} else {
			updates["endpoint"] = ""
			updates["upstream_model"] = ""
		}
		if err := tx.Model(&publication).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.First(&publication, publication.ID).Error; err != nil {
			return err
		}
		return rebuildFlyteManagedChannel(tx, publication.GatewayID)
	})
	if err != nil {
		return nil, err
	}
	InitChannelCache()
	return loadFlytePublicationView(DB, publication)
}

func MarkFlytePublicationMissing(publicationID int64) (bool, error) {
	unpublished := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var publication FlytePublication
		if err := lockForUpdate(tx).First(&publication, publicationID).Error; err != nil {
			return err
		}
		now := common.GetTimestamp()
		missingSince := publication.MissingSince
		if missingSince == 0 {
			missingSince = now
		}
		missingCount := publication.ConsecutiveMissing + 1
		if missingCount >= 3 && now-missingSince >= 120 {
			if err := tx.Delete(&publication).Error; err != nil {
				return err
			}
			unpublished = true
			return rebuildFlyteManagedChannel(tx, publication.GatewayID)
		}
		return tx.Model(&publication).Updates(map[string]any{"consecutive_missing": missingCount, "missing_since": missingSince, "reason_code": "deployment_missing", "updated_at": now}).Error
	})
	if err == nil && unpublished {
		InitChannelCache()
	}
	return unpublished, err
}

func MarkFlytePublicationCleanupPending(publicationID int64, lastError string) error {
	result := DB.Model(&FlytePublication{}).Where("id = ?", publicationID).Updates(map[string]any{"phase": FlytePublicationPhaseCleanup, "reason_code": "local_cleanup_failed", "last_error": lastError, "next_retry_at": common.GetTimestamp() + 15, "updated_at": common.GetTimestamp()})
	if result.Error == nil && result.RowsAffected > 0 {
		InitChannelCache()
	}
	return result.Error
}

func loadFlytePublicationView(db *gorm.DB, publication FlytePublication) (*FlytePublicationView, error) {
	view := &FlytePublicationView{FlytePublication: publication}
	if err := db.First(&view.Gateway, publication.GatewayID).Error; err != nil {
		return nil, err
	}
	return view, nil
}

func PublishFlyteDeployment(mutation FlytePublicationMutation) (*FlytePublicationView, error) {
	flytePublicationMutationLock.Lock()
	defer flytePublicationMutationLock.Unlock()

	modelCode, err := ValidateFlyteModelCode(mutation.ModelCode)
	if err != nil {
		return nil, err
	}
	accessGroup := strings.TrimSpace(mutation.AccessGroup)
	if accessGroup == "" || strings.EqualFold(accessGroup, "auto") {
		return nil, fmt.Errorf("a fixed access group is required")
	}
	idempotencyKey := strings.TrimSpace(mutation.IdempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > 128 {
		return nil, fmt.Errorf("idempotency key must contain 1 to 128 bytes")
	}
	mutation.ModelCode = modelCode
	baseURL := strings.TrimRight(strings.TrimSpace(mutation.BaseURL), "/")
	organization := strings.TrimSpace(mutation.Organization)
	project := strings.TrimSpace(mutation.Project)
	domain := strings.TrimSpace(mutation.Domain)
	deploymentID := strings.TrimSpace(mutation.DeploymentID)
	requestedUpstreamModel := strings.TrimSpace(mutation.RequestedUpstreamModel)
	idempotencyHash := sha256Hex(strings.Join([]string{baseURL, organization, project, domain, accessGroup, deploymentID, modelCode, requestedUpstreamModel}, "\x00"))
	now := common.GetTimestamp()
	var publication FlytePublication
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&publication).Error; err == nil {
			var existingGateway FlyteGateway
			if err := tx.First(&existingGateway, publication.GatewayID).Error; err != nil {
				return err
			}
			if publication.IdempotencyHash != idempotencyHash || existingGateway.BaseURL != baseURL || existingGateway.Organization != organization || existingGateway.Project != project || existingGateway.Domain != domain || existingGateway.AccessGroup != accessGroup || publication.DeploymentID != deploymentID || publication.ModelCode != modelCode {
				return fmt.Errorf("%w: idempotency key was already used for a different publication", ErrFlytePublicationConflict)
			}
			return nil
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		var lockedScope FlyteGateway
		if scopeErr := lockForUpdate(tx).Where("base_url = ? AND project = ? AND domain = ?", baseURL, project, domain).First(&lockedScope).Error; scopeErr == nil && lockedScope.Organization != organization {
			return fmt.Errorf("Flyte2 organization changed from %s to %s; publication is frozen", lockedScope.Organization, organization)
		} else if scopeErr != nil && scopeErr != gorm.ErrRecordNotFound {
			return scopeErr
		}
		scopeKey := FlyteScopeKey(baseURL, organization, project, domain)
		var gateway FlyteGateway
		err := lockForUpdate(tx).Where("scope_key = ?", scopeKey).First(&gateway).Error
		if err == gorm.ErrRecordNotFound {
			channel, createErr := createFlyteManagedChannel(tx, accessGroup)
			if createErr != nil {
				return createErr
			}
			gateway = FlyteGateway{ScopeKey: scopeKey, BaseURL: baseURL, Organization: organization, Project: project, Domain: domain, AccessGroup: accessGroup, ChannelID: channel.Id, CreatedBy: mutation.CreatedBy, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&gateway).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if gateway.AccessGroup != accessGroup {
			return fmt.Errorf("Flyte gateway is fixed to access group %s", gateway.AccessGroup)
		}
		if conflictID, conflict := enabledChannelConflictWithTx(tx, gateway.ChannelID, gateway.AccessGroup, modelCode); conflict {
			return fmt.Errorf("%w: enabled channel %d already serves model %s in group %s; disable it before publishing", ErrFlytePublicationConflict, conflictID, modelCode, gateway.AccessGroup)
		}
		var duplicateCount int64
		if err := tx.Model(&FlytePublication{}).Where("model_code_key = ? OR (gateway_id = ? AND deployment_id = ?)", FlyteModelCodeKey(modelCode), gateway.ID, deploymentID).Count(&duplicateCount).Error; err != nil {
			return err
		}
		if duplicateCount > 0 {
			return fmt.Errorf("%w: model code or deployment is already published", ErrFlytePublicationConflict)
		}

		nextRetryAt := now + 120
		if mutation.Phase == FlytePublicationPhasePending {
			nextRetryAt = now + 15
		}
		publication = FlytePublication{GatewayID: gateway.ID, DeploymentID: deploymentID, ModelCode: modelCode, ModelCodeKey: FlyteModelCodeKey(modelCode), Endpoint: mutation.Endpoint, UpstreamModel: mutation.UpstreamModel, Phase: mutation.Phase, ReasonCode: mutation.ReasonCode, LastError: mutation.LastError, LastSeenAt: now, NextRetryAt: nextRetryAt, IdempotencyKey: &idempotencyKey, IdempotencyHash: idempotencyHash, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&publication).Error; err != nil {
			return err
		}
		return rebuildFlyteManagedChannel(tx, gateway.ID)
	})
	if err != nil {
		return nil, err
	}
	InitChannelCache()
	return loadFlytePublicationView(DB, publication)
}

func enabledChannelConflictWithTx(tx *gorm.DB, managedChannelID int, group, modelCode string) (int, bool) {
	var channels []Channel
	if err := tx.Select("id", "models", commonGroupCol).Where("status = ? AND id <> ?", common.ChannelStatusEnabled, managedChannelID).Find(&channels).Error; err != nil {
		return 0, false
	}
	for _, channel := range channels {
		groupMatches := false
		for _, candidate := range strings.Split(channel.Group, ",") {
			if strings.TrimSpace(candidate) == group {
				groupMatches = true
				break
			}
		}
		if !groupMatches {
			continue
		}
		for _, candidate := range strings.Split(channel.Models, ",") {
			if strings.TrimSpace(candidate) == modelCode {
				return channel.Id, true
			}
		}
	}
	return 0, false
}

func UnpublishFlyteDeployment(publicationID int64) error {
	err := DB.Transaction(func(tx *gorm.DB) error {
		var publication FlytePublication
		if err := lockForUpdate(tx).First(&publication, publicationID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&publication).Error; err != nil {
			return err
		}
		return rebuildFlyteManagedChannel(tx, publication.GatewayID)
	})
	if err == nil {
		InitChannelCache()
	}
	return err
}

func createFlyteManagedChannel(tx *gorm.DB, group string) (*Channel, error) {
	zero, priority, weight := 0, int64(0), uint(0)
	baseURL, mapping, tag := "", "{}", "flyte2-managed"
	channel := &Channel{Type: constant.ChannelTypeAdvancedCustom, Key: "", Status: common.ChannelStatusEnabled, Name: "Flyte2 managed gateway", Weight: &weight, CreatedTime: common.GetTimestamp(), BaseURL: &baseURL, Models: "", Group: group, ModelMapping: &mapping, Priority: &priority, AutoBan: &zero, Tag: &tag}
	channel.SetOtherSettings(dto.ChannelOtherSettings{AdvancedCustom: &dto.AdvancedCustomConfig{Routes: []dto.AdvancedCustomRoute{
		{IncomingPath: "/v1/chat/completions", UpstreamPath: "/v1/chat/completions", Converter: "none", Auth: &dto.AdvancedCustomRouteAuth{Type: dto.AdvancedCustomAuthTypeNone}},
		{IncomingPath: "/v1/completions", UpstreamPath: "/v1/completions", Converter: "none", Auth: &dto.AdvancedCustomRouteAuth{Type: dto.AdvancedCustomAuthTypeNone}},
		{IncomingPath: "/v1/responses", UpstreamPath: "/v1/responses", Converter: "none", Auth: &dto.AdvancedCustomRouteAuth{Type: dto.AdvancedCustomAuthTypeNone}},
		{IncomingPath: "/v1/embeddings", UpstreamPath: "/v1/embeddings", Converter: "none", Auth: &dto.AdvancedCustomRouteAuth{Type: dto.AdvancedCustomAuthTypeNone}},
	}}})
	if err := channel.ValidateSettings(); err != nil {
		return nil, err
	}
	if err := tx.Create(channel).Error; err != nil {
		return nil, err
	}
	return channel, nil
}

func rebuildFlyteManagedChannel(tx *gorm.DB, gatewayID int64) error {
	var gateway FlyteGateway
	if err := lockForUpdate(tx).First(&gateway, gatewayID).Error; err != nil {
		return err
	}
	var publications []FlytePublication
	if err := tx.Where("gateway_id = ? AND phase IN ? AND endpoint <> '' AND upstream_model <> ''", gatewayID, []string{FlytePublicationPhasePublished, FlytePublicationPhaseDrifted}).Find(&publications).Error; err != nil {
		return err
	}
	models := make([]string, 0, len(publications))
	for _, publication := range publications {
		models = append(models, publication.ModelCode)
	}
	sort.Strings(models)
	var channel Channel
	if err := lockForUpdate(tx).First(&channel, gateway.ChannelID).Error; err != nil {
		return err
	}
	channel.Models = strings.Join(models, ",")
	if err := tx.Model(&channel).Select("models").Update("models", channel.Models).Error; err != nil {
		return err
	}
	return channel.UpdateAbilities(tx)
}

func IsFlyteManagedChannel(channelID int) bool {
	if !flytePublicationTablesReady(DB) {
		return false
	}
	var count int64
	return DB.Model(&FlyteGateway{}).Where("channel_id = ?", channelID).Count(&count).Error == nil && count > 0
}

func HasFlyteManagedChannelWithTag(tag string) bool {
	if !flytePublicationTablesReady(DB) {
		return false
	}
	var count int64
	return DB.Table("flyte_gateways").Joins("JOIN channels ON channels.id = flyte_gateways.channel_id").Where("channels.tag = ?", tag).Count(&count).Error == nil && count > 0
}

func MarkFlyteManagedChannels(channels []*Channel) {
	if len(channels) == 0 || !flytePublicationTablesReady(DB) {
		return
	}
	ids := make([]int, 0, len(channels))
	for _, channel := range channels {
		ids = append(ids, channel.Id)
	}
	var gateways []FlyteGateway
	if err := DB.Where("channel_id IN ?", ids).Find(&gateways).Error; err != nil {
		return
	}
	byChannel := make(map[int]FlyteGateway, len(gateways))
	for _, gateway := range gateways {
		byChannel[gateway.ChannelID] = gateway
	}
	for _, channel := range channels {
		if gateway, ok := byChannel[channel.Id]; ok {
			channel.Flyte2Managed = true
			channel.FlyteGatewayID = gateway.ID
			channel.Flyte2Domain = gateway.Domain
		}
	}
}

func FlytePublishedChannelConflict(channelID int, groupsValue, modelsValue string) (string, bool) {
	if !flytePublicationTablesReady(DB) || IsFlyteManagedChannel(channelID) {
		return "", false
	}
	groups := map[string]bool{}
	for _, group := range strings.Split(groupsValue, ",") {
		groups[strings.TrimSpace(group)] = true
	}
	models := map[string]bool{}
	for _, modelCode := range strings.Split(modelsValue, ",") {
		models[strings.TrimSpace(modelCode)] = true
	}
	var rows []struct {
		ModelCode   string
		AccessGroup string
	}
	err := DB.Table("flyte_publications").Select("flyte_publications.model_code, flyte_gateways.access_group").
		Joins("JOIN flyte_gateways ON flyte_gateways.id = flyte_publications.gateway_id").
		Joins("JOIN channels ON channels.id = flyte_gateways.channel_id").
		Where("channels.status = ? AND flyte_publications.phase IN ?", common.ChannelStatusEnabled, []string{FlytePublicationPhasePublished, FlytePublicationPhaseDrifted}).Scan(&rows).Error
	if err != nil {
		return "", false
	}
	for _, row := range rows {
		if groups[row.AccessGroup] && models[row.ModelCode] {
			return row.ModelCode, true
		}
	}
	return "", false
}

func FlyteManagedChannelEnableConflict(channelID int) (string, bool) {
	if !IsFlyteManagedChannel(channelID) {
		return "", false
	}
	var channel Channel
	if err := DB.Select("id", "models", commonGroupCol).First(&channel, channelID).Error; err != nil {
		return "", false
	}
	for _, group := range strings.Split(channel.Group, ",") {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		for _, modelCode := range strings.Split(channel.Models, ",") {
			modelCode = strings.TrimSpace(modelCode)
			if modelCode == "" {
				continue
			}
			if _, conflict := enabledChannelConflictWithTx(DB, channelID, group, modelCode); conflict {
				return modelCode, true
			}
		}
	}
	return "", false
}

func FlyteManagedChannelDefinitionConflict(channelID int, groupsValue, modelsValue string) (string, bool) {
	if !IsFlyteManagedChannel(channelID) {
		return "", false
	}
	var managed Channel
	if err := DB.Select("id", "models", commonGroupCol).First(&managed, channelID).Error; err != nil {
		return "", false
	}
	groups := map[string]bool{}
	for _, group := range strings.Split(groupsValue, ",") {
		groups[strings.TrimSpace(group)] = true
	}
	models := map[string]bool{}
	for _, modelCode := range strings.Split(modelsValue, ",") {
		models[strings.TrimSpace(modelCode)] = true
	}
	for _, group := range strings.Split(managed.Group, ",") {
		if !groups[strings.TrimSpace(group)] {
			continue
		}
		for _, modelCode := range strings.Split(managed.Models, ",") {
			modelCode = strings.TrimSpace(modelCode)
			if modelCode != "" && models[modelCode] {
				return modelCode, true
			}
		}
	}
	return "", false
}

func GetFlyteManagedHealthEndpoint(channelID int) (string, bool) {
	if !flytePublicationTablesReady(DB) {
		return "", false
	}
	var publication FlytePublication
	err := DB.Table("flyte_publications").Select("flyte_publications.*").
		Joins("JOIN flyte_gateways ON flyte_gateways.id = flyte_publications.gateway_id").
		Where("flyte_gateways.channel_id = ? AND flyte_publications.phase IN ? AND flyte_publications.endpoint <> ''", channelID, []string{FlytePublicationPhasePublished, FlytePublicationPhaseDrifted}).
		Order("flyte_publications.id").First(&publication).Error
	return publication.Endpoint, err == nil && publication.Endpoint != ""
}

func UpdateFlyteManagedChannelTuning(channelID int, priority *int64, weight *uint, tag *string, updatePriority, updateWeight, updateTag bool) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		channelUpdates := map[string]any{}
		abilityUpdates := map[string]any{}
		if updatePriority {
			channelUpdates["priority"] = priority
			abilityUpdates["priority"] = priority
		}
		if updateWeight {
			channelUpdates["weight"] = weight
			if weight == nil {
				abilityUpdates["weight"] = 0
			} else {
				abilityUpdates["weight"] = *weight
			}
		}
		if updateTag {
			channelUpdates["tag"] = tag
			abilityUpdates["tag"] = tag
		}
		if len(channelUpdates) == 0 {
			return nil
		}
		if err := tx.Model(&Channel{}).Where("id = ?", channelID).Updates(channelUpdates).Error; err != nil {
			return err
		}
		return tx.Model(&Ability{}).Where("channel_id = ?", channelID).Updates(abilityUpdates).Error
	})
}

func flytePublicationTablesReady(db *gorm.DB) bool {
	return db.Migrator().HasTable(&FlyteGateway{}) && db.Migrator().HasTable(&FlytePublication{})
}

func DetachLegacyFlytePublicationBindings() error {
	if !DB.Migrator().HasTable(&FlytePublicationBinding{}) {
		return nil
	}
	result := DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&FlytePublicationBinding{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		common.SysLog(fmt.Sprintf("detached %d legacy Flyte publication API key bindings", result.RowsAffected))
	}
	return nil
}
