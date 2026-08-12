package flyte2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListModelsSendsScopeAndAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/api/aione/models", r.URL.Path)
		assert.Equal(t, "aione", r.URL.Query().Get("project"))
		assert.Equal(t, "development", r.URL.Query().Get("domain"))
		assert.Equal(t, "secret-key", r.Header.Get("X-API-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":200,"data":{"items":[],"total":0,"page":1,"pageSize":20}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v2", "secret-key")
	require.NoError(t, err)
	result, err := client.ListModels(context.Background(), "aione", "development", "", "", 1, 20)
	require.NoError(t, err)
	assert.Zero(t, result.Total)
}

func TestNewClientRejectsUnsafeURLs(t *testing.T) {
	tests := []string{
		"ftp://flyte.example/v2",
		"https://user:password@flyte.example/v2",
		"https://flyte.example/v2?key=value",
		"https://flyte.example/v2#fragment",
		"https://flyte.example",
	}
	for _, rawURL := range tests {
		t.Run(rawURL, func(t *testing.T) {
			_, err := NewClient(rawURL, "key")
			assert.Error(t, err)
		})
	}
}

func TestAPIErrorDoesNotExposeAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"status":401,"message":"unauthorized"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v2", "sensitive-key")
	require.NoError(t, err)
	_, err = client.ListModels(context.Background(), "aione", "development", "", "", 1, 20)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "sensitive-key")
}
