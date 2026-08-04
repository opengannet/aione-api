package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type accessTokenResponse struct {
	Success bool   `json:"success"`
	Data    string `json:"data"`
}

func setupAccessTokenControllerTest(t *testing.T, accessToken *string) *model.User {
	t.Helper()
	previousDB := model.DB
	previousType := common.MainDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	user := &model.User{
		Username:    "access-token-user",
		Password:    "hashed-password",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AuthVersion: 1,
		AccessToken: accessToken,
	}
	require.NoError(t, db.Create(user).Error)

	t.Cleanup(func() {
		model.DB = previousDB
		common.SetMainDatabaseType(previousType)
		common.RedisEnabled = previousRedisEnabled
	})
	return user
}

func callAccessTokenController(t *testing.T, method string, userID int, handler gin.HandlerFunc) accessTokenResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, "/api/user/token", nil)
	c.Set("id", userID)

	handler(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response accessTokenResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	return response
}

func TestGetAccessTokenReturnsExistingTokenWithoutRotating(t *testing.T) {
	existingToken := "existing-system-access-token"
	user := setupAccessTokenControllerTest(t, &existingToken)

	response := callAccessTokenController(t, http.MethodGet, user.Id, GetAccessToken)

	assert.Equal(t, existingToken, response.Data)
	storedUser, err := model.GetUserById(user.Id, true)
	require.NoError(t, err)
	assert.Equal(t, existingToken, storedUser.GetAccessToken())
}

func TestGetAccessTokenDoesNotCreateMissingToken(t *testing.T) {
	user := setupAccessTokenControllerTest(t, nil)

	response := callAccessTokenController(t, http.MethodGet, user.Id, GetAccessToken)

	assert.Empty(t, response.Data)
	storedUser, err := model.GetUserById(user.Id, true)
	require.NoError(t, err)
	assert.Nil(t, storedUser.AccessToken)
}

func TestGenerateAccessTokenCreatesMissingToken(t *testing.T) {
	user := setupAccessTokenControllerTest(t, nil)

	response := callAccessTokenController(t, http.MethodPost, user.Id, GenerateAccessToken)

	assert.NotEmpty(t, response.Data)
	storedUser, err := model.GetUserById(user.Id, true)
	require.NoError(t, err)
	assert.Equal(t, response.Data, storedUser.GetAccessToken())
}

func TestGenerateAccessTokenReplacesExistingToken(t *testing.T) {
	existingToken := "existing-system-access-token"
	user := setupAccessTokenControllerTest(t, &existingToken)

	response := callAccessTokenController(t, http.MethodPost, user.Id, GenerateAccessToken)

	require.NotEmpty(t, response.Data)
	assert.NotEqual(t, existingToken, response.Data)
	oldTokenUser, err := model.ValidateAccessToken(existingToken)
	require.NoError(t, err)
	assert.Nil(t, oldTokenUser)
	newTokenUser, err := model.ValidateAccessToken(response.Data)
	require.NoError(t, err)
	require.NotNil(t, newTokenUser)
	assert.Equal(t, user.Id, newTokenUser.Id)
}
