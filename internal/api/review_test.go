package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupReviewTestServer(t *testing.T) (*api.Server, *store.Store, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	st, err := store.New(":memory:")
	require.NoError(t, err)

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API: config.APIConfig{
			Host: "localhost",
			Port: 8080,
			Auth: config.AuthConfig{Enabled: true, Key: "test-api-key"},
		},
	}
	srv := api.NewServer(st, cfg)
	return srv, st, func() { st.Close() }
}

func createReviewTestProject(t *testing.T, st *store.Store, id, status, token string) string {
	t.Helper()
	tmpDir := t.TempDir()
	require.NoError(t, st.CreateProject(&domain.Project{
		ID: id, SCPID: "SCP-173", Status: status,
		SceneCount: 1, WorkspacePath: tmpDir, ReviewToken: token,
	}))
	return tmpDir
}

// --- Review Token Validation ---

func TestReviewPage_ValidToken(t *testing.T) {
	srv, st, cleanup := setupReviewTestServer(t)
	defer cleanup()

	createReviewTestProject(t, st, "p1", domain.StageScenario, "valid-token-123")

	req := httptest.NewRequest(http.MethodGet, "/review/p1?token=valid-token-123", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Should not be auth failure
	assert.NotEqual(t, http.StatusUnauthorized, w.Code)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

func TestReviewPage_MissingToken(t *testing.T) {
	srv, st, cleanup := setupReviewTestServer(t)
	defer cleanup()

	createReviewTestProject(t, st, "p1", domain.StageScenario, "valid-token-123")

	req := httptest.NewRequest(http.MethodGet, "/review/p1", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestReviewPage_InvalidToken(t *testing.T) {
	srv, st, cleanup := setupReviewTestServer(t)
	defer cleanup()

	createReviewTestProject(t, st, "p1", domain.StageScenario, "valid-token-123")

	req := httptest.NewRequest(http.MethodGet, "/review/p1?token=wrong-token", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestReviewPage_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupReviewTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/review/nonexistent?token=abc", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- CSRF Verification ---

func TestReviewApprove_CsrfRequired(t *testing.T) {
	srv, st, cleanup := setupReviewTestServer(t)
	defer cleanup()

	token := "review-token-abc"
	createReviewTestProject(t, st, "p1", domain.StageImages, token)
	require.NoError(t, st.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, st.MarkGenerated("p1", 1, domain.AssetTypeImage))

	// POST without X-Review-Token header — CSRF should fail
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/projects/p1/scenes/1/approve?type=image&token="+token, nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp.Error.Message, "CSRF")
}

func TestReviewApprove_WithCsrf_Success(t *testing.T) {
	srv, st, cleanup := setupReviewTestServer(t)
	defer cleanup()

	token := "review-token-abc"
	createReviewTestProject(t, st, "p1", domain.StageImages, token)
	require.NoError(t, st.InitApproval("p1", 1, domain.AssetTypeImage))
	require.NoError(t, st.MarkGenerated("p1", 1, domain.AssetTypeImage))

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/projects/p1/scenes/1/approve?type=image&token="+token, nil)
	req.Header.Set("X-Review-Token", token)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "approved", data["status"])
}

// --- Narration Update ---

func TestUpdateNarration_WithReviewToken(t *testing.T) {
	srv, st, cleanup := setupReviewTestServer(t)
	defer cleanup()

	token := "review-token-abc"
	tmpDir := t.TempDir()
	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageScenario,
		SceneCount: 1, WorkspacePath: tmpDir, ReviewToken: token,
	}))

	// Create scenario.json
	scenarioJSON := `{"Scenes":[{"SceneNum":1,"Narration":"old text"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "scenario.json"), []byte(scenarioJSON), 0o644))

	body := `{"narration":"Updated narration"}`
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/projects/p1/scenes/1/narration?token="+token,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Review-Token", token)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateNarration_EmptyText(t *testing.T) {
	srv, st, cleanup := setupReviewTestServer(t)
	defer cleanup()

	token := "review-token-abc"
	tmpDir := t.TempDir()
	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageScenario,
		SceneCount: 1, WorkspacePath: tmpDir, ReviewToken: token,
	}))

	scenarioJSON := `{"Scenes":[{"SceneNum":1,"Narration":"old text"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "scenario.json"), []byte(scenarioJSON), 0o644))

	body := `{"narration":""}`
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/projects/p1/scenes/1/narration?token="+token,
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Review-Token", token)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Token Rotation ---

func TestRotateReviewToken_RequiresBearer(t *testing.T) {
	srv, st, cleanup := setupReviewTestServer(t)
	defer cleanup()

	createReviewTestProject(t, st, "p1", domain.StageScenario, "old-token")

	// POST with review token (not bearer) — should fail
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/projects/p1/review-token/rotate?token=old-token", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRotateReviewToken_WithBearer(t *testing.T) {
	srv, st, cleanup := setupReviewTestServer(t)
	defer cleanup()

	createReviewTestProject(t, st, "p1", domain.StageScenario, "old-token")

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/projects/p1/review-token/rotate", nil)
	req.Header.Set("Authorization", "Bearer test-api-key")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.NotEqual(t, "old-token", data["review_token"])
	assert.NotEmpty(t, data["review_token"])
}

// --- BuildReviewURL ---

func TestBuildReviewURL(t *testing.T) {
	url := api.BuildReviewURL("proj-1", "token-abc")
	assert.Equal(t, "/review/proj-1?token=token-abc", url)
}

func TestBuildReviewURL_EmptyToken(t *testing.T) {
	url := api.BuildReviewURL("proj-1", "")
	assert.Empty(t, url)
}
