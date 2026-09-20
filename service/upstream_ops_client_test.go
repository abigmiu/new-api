package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpstreamOpsClientContract(t *testing.T) {
	token := "contract-secret-token"
	requests := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.RequestURI() {
		case "GET /api/channels?page=1&page_size=-1":
			_, _ = w.Write([]byte(`{"data":{"items":[{"id":3,"name":"supplier","type":"new-api","site_url":"https://example.com","monitor_enabled":true}]}}`))
		case "POST /api/channels/3/refresh-rates":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "GET /api/channels/3/rates":
			_, _ = w.Write([]byte(`{"data":[{"id":4,"channel_id":3,"model_name":"pro","ratio":0.075,"completion_ratio":1}]}`))
		case "GET /api/channels/3/api-keys/groups":
			_, _ = w.Write([]byte(`{"data":[{"name":"pro","ratio":0.075}]}`))
		case "GET /api/channels/3/api-keys?page=2&page_size=100":
			_, _ = w.Write([]byte(`{"data":{"items":[{"id":8,"name":"managed","status":"active","group":"pro"}],"page":2,"pages":2}}`))
		case "POST /api/channels/3/api-keys":
			_, _ = w.Write([]byte(`{"data":{"id":9,"name":"created","status":"active","group":"pro"}}`))
		case "POST /api/channels/3/api-keys/9/reveal":
			_, _ = w.Write([]byte(`{"data":{"key":"sk-upstream-key"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := &UpstreamOpsClient{baseUrl: server.URL, token: token, http: server.Client()}
	ctx := context.Background()

	channels, err := client.ListChannels(ctx)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.True(t, channels[0].MonitorEnabled)
	require.NoError(t, client.RefreshRates(ctx, 3))
	rates, err := client.ListRates(ctx, 3)
	require.NoError(t, err)
	require.Len(t, rates, 1)
	assert.Equal(t, "0.075", rates[0].Ratio.String())
	groups, err := client.ListKeyGroups(ctx, 3)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, "pro", groups[0].Name)
	keys, err := client.ListKeys(ctx, 3, 2)
	require.NoError(t, err)
	require.Len(t, keys.Items, 1)
	created, err := client.CreateKey(ctx, 3, UpstreamOpsCreateKeyRequest{Name: "created", Group: "pro"})
	require.NoError(t, err)
	assert.Equal(t, int64(9), created.Id)
	revealed, err := client.RevealKey(ctx, 3, 9)
	require.NoError(t, err)
	assert.Equal(t, "sk-upstream-key", revealed)
	assert.Len(t, requests, 7)
}

func TestUpstreamOpsClientErrorsDoNotLeakSecrets(t *testing.T) {
	token := "header-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"body-secret-key"}`))
	}))
	defer server.Close()
	client := &UpstreamOpsClient{baseUrl: server.URL, token: token, http: server.Client()}

	_, err := client.ListChannels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 401")
	assert.False(t, strings.Contains(err.Error(), token))
	assert.False(t, strings.Contains(err.Error(), "body-secret-key"))
}

func TestUpstreamOpsClientRejectsOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", upstreamOpsMaxResponseBytes+1)))
	}))
	defer server.Close()
	client := &UpstreamOpsClient{baseUrl: server.URL, token: "test", http: server.Client()}

	_, err := client.ListChannels(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}
