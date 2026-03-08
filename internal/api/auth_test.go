package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jay/youtube-pipeline/internal/api"
	"github.com/jay/youtube-pipeline/internal/config"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAuthServer(t *testing.T, authEnabled bool, apiKey string) (*api.Server, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	st, err := store.New(":memory:")
	require.NoError(t, err)

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API: config.APIConfig{
			Host: "localhost",
			Port: 8080,
			Auth: config.AuthConfig{
				Enabled: authEnabled,
				Key:     apiKey,
			},
		},
	}
	srv := api.NewServer(st, cfg)
	return srv, func() { st.Close() }
}

func TestAuth_HealthExempt(t *testing.T) {
	srv, cleanup := setupAuthServer(t, true, "secret-key-123")
	defer cleanup()

	// Health should work without auth
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_ReadyExempt(t *testing.T) {
	srv, cleanup := setupAuthServer(t, true, "secret-key-123")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_RequiredForAPI(t *testing.T) {
	srv, cleanup := setupAuthServer(t, true, "secret-key-123")
	defer cleanup()

	// No auth header → 401
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "UNAUTHORIZED", resp.Error.Code)
}

func TestAuth_InvalidKey(t *testing.T) {
	srv, cleanup := setupAuthServer(t, true, "secret-key-123")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ValidKey(t *testing.T) {
	srv, cleanup := setupAuthServer(t, true, "secret-key-123")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer secret-key-123")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_InvalidFormat(t *testing.T) {
	srv, cleanup := setupAuthServer(t, true, "secret-key-123")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_DisabledAllowsAll(t *testing.T) {
	srv, cleanup := setupAuthServer(t, false, "")
	defer cleanup()

	// No auth header but auth is disabled → should pass
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAuth_EmptyBearerToken(t *testing.T) {
	srv, cleanup := setupAuthServer(t, true, "secret-key-123")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
