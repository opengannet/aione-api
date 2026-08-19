package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFlytePublicationCreatesOneManagedGatewayAndOwnsOnlyAddedPermissions(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&Channel{}, &Ability{}, &Token{}, &FlyteGateway{}, &FlytePublication{}, &FlytePublicationBinding{}))
	previousDB, previousCache := DB, common.MemoryCacheEnabled
	DB, common.MemoryCacheEnabled = database, true
	t.Cleanup(func() { DB, common.MemoryCacheEnabled = previousDB, previousCache })

	restricted := Token{UserId: 1, Key: "restricted-key", Name: "restricted", Group: "aione", ModelLimitsEnabled: true, ModelLimits: "existing"}
	unrestricted := Token{UserId: 1, Key: "unrestricted-key", Name: "unrestricted", Group: "aione"}
	require.NoError(t, DB.Create(&restricted).Error)
	require.NoError(t, DB.Create(&unrestricted).Error)

	publication, _, err := PublishFlyteDeployment(FlytePublicationMutation{
		BaseURL: "https://flyte.example/v2", Organization: "org", Project: "aione", Domain: "development",
		AccessGroup: "aione", DeploymentID: "model-a", ModelCode: "qwen25-15b", Endpoint: "https://model.example",
		UpstreamModel: "served-qwen", Phase: FlytePublicationPhasePublished, TokenIDs: []int{restricted.Id, unrestricted.Id}, IdempotencyKey: "publish-1",
	})
	require.NoError(t, err)
	require.Len(t, publication.Bindings, 2)
	repeated, repeatedToken, err := PublishFlyteDeployment(FlytePublicationMutation{
		BaseURL: "https://flyte.example/v2", Organization: "org", Project: "aione", Domain: "development",
		AccessGroup: "aione", DeploymentID: "model-a", ModelCode: "qwen25-15b", Endpoint: "https://model.example",
		UpstreamModel: "served-qwen", Phase: FlytePublicationPhasePublished, TokenIDs: []int{restricted.Id, unrestricted.Id}, IdempotencyKey: "publish-1",
	})
	require.NoError(t, err)
	assert.Equal(t, publication.ID, repeated.ID)
	assert.Nil(t, repeatedToken)

	var gateways []FlyteGateway
	require.NoError(t, DB.Find(&gateways).Error)
	require.Len(t, gateways, 1)
	var channel Channel
	require.NoError(t, DB.First(&channel, gateways[0].ChannelID).Error)
	assert.Empty(t, channel.Key)
	assert.Empty(t, channel.GetBaseURL())
	assert.Equal(t, "aione", channel.Group)
	assert.Equal(t, "qwen25-15b", channel.Models)
	assert.False(t, channel.GetAutoBan())
	settings := channel.GetOtherSettings()
	require.NotNil(t, settings.AdvancedCustom)
	require.Len(t, settings.AdvancedCustom.Routes, 4)
	for _, route := range settings.AdvancedCustom.Routes {
		assert.Empty(t, route.Models)
		require.NotNil(t, route.Auth)
		assert.Equal(t, dto.AdvancedCustomAuthTypeNone, route.Auth.Type)
	}

	require.NoError(t, DB.First(&restricted, restricted.Id).Error)
	assert.Equal(t, "existing,qwen25-15b", restricted.ModelLimits)
	require.NoError(t, DB.First(&unrestricted, unrestricted.Id).Error)
	assert.False(t, unrestricted.ModelLimitsEnabled)
	baseURL, mapping, ok := GetFlyteManagedRoute(channel.Id, "qwen25-15b")
	assert.True(t, ok)
	assert.Equal(t, "https://model.example", baseURL)
	assert.JSONEq(t, `{"qwen25-15b":"served-qwen"}`, mapping)
	_, _, found := GetFlyteManagedRoute(channel.Id, "qwen25-15b-extra")
	assert.False(t, found)

	require.NoError(t, DB.Model(&channel).Update("status", common.ChannelStatusManuallyDisabled).Error)
	manual := Channel{Type: 1, Key: "manual-key", Status: common.ChannelStatusEnabled, Name: "rollback channel", Models: "qwen25-15b", Group: "aione"}
	require.NoError(t, DB.Create(&manual).Error)
	_, conflict := FlytePublishedChannelConflict(manual.Id, manual.Group, manual.Models)
	assert.False(t, conflict, "a disabled managed channel must not block rollback")
	conflictModel, conflict := FlyteManagedChannelEnableConflict(channel.Id)
	assert.True(t, conflict)
	assert.Equal(t, "qwen25-15b", conflictModel)

	unpublished, err := RemoveFlytePublicationBinding(publication.ID, restricted.Id)
	require.NoError(t, err)
	assert.False(t, unpublished)
	require.NoError(t, DB.First(&restricted, restricted.Id).Error)
	assert.Equal(t, "existing", restricted.ModelLimits)
	unpublished, err = RemoveFlytePublicationBinding(publication.ID, unrestricted.Id)
	require.NoError(t, err)
	assert.True(t, unpublished)
	require.NoError(t, DB.First(&channel, channel.Id).Error)
	assert.Empty(t, channel.Models)
	var abilityCount int64
	require.NoError(t, DB.Model(&Ability{}).Where("channel_id = ?", channel.Id).Count(&abilityCount).Error)
	assert.Zero(t, abilityCount)
}

func TestValidateFlyteModelCode(t *testing.T) {
	valid, err := ValidateFlyteModelCode(" qwen/model ")
	require.NoError(t, err)
	assert.Equal(t, "qwen/model", valid)
	for _, value := range []string{"", "a,b", "line\nbreak", string(make([]byte, 256))} {
		_, err := ValidateFlyteModelCode(value)
		assert.Error(t, err)
	}
}

func TestFlytePublicationScopeSeparatesMatchingDeploymentIDs(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&Channel{}, &Ability{}, &Token{}, &FlyteGateway{}, &FlytePublication{}, &FlytePublicationBinding{}))
	previousDB, previousCache := DB, common.MemoryCacheEnabled
	DB, common.MemoryCacheEnabled = database, true
	t.Cleanup(func() { DB, common.MemoryCacheEnabled = previousDB, previousCache })

	token := Token{UserId: 1, Key: "scope-key", Name: "scope", Group: "aione", ModelLimitsEnabled: true}
	require.NoError(t, DB.Create(&token).Error)
	development, _, err := PublishFlyteDeployment(FlytePublicationMutation{
		BaseURL: "https://flyte.example/v2", Organization: "org", Project: "aione", Domain: "development",
		AccessGroup: "aione", DeploymentID: "shared-id", ModelCode: "development-model", Phase: FlytePublicationPhasePending,
		TokenIDs: []int{token.Id}, IdempotencyKey: "development-publication",
	})
	require.NoError(t, err)
	production, _, err := PublishFlyteDeployment(FlytePublicationMutation{
		BaseURL: "https://flyte.example/v2", Organization: "org", Project: "aione", Domain: "production",
		AccessGroup: "aione", DeploymentID: "shared-id", ModelCode: "production-model", Phase: FlytePublicationPhasePending,
		TokenIDs: []int{token.Id}, IdempotencyKey: "production-publication",
	})
	require.NoError(t, err)

	developmentLookup, err := GetFlytePublication("https://flyte.example/v2", "aione", "development", "shared-id")
	require.NoError(t, err)
	assert.Equal(t, development.ID, developmentLookup.ID)
	productionLookup, err := GetFlytePublication("https://flyte.example/v2", "aione", "production", "shared-id")
	require.NoError(t, err)
	assert.Equal(t, production.ID, productionLookup.ID)
	assert.NotEqual(t, developmentLookup.Gateway.ChannelID, productionLookup.Gateway.ChannelID)
	require.NoError(t, DB.First(&token, token.Id).Error)
	assert.ElementsMatch(t, []string{"development-model", "production-model"}, token.GetModelLimits())

	require.NoError(t, UnpublishFlyteDeployment(development.ID))
	_, err = GetFlytePublication("https://flyte.example/v2", "aione", "development", "shared-id")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	productionLookup, err = GetFlytePublication("https://flyte.example/v2", "aione", "production", "shared-id")
	require.NoError(t, err)
	assert.Equal(t, production.ID, productionLookup.ID)
	require.NoError(t, DB.First(&token, token.Id).Error)
	assert.Equal(t, []string{"production-model"}, token.GetModelLimits())
}

func TestCreateFlytePublicationTokenIsAtomicAndIdempotent(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&Channel{}, &Ability{}, &Token{}, &FlyteGateway{}, &FlytePublication{}, &FlytePublicationBinding{}))
	previousDB, previousCache := DB, common.MemoryCacheEnabled
	DB, common.MemoryCacheEnabled = database, true
	t.Cleanup(func() { DB, common.MemoryCacheEnabled = previousDB, previousCache })

	seed := Token{UserId: 1, Key: "seed-key", Name: "seed", Group: "aione", ModelLimitsEnabled: true}
	require.NoError(t, DB.Create(&seed).Error)
	publication, _, err := PublishFlyteDeployment(FlytePublicationMutation{
		BaseURL: "https://flyte.example/v2", Organization: "org", Project: "aione", Domain: "development",
		AccessGroup: "aione", DeploymentID: "model-a", ModelCode: "org/qwen", Endpoint: "https://model.example",
		UpstreamModel: "served-qwen", Phase: FlytePublicationPhasePublished, TokenIDs: []int{seed.Id}, IdempotencyKey: "publish-token-test",
	})
	require.NoError(t, err)

	mutation := FlytePublicationTokenMutation{
		ModelCode: "org/qwen", Name: "flyte-created", IdempotencyKey: "issue-1", UserID: 7, ExpectedAccessGroup: "aione",
	}
	issued, created, err := CreateFlytePublicationToken(mutation)
	require.NoError(t, err)
	assert.True(t, created)
	assert.NotEmpty(t, issued.Key)

	var stored Token
	require.NoError(t, DB.First(&stored, issued.Id).Error)
	assert.Equal(t, 7, stored.UserId)
	assert.Equal(t, "aione", stored.Group)
	assert.Equal(t, common.TokenStatusEnabled, stored.Status)
	assert.Equal(t, int64(-1), stored.ExpiredTime)
	assert.True(t, stored.UnlimitedQuota)
	assert.True(t, stored.ModelLimitsEnabled)
	assert.Equal(t, "org/qwen", stored.ModelLimits)
	assert.False(t, stored.CrossGroupRetry)

	var binding FlytePublicationBinding
	require.NoError(t, DB.Where("publication_id = ? AND token_id = ?", publication.ID, issued.Id).First(&binding).Error)
	assert.True(t, binding.ManagedPermissionAdded)
	assert.Equal(t, "issue-1", binding.IdempotencyKey)

	replayed, replayCreated, err := CreateFlytePublicationToken(mutation)
	require.NoError(t, err)
	assert.False(t, replayCreated)
	assert.Equal(t, issued.Id, replayed.Id)
	assert.Equal(t, issued.Key, replayed.Key)

	secondMutation := mutation
	secondMutation.IdempotencyKey = "issue-2"
	secondMutation.Name = "flyte-created-2"
	second, secondCreated, err := CreateFlytePublicationToken(secondMutation)
	require.NoError(t, err)
	assert.True(t, secondCreated)
	assert.NotEqual(t, issued.Id, second.Id)
	assert.NotEqual(t, issued.Key, second.Key)

	var tokenCount int64
	require.NoError(t, DB.Model(&Token{}).Count(&tokenCount).Error)
	assert.Equal(t, int64(3), tokenCount)
	previousMaxTokens := operation_setting.GetTokenSetting().MaxUserTokens
	t.Cleanup(func() { operation_setting.GetTokenSetting().MaxUserTokens = previousMaxTokens })
	operation_setting.GetTokenSetting().MaxUserTokens = 2
	limitMutation := mutation
	limitMutation.IdempotencyKey = "issue-limit"
	_, _, err = CreateFlytePublicationToken(limitMutation)
	require.ErrorIs(t, err, ErrFlytePublicationTokenLimit)
	operation_setting.GetTokenSetting().MaxUserTokens = previousMaxTokens
	require.NoError(t, DB.Model(&Token{}).Count(&tokenCount).Error)
	assert.Equal(t, int64(3), tokenCount)

	badGroup := mutation
	badGroup.IdempotencyKey = "issue-bad-group"
	badGroup.ExpectedAccessGroup = "other"
	_, _, err = CreateFlytePublicationToken(badGroup)
	require.ErrorIs(t, err, ErrFlytePublicationConflict)
	require.NoError(t, DB.Model(&Token{}).Count(&tokenCount).Error)
	assert.Equal(t, int64(3), tokenCount)

	_, _, err = CreateFlytePublicationToken(FlytePublicationTokenMutation{
		ModelCode: "missing", Name: "missing", IdempotencyKey: "issue-missing", UserID: 7, ExpectedAccessGroup: "aione",
	})
	require.ErrorIs(t, err, ErrFlytePublicationNotFound)
	require.NoError(t, DB.Model(&Token{}).Count(&tokenCount).Error)
	assert.Equal(t, int64(3), tokenCount)

	require.NoError(t, DB.Model(&FlytePublication{}).Where("id = ?", publication.ID).Update("phase", FlytePublicationPhaseCleanup).Error)
	cleanupMutation := mutation
	cleanupMutation.IdempotencyKey = "issue-cleanup"
	_, _, err = CreateFlytePublicationToken(cleanupMutation)
	require.ErrorIs(t, err, ErrFlytePublicationNotIssuable)
	require.NoError(t, DB.Model(&Token{}).Count(&tokenCount).Error)
	assert.Equal(t, int64(3), tokenCount)

	require.NoError(t, DB.Model(&FlytePublication{}).Where("id = ?", publication.ID).Update("phase", FlytePublicationPhasePublished).Error)
	require.NoError(t, UnpublishFlyteDeployment(publication.ID))
	require.NoError(t, DB.First(&stored, issued.Id).Error)
	assert.Empty(t, stored.ModelLimits)
}
