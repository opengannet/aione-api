package flyte2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
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

func TestNormalizeBaseURLProducesStableScopeValue(t *testing.T) {
	normalized, err := NormalizeBaseURL(" HTTPS://FLYTE.EXAMPLE:443/console/v2/ ")
	require.NoError(t, err)
	assert.Equal(t, "https://flyte.example/console/v2", normalized)
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

func TestClientMapsNonJSONHTTPErrorWithoutLeakingResponseDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream proxy failure containing internal details"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v2", "sensitive-key")
	require.NoError(t, err)

	_, err = client.ListModels(context.Background(), "aione", "development", "", "", 1, 20)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusBadGateway, apiErr.Status)
	assert.Equal(t, http.StatusText(http.StatusBadGateway), apiErr.Message)
	assert.NotContains(t, err.Error(), "sensitive-key")
	assert.NotContains(t, err.Error(), "internal details")
}

func TestClientModelLifecycleRequests(t *testing.T) {
	type observedRequest struct {
		method string
		path   string
		query  string
		body   map[string]any
	}
	observed := make([]observedRequest, 0, 7)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "secret-key", r.Header.Get("X-API-Key"))
		entry := observedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery}
		if r.Body != nil && r.Header.Get("Content-Type") == "application/json" {
			require.NoError(t, common.DecodeJson(r.Body, &entry.body))
		}
		observed = append(observed, entry)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/log"):
			_, _ = w.Write([]byte(`{"status":200,"data":{"items":[],"total":0,"page":2,"size":50}}`))
		case r.Method == http.MethodDelete:
			_, _ = w.Write([]byte(`{"status":200,"data":{}}`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"status":200,"data":{"id":"model-a","config":{"codes":[]}}}`))
		default:
			_, _ = w.Write([]byte(`{"status":200,"data":{"id":"model-a","type":"VLLM"}}`))
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v2", "secret-key")
	require.NoError(t, err)
	ctx := context.Background()
	_, err = client.CreateModel(ctx, CreateModelRequest{
		Name: "Model A", ID: "model-a", Code: "qwen", Project: "aione", Domain: "development",
		Codes: []CodeSource{{ID: "https://git.example/model.git", Token: "source-token"}},
	})
	require.NoError(t, err)
	_, err = client.GetModel(ctx, "model-a", "aione", "development")
	require.NoError(t, err)
	_, err = client.UpdateModel(ctx, "model-a", "aione", "development", UpdateModelRequest{Name: "Edited", ModelCacheSize: "2Gi"})
	require.NoError(t, err)
	_, err = client.StartModel(ctx, "model-a", "aione", "development")
	require.NoError(t, err)
	_, err = client.StopModel(ctx, "model-a", "aione", "development")
	require.NoError(t, err)
	logs, err := client.GetModelLogs(ctx, "model-a", "aione", "development", 2, 50)
	require.NoError(t, err)
	assert.Equal(t, 2, logs.Page)
	require.NoError(t, client.DeleteModel(ctx, "model-a", "aione", "development"))

	require.Len(t, observed, 7)
	assert.Equal(t, observedRequest{method: http.MethodPost, path: "/v2/api/aione/model/run", body: observed[0].body}, observed[0])
	assert.Equal(t, "aione", observed[0].body["project"])
	assert.Equal(t, "development", observed[0].body["domain"])
	assert.Equal(t, http.MethodGet, observed[1].method)
	assert.Equal(t, "/v2/api/aione/model/model-a", observed[1].path)
	assert.Contains(t, observed[1].query, "project=aione")
	assert.Equal(t, http.MethodPut, observed[2].method)
	assert.Equal(t, "/v2/api/aione/model/model-a/start", observed[3].path)
	assert.Equal(t, "/v2/api/aione/model/model-a/stop", observed[4].path)
	assert.Equal(t, "/v2/api/aione/model/model-a/log", observed[5].path)
	assert.Contains(t, observed[5].query, "page=2")
	assert.Contains(t, observed[5].query, "size=50")
	assert.Equal(t, http.MethodDelete, observed[6].method)
}

func TestClientMapsFlyteHTTPStatuses(t *testing.T) {
	for _, status := range []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusNotFound,
		http.StatusConflict,
		http.StatusBadGateway,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"status":500,"message":"mapped error"}`))
			}))
			defer server.Close()

			client, err := NewClient(server.URL+"/v2", "secret-key")
			require.NoError(t, err)
			_, err = client.GetModel(context.Background(), "model-a", "aione", "development")
			var apiErr *APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, status, apiErr.Status)
		})
	}
}

func TestClientRedactsAPIKeyAndCodeSourceTokenFromErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"status":400,"message":"key sensitive-key token source-token"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v2", "sensitive-key")
	require.NoError(t, err)
	_, err = client.CreateModel(context.Background(), CreateModelRequest{
		Codes: []CodeSource{{ID: "repo", Token: "source-token"}},
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "sensitive-key")
	assert.NotContains(t, err.Error(), "source-token")
	assert.Contains(t, err.Error(), "[REDACTED]")
}
