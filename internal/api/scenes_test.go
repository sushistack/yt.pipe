package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createProjectAndApprovals sets up a project with scene approvals in the given store.
func createProjectAndApprovals(t *testing.T, st *store.Store, projectID, scpID string, scenes int, assetType string, markGenerated bool) {
	t.Helper()
	require.NoError(t, st.CreateProject(&domain.Project{
		ID: projectID, SCPID: scpID, Status: domain.StagePending,
		SceneCount: scenes, WorkspacePath: "/w",
	}))
	for i := 1; i <= scenes; i++ {
		require.NoError(t, st.InitApproval(projectID, i, assetType))
		if markGenerated {
			require.NoError(t, st.MarkGenerated(projectID, i, assetType))
		}
	}
}

// --- GET /api/v1/projects/{id}/scenes ---

func TestGetScenes_Success(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 3, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p1/scenes", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
}

func TestGetScenes_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/nonexistent/scenes", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Success)
	assert.Equal(t, "not_found", resp.Error.Code)
}

// --- POST /api/v1/projects/{id}/scenes/{num}/approve ---

func TestApproveScene_Success(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/approve?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "p1", data["project_id"])
	assert.Equal(t, float64(1), data["scene_num"])
	assert.Equal(t, "image", data["asset_type"])
	assert.Equal(t, "approved", data["status"])
}

func TestApproveScene_InvalidSceneNumber(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/abc/approve?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "bad_request", resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "invalid scene number")
}

func TestApproveScene_InvalidAssetType(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/approve?type=video", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "bad_request", resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "type must be")
}

func TestApproveScene_MissingType(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/approve", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApproveScene_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/scenes/1/approve?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestApproveScene_SceneNotFound(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/99/approve?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestApproveScene_InvalidTransition(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	// Create project with pending (not generated) approvals → can't approve from pending
	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, false)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/approve?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "CONFLICT", resp.Error.Code)
}

func TestApproveScene_TTS(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 1, domain.AssetTypeTTS, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/approve?type=tts", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "tts", data["asset_type"])
	assert.Equal(t, "approved", data["status"])
}

// --- POST /api/v1/projects/{id}/scenes/{num}/reject ---

func TestRejectScene_Success(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/reject?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "p1", data["project_id"])
	assert.Equal(t, float64(1), data["scene_num"])
	assert.Equal(t, "image", data["asset_type"])
	assert.Equal(t, "rejected", data["status"])
}

func TestRejectScene_InvalidSceneNumber(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/xyz/reject?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRejectScene_InvalidAssetType(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/reject?type=video", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRejectScene_SceneNotFound(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/99/reject?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRejectScene_InvalidTransition(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	// Pending → can't reject (must be generated first)
	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, false)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/reject?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "invalid_transition", resp.Error.Code)
}

func TestRejectScene_AlreadyApproved(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 1, domain.AssetTypeImage, true)

	// Approve first
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/approve?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Try to reject approved scene → conflict
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/reject?type=image", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

// --- Full workflow: approve → reject cycle ---

func TestSceneApproval_FullWorkflow(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 2, domain.AssetTypeImage, true)

	// Step 1: Reject scene 1
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/reject?type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Step 2: Regenerate (mark as generated again via store directly)
	require.NoError(t, st.MarkGenerated("p1", 1, domain.AssetTypeImage))

	// Step 3: Approve scene 1 after regeneration
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/scenes/1/approve?type=image", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "approved", data["status"])
}

// --- GET /api/v1/projects/{id}/preview ---

func TestBatchPreview_Success(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 3, domain.AssetTypeImage, true)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p1/preview?asset_type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)

	// Data should be an array
	items, ok := resp.Data.([]interface{})
	require.True(t, ok)
	assert.Len(t, items, 3)
}

func TestBatchPreview_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/nonexistent/preview?asset_type=image", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- POST /api/v1/projects/{id}/batch-approve ---

func TestBatchApprove_Success(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 4, domain.AssetTypeImage, true)

	body := `{"asset_type": "image", "flagged_scenes": [2, 4]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/batch-approve",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(2), data["approved_count"])
	assert.Equal(t, float64(2), data["flagged_count"])
}

func TestBatchApprove_EmptyFlags(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	createProjectAndApprovals(t, st, "p1", "SCP-173", 3, domain.AssetTypeImage, true)

	body := `{"asset_type": "image", "flagged_scenes": []}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/batch-approve",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(3), data["approved_count"])
	assert.Equal(t, float64(0), data["flagged_count"])
}

func TestBatchApprove_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	body := `{"asset_type": "image", "flagged_scenes": []}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/batch-approve",
		bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
