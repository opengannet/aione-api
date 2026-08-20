package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRetiredFrontendAPIRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	_, hasAsyncCleanup := routes[http.MethodPost+" /api/system-task/log-cleanup"]
	_, hasDirectDelete := routes[http.MethodDelete+" /api/log/"]
	_, hasConsoleMigration := routes[http.MethodPost+" /api/option/migrate_console_setting"]
	assert.True(t, hasAsyncCleanup)
	assert.False(t, hasDirectDelete)
	assert.False(t, hasConsoleMigration)
}

func TestRetiredFlytePublicationAPIKeyRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	_, hasPublication := routes[http.MethodPost+" /api/deployments/:id/publication"]
	_, hasTokenCreation := routes[http.MethodPost+" /api/token/"]
	_, hasPublicationTokenCreation := routes[http.MethodPost+" /api/token/flyte-publication"]
	_, hasBindingCreation := routes[http.MethodPost+" /api/deployments/:id/publication/bindings"]
	_, hasBindingDeletion := routes[http.MethodDelete+" /api/deployments/:id/publication/bindings/:token_id"]

	assert.True(t, hasPublication)
	assert.True(t, hasTokenCreation)
	assert.False(t, hasPublicationTokenCreation)
	assert.False(t, hasBindingCreation)
	assert.False(t, hasBindingDeletion)
}
