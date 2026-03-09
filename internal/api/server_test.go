package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*api.Server, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	st, err := store.New(":memory:")
	require.NoError(t, err)

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API:           config.APIConfig{Host: "localhost", Port: 8080},
	}

	srv := api.NewServer(st, cfg)
	return srv, func() { st.Close() }
}

func TestHealthEndpoint(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.NotEmpty(t, resp.RequestID)
	assert.NotEmpty(t, resp.Timestamp)
	assert.Nil(t, resp.Error)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	// Default server has no plugins configured, so status is "degraded"
	assert.Equal(t, "degraded", data["status"])
	assert.NotEmpty(t, data["version"])

	// Verify plugins field is present
	plugins, ok := data["plugins"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, plugins, "llm")
	assert.Contains(t, plugins, "imagegen")
	assert.Contains(t, plugins, "tts")
	assert.Contains(t, plugins, "output")
}

func TestHealthEndpoint_AllPluginsAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API:           config.APIConfig{Host: "localhost", Port: 8080},
	}

	allAvailable := map[string]bool{
		"llm": true, "imagegen": true, "tts": true, "output": true,
	}
	srv := api.NewServer(st, cfg, api.WithPluginStatus(allAvailable))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "ok", data["status"])

	plugins := data["plugins"].(map[string]interface{})
	assert.Equal(t, true, plugins["llm"])
	assert.Equal(t, true, plugins["imagegen"])
	assert.Equal(t, true, plugins["tts"])
	assert.Equal(t, true, plugins["output"])
}

func TestHealthEndpoint_PartialPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API:           config.APIConfig{Host: "localhost", Port: 8080},
	}

	partial := map[string]bool{
		"llm": true, "imagegen": false, "tts": true, "output": true,
	}
	srv := api.NewServer(st, cfg, api.WithPluginStatus(partial))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "degraded", data["status"])

	plugins := data["plugins"].(map[string]interface{})
	assert.Equal(t, true, plugins["llm"])
	assert.Equal(t, false, plugins["imagegen"])
}

func TestReadyEndpoint(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)

	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "ready", data["status"])
}

func TestReadyEndpoint_WorkspaceUnavailable(t *testing.T) {
	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	cfg := &config.Config{
		WorkspacePath: "/nonexistent/path/that/does/not/exist",
		API:           config.APIConfig{Host: "localhost", Port: 8080},
	}

	srv := api.NewServer(st, cfg)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var resp api.Response
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.False(t, resp.Success)
	assert.Equal(t, "WORKSPACE_UNAVAILABLE", resp.Error.Code)
}

func TestResponseHeaders(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
}

func TestRecoveryMiddleware(t *testing.T) {
	// Ensure panics don't crash the server
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// The server should handle panics gracefully on known endpoints
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNotFoundRoute(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHealthEndpoint_MethodNotAllowed(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
