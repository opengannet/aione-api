package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
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

	unpublished, err := RemoveFlytePublicationBinding("model-a", restricted.Id)
	require.NoError(t, err)
	assert.False(t, unpublished)
	require.NoError(t, DB.First(&restricted, restricted.Id).Error)
	assert.Equal(t, "existing", restricted.ModelLimits)
	unpublished, err = RemoveFlytePublicationBinding("model-a", unrestricted.Id)
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
