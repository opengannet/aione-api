package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFlytePublicationTest(t *testing.T) {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(&Channel{}, &Ability{}, &Token{}, &FlyteGateway{}, &FlytePublication{}, &FlytePublicationBinding{}))
	previousDB, previousCache := DB, common.MemoryCacheEnabled
	DB, common.MemoryCacheEnabled = database, true
	t.Cleanup(func() { DB, common.MemoryCacheEnabled = previousDB, previousCache })
}

func TestFlytePublicationCreatesManagedGatewayWithoutMutatingAPIKeys(t *testing.T) {
	setupFlytePublicationTest(t)
	token := Token{UserId: 1, Key: "ordinary-key", Name: "ordinary", Group: "aione", ModelLimitsEnabled: true, ModelLimits: "existing", RemainQuota: 123}
	require.NoError(t, DB.Create(&token).Error)

	mutation := FlytePublicationMutation{
		BaseURL: "https://flyte.example/v2", Organization: "org", Project: "aione", Domain: "development",
		AccessGroup: "aione", DeploymentID: "model-a", ModelCode: "qwen25-15b", Endpoint: "https://model.example",
		UpstreamModel: "served-qwen", Phase: FlytePublicationPhasePublished, IdempotencyKey: "publish-1",
	}
	publication, err := PublishFlyteDeployment(mutation)
	require.NoError(t, err)
	replayed, err := PublishFlyteDeployment(mutation)
	require.NoError(t, err)
	assert.Equal(t, publication.ID, replayed.ID)

	conflict := mutation
	conflict.DeploymentID = "model-b"
	conflict.ModelCode = "other-model"
	_, err = PublishFlyteDeployment(conflict)
	require.ErrorIs(t, err, ErrFlytePublicationConflict)

	differentUpstream := mutation
	differentUpstream.RequestedUpstreamModel = "different-upstream"
	_, err = PublishFlyteDeployment(differentUpstream)
	require.ErrorIs(t, err, ErrFlytePublicationConflict)

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

	var stored Token
	require.NoError(t, DB.First(&stored, token.Id).Error)
	assert.Equal(t, "existing", stored.ModelLimits)
	assert.Equal(t, 123, stored.RemainQuota)
	var bindingCount int64
	require.NoError(t, DB.Model(&FlytePublicationBinding{}).Count(&bindingCount).Error)
	assert.Zero(t, bindingCount)

	baseURL, mapping, ok := GetFlyteManagedRoute(channel.Id, "qwen25-15b")
	assert.True(t, ok)
	assert.Equal(t, "https://model.example", baseURL)
	assert.JSONEq(t, `{"qwen25-15b":"served-qwen"}`, mapping)

	require.NoError(t, UnpublishFlyteDeployment(publication.ID))
	require.NoError(t, DB.First(&stored, token.Id).Error)
	assert.Equal(t, "existing", stored.ModelLimits)
	assert.Equal(t, 123, stored.RemainQuota)
	require.NoError(t, DB.First(&channel, channel.Id).Error)
	assert.Empty(t, channel.Models)
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
	setupFlytePublicationTest(t)
	development, err := PublishFlyteDeployment(FlytePublicationMutation{
		BaseURL: "https://flyte.example/v2", Organization: "org", Project: "aione", Domain: "development",
		AccessGroup: "aione", DeploymentID: "shared-id", ModelCode: "development-model", Phase: FlytePublicationPhasePending,
		IdempotencyKey: "development-publication",
	})
	require.NoError(t, err)
	production, err := PublishFlyteDeployment(FlytePublicationMutation{
		BaseURL: "https://flyte.example/v2", Organization: "org", Project: "aione", Domain: "production",
		AccessGroup: "aione", DeploymentID: "shared-id", ModelCode: "production-model", Phase: FlytePublicationPhasePending,
		IdempotencyKey: "production-publication",
	})
	require.NoError(t, err)

	developmentLookup, err := GetFlytePublication("https://flyte.example/v2", "aione", "development", "shared-id")
	require.NoError(t, err)
	assert.Equal(t, development.ID, developmentLookup.ID)
	productionLookup, err := GetFlytePublication("https://flyte.example/v2", "aione", "production", "shared-id")
	require.NoError(t, err)
	assert.Equal(t, production.ID, productionLookup.ID)
	assert.NotEqual(t, developmentLookup.Gateway.ChannelID, productionLookup.Gateway.ChannelID)

	require.NoError(t, UnpublishFlyteDeployment(development.ID))
	_, err = GetFlytePublication("https://flyte.example/v2", "aione", "development", "shared-id")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	productionLookup, err = GetFlytePublication("https://flyte.example/v2", "aione", "production", "shared-id")
	require.NoError(t, err)
	assert.Equal(t, production.ID, productionLookup.ID)
}

func TestDetachLegacyFlytePublicationBindingsPreservesAPIKeyConfiguration(t *testing.T) {
	setupFlytePublicationTest(t)
	token := Token{UserId: 7, Key: "legacy-key", Name: "legacy", Group: "aione", ModelLimitsEnabled: true, ModelLimits: "org/qwen", RemainQuota: 456, UnlimitedQuota: false}
	require.NoError(t, DB.Create(&token).Error)
	gateway := FlyteGateway{ScopeKey: FlyteScopeKey("https://flyte.example/v2", "org", "aione", "development"), BaseURL: "https://flyte.example/v2", Organization: "org", Project: "aione", Domain: "development", AccessGroup: "aione", ChannelID: 99}
	require.NoError(t, DB.Create(&gateway).Error)
	publication := FlytePublication{GatewayID: gateway.ID, DeploymentID: "model-a", ModelCode: "org/qwen", ModelCodeKey: FlyteModelCodeKey("org/qwen")}
	require.NoError(t, DB.Create(&publication).Error)
	binding := FlytePublicationBinding{PublicationID: publication.ID, TokenID: token.Id, ManagedPermissionAdded: true, IdempotencyKey: "legacy-binding"}
	require.NoError(t, DB.Create(&binding).Error)

	require.NoError(t, DetachLegacyFlytePublicationBindings())
	require.NoError(t, DetachLegacyFlytePublicationBindings())
	var bindingCount int64
	require.NoError(t, DB.Model(&FlytePublicationBinding{}).Count(&bindingCount).Error)
	assert.Zero(t, bindingCount)
	var stored Token
	require.NoError(t, DB.First(&stored, token.Id).Error)
	assert.Equal(t, "aione", stored.Group)
	assert.Equal(t, "org/qwen", stored.ModelLimits)
	assert.Equal(t, 456, stored.RemainQuota)
	assert.False(t, stored.UnlimitedQuota)
	var publicationCount int64
	require.NoError(t, DB.Model(&FlytePublication{}).Count(&publicationCount).Error)
	assert.Equal(t, int64(1), publicationCount)
}
