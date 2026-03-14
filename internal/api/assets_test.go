package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
)

func createProjectWithScenes(t *testing.T, srv *api.Server, st *store.Store, scpID string, sceneCount int) string {
	t.Helper()
	id := createTestProject(t, srv, scpID)
	// Update scene count directly in DB
	p, err := st.GetProject(id)
	require.NoError(t, err)
	p.SceneCount = sceneCount
	require.NoError(t, st.UpdateProject(p))
	return id
}

// createApprovedProjectWithScenes creates a project with scenes set to "approved" state.
func createApprovedProjectWithScenes(t *testing.T, srv *api.Server, st *store.Store, scpID string, sceneCount int) string {
	t.Helper()
	id := createProjectWithScenes(t, srv, st, scpID, sceneCount)
	p, err := st.GetProject(id)
	require.NoError(t, err)
	p.Status = domain.StageImages
	require.NoError(t, st.UpdateProject(p))
	return id
}

func setupTestServerWithStore(t *testing.T, opts ...api.ServerOption) (*api.Server, *store.Store, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	st, err := store.New(":memory:")
	require.NoError(t, err)

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API:           config.APIConfig{Host: "localhost", Port: 8080},
	}
	srv := api.NewServer(st, cfg, opts...)
	return srv, st, func() { st.Close() }
}

func allPluginsAvailable() api.ServerOption {
	return api.WithPluginStatus(map[string]bool{
		"llm": true, "imagegen": true, "tts": true, "output": true,
	})
}

func TestGenerateImages_PluginUnavailable(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 10)

	body := `{"scenes":[3]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/images/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "API_UPSTREAM_ERROR", resp.Error.Code)
}

func TestGenerateTTS_PluginUnavailable(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 10)

	body := `{"scenes":[1]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/tts/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "API_UPSTREAM_ERROR", resp.Error.Code)
}

func TestGenerateImages(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createApprovedProjectWithScenes(t, srv, st, "SCP-173", 10)

	body := `{"scenes":[3,5,7]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/images/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "image_generate", data["type"])
	assert.NotEmpty(t, data["job_id"])

	// Verify scenes list in response
	scenes := data["scenes"].([]interface{})
	assert.Len(t, scenes, 3)
}

func TestGenerateImages_InvalidScene(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createApprovedProjectWithScenes(t, srv, st, "SCP-173", 5)

	body := `{"scenes":[1,99]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/images/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGenerateImages_WrongState(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	// Project is in "pending" state (not approved)
	id := createProjectWithScenes(t, srv, st, "SCP-173", 10)

	body := `{"scenes":[3]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/images/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "CONFLICT", resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "pending")
}

func TestGenerateImages_EmptyScenes_GeneratesAll(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createApprovedProjectWithScenes(t, srv, st, "SCP-173", 5)

	body := `{"scenes":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/images/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})

	// Should return all 5 scenes
	scenes := data["scenes"].([]interface{})
	assert.Len(t, scenes, 5)
	assert.Equal(t, float64(1), scenes[0])
	assert.Equal(t, float64(5), scenes[4])
}

func TestGenerateImages_DuplicateConflict(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createApprovedProjectWithScenes(t, srv, st, "SCP-173", 10)

	// Create a running job in DB to simulate an active generation
	runningJob := &domain.Job{
		ID:        "existing-image-job",
		ProjectID: id,
		Type:      "image_generate",
		Status:    "running",
	}
	require.NoError(t, st.CreateJob(runningJob))

	body := `{"scenes":[3]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/images/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "CONFLICT", resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "existing-image-job")
}

func TestGenerateImages_ImageReviewState(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 5)
	// Set to image_review state
	p, err := st.GetProject(id)
	require.NoError(t, err)
	p.Status = domain.StageImages
	require.NoError(t, st.UpdateProject(p))

	body := `{"scenes":[1,2]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/images/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestGenerateTTS(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createApprovedProjectWithScenes(t, srv, st, "SCP-173", 10)

	body := `{"scenes":[5]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/tts/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "tts_generate", data["type"])
	assert.NotEmpty(t, data["job_id"])
}

func TestGenerateTTS_WrongState(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	// Project is in "pending" state
	id := createProjectWithScenes(t, srv, st, "SCP-173", 10)

	body := `{"scenes":[1]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/tts/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "CONFLICT", resp.Error.Code)
}

func TestGenerateTTS_EmptyScenes_GeneratesAll(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createApprovedProjectWithScenes(t, srv, st, "SCP-173", 3)

	body := `{"scenes":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/tts/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})

	scenes := data["scenes"].([]interface{})
	assert.Len(t, scenes, 3)
}

func TestGenerateTTS_DuplicateConflict(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createApprovedProjectWithScenes(t, srv, st, "SCP-173", 10)

	// Create a running TTS job in DB
	runningJob := &domain.Job{
		ID:        "existing-tts-job",
		ProjectID: id,
		Type:      "tts_generate",
		Status:    "running",
	}
	require.NoError(t, st.CreateJob(runningJob))

	body := `{"scenes":[1]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/tts/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "existing-tts-job", data["job_id"])
	assert.Equal(t, true, data["already_running"])
}

func TestGenerateTTS_TTSReviewState(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 5)
	p, err := st.GetProject(id)
	require.NoError(t, err)
	p.Status = domain.StageTTS
	require.NoError(t, st.UpdateProject(p))

	body := `{"scenes":[1]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/tts/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestGenerateImages_JobCreatedInDB(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createApprovedProjectWithScenes(t, srv, st, "SCP-173", 5)

	body := `{"scenes":[1,2]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/images/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	jobID := data["job_id"].(string)

	// Wait briefly for background goroutine to start
	time.Sleep(50 * time.Millisecond)

	// Verify job exists in DB
	j, err := st.GetJob(jobID)
	require.NoError(t, err)
	assert.Equal(t, "image_generate", j.Type)
	assert.Equal(t, id, j.ProjectID)
}

func TestUpdatePrompt(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 10)

	body := `{"prompt":"A dark containment cell with flickering lights"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+id+"/scenes/3/prompt", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, true, data["updated"])
	assert.Equal(t, float64(3), data["scene_num"])
	assert.NotEmpty(t, data["updated_at"])

	// Verify the prompt was persisted to disk
	p, err := st.GetProject(id)
	require.NoError(t, err)
	if p.WorkspacePath != "" {
		promptPath := filepath.Join(p.WorkspacePath, "scenes", "3", "prompt.txt")
		fileData, readErr := os.ReadFile(promptPath)
		require.NoError(t, readErr)
		assert.Equal(t, "A dark containment cell with flickering lights", string(fileData))
	}
}

func TestUpdatePrompt_PersistsToWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API:           config.APIConfig{Host: "localhost", Port: 8080},
	}
	srv := api.NewServer(st, cfg)

	id := createTestProject(t, srv, "SCP-173")
	p, err := st.GetProject(id)
	require.NoError(t, err)
	p.SceneCount = 5
	require.NoError(t, st.UpdateProject(p))

	promptText := "A dark containment cell with flickering lights"
	body := `{"prompt":"` + promptText + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+id+"/scenes/2/prompt", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify file was persisted in project's workspace path
	// WorkspacePath is now a unique subdirectory created by workspace.InitProject
	p, err = st.GetProject(id)
	require.NoError(t, err)
	promptPath := filepath.Join(p.WorkspacePath, "scenes", "2", "prompt.txt")
	data, err := os.ReadFile(promptPath)
	require.NoError(t, err)
	assert.Equal(t, promptText, string(data))
}

func TestUpdatePrompt_InvalidatesContentHash(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API:           config.APIConfig{Host: "localhost", Port: 8080},
	}
	srv := api.NewServer(st, cfg)

	id := createTestProject(t, srv, "SCP-173")
	p, err := st.GetProject(id)
	require.NoError(t, err)
	p.SceneCount = 5
	require.NoError(t, st.UpdateProject(p))

	// Create a manifest with a content hash
	manifest := &domain.SceneManifest{
		ProjectID:   id,
		SceneNum:    2,
		ContentHash: "abc123",
		Status:      "complete",
	}
	require.NoError(t, st.CreateManifest(manifest))

	body := `{"prompt":"new prompt text"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+id+"/scenes/2/prompt", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify content hash was invalidated
	updated, err := st.GetManifest(id, 2)
	require.NoError(t, err)
	assert.Empty(t, updated.ContentHash, "content hash should be invalidated after prompt update")
}

func TestUpdatePrompt_InvalidScene(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 5)

	body := `{"prompt":"test"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+id+"/scenes/99/prompt", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdatePrompt_EmptyPrompt(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 5)

	body := `{"prompt":""}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+id+"/scenes/1/prompt", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateFeedback(t *testing.T) {
	srv, _, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createTestProject(t, srv, "SCP-173")

	body := `{"scene_num":1,"asset_type":"image","rating":"good","comment":"looks great"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/feedback", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "image", data["asset_type"])
	assert.Equal(t, "good", data["rating"])
}

func TestCreateFeedback_InvalidAssetType(t *testing.T) {
	srv, _, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createTestProject(t, srv, "SCP-173")

	body := `{"scene_num":1,"asset_type":"invalid","rating":"good"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/feedback", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateFeedback_InvalidRating(t *testing.T) {
	srv, _, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createTestProject(t, srv, "SCP-173")

	body := `{"scene_num":1,"asset_type":"image","rating":"amazing"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/feedback", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateFeedback_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	body := `{"scene_num":1,"asset_type":"image","rating":"good"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/feedback", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Assembly endpoint tests ---

func TestAssemble_PluginUnavailable(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 5)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/assemble", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "API_UPSTREAM_ERROR", resp.Error.Code)
}

func TestAssemble_WrongState(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	// Project is in "pending" state
	id := createProjectWithScenes(t, srv, st, "SCP-173", 5)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/assemble", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "INVALID_STATE", resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "pending")
}

func TestAssemble_UnapprovedScenes(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 3)
	// Set to tts_review state (valid for assembly)
	p, err := st.GetProject(id)
	require.NoError(t, err)
	p.Status = domain.StageTTS
	require.NoError(t, st.UpdateProject(p))

	// Init approvals but don't approve them
	require.NoError(t, st.InitApproval(id, 1, domain.AssetTypeImage))
	require.NoError(t, st.InitApproval(id, 2, domain.AssetTypeImage))
	require.NoError(t, st.InitApproval(id, 1, domain.AssetTypeTTS))
	require.NoError(t, st.InitApproval(id, 2, domain.AssetTypeTTS))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/assemble", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "INVALID_STATE", resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "not all scenes")
}

func TestAssemble_PartiallyApproved(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 2)
	p, err := st.GetProject(id)
	require.NoError(t, err)
	p.Status = domain.StageTTS
	require.NoError(t, st.UpdateProject(p))

	// Approve all images but not all TTS
	require.NoError(t, st.InitApproval(id, 1, domain.AssetTypeImage))
	require.NoError(t, st.InitApproval(id, 2, domain.AssetTypeImage))
	require.NoError(t, st.MarkGenerated(id, 1, domain.AssetTypeImage))
	require.NoError(t, st.ApproveScene(id, 1, domain.AssetTypeImage))
	require.NoError(t, st.MarkGenerated(id, 2, domain.AssetTypeImage))
	require.NoError(t, st.ApproveScene(id, 2, domain.AssetTypeImage))

	require.NoError(t, st.InitApproval(id, 1, domain.AssetTypeTTS))
	require.NoError(t, st.InitApproval(id, 2, domain.AssetTypeTTS))
	// Only approve scene 1 TTS
	require.NoError(t, st.MarkGenerated(id, 1, domain.AssetTypeTTS))
	require.NoError(t, st.ApproveScene(id, 1, domain.AssetTypeTTS))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/assemble", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "INVALID_STATE", resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "TTS")
}

func TestAssemble_AllApproved_CreatesJob(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 2)
	p, err := st.GetProject(id)
	require.NoError(t, err)
	p.Status = domain.StageTTS
	require.NoError(t, st.UpdateProject(p))

	// Approve all scenes for both image and TTS
	for _, sceneNum := range []int{1, 2} {
		require.NoError(t, st.InitApproval(id, sceneNum, domain.AssetTypeImage))
		require.NoError(t, st.MarkGenerated(id, sceneNum, domain.AssetTypeImage))
		require.NoError(t, st.ApproveScene(id, sceneNum, domain.AssetTypeImage))

		require.NoError(t, st.InitApproval(id, sceneNum, domain.AssetTypeTTS))
		require.NoError(t, st.MarkGenerated(id, sceneNum, domain.AssetTypeTTS))
		require.NoError(t, st.ApproveScene(id, sceneNum, domain.AssetTypeTTS))
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/assemble", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "assembly", data["type"])
	assert.NotEmpty(t, data["job_id"])
	assert.Equal(t, id, data["project_id"])

	// Verify job exists in DB
	time.Sleep(50 * time.Millisecond)
	jobID := data["job_id"].(string)
	j, err := st.GetJob(jobID)
	require.NoError(t, err)
	assert.Equal(t, "assembly", j.Type)
	assert.Equal(t, id, j.ProjectID)
}

func TestAssemble_DuplicateConflict(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 2)
	p, err := st.GetProject(id)
	require.NoError(t, err)
	p.Status = domain.StageTTS
	require.NoError(t, st.UpdateProject(p))

	// Approve all scenes
	for _, sceneNum := range []int{1, 2} {
		require.NoError(t, st.InitApproval(id, sceneNum, domain.AssetTypeImage))
		require.NoError(t, st.MarkGenerated(id, sceneNum, domain.AssetTypeImage))
		require.NoError(t, st.ApproveScene(id, sceneNum, domain.AssetTypeImage))
		require.NoError(t, st.InitApproval(id, sceneNum, domain.AssetTypeTTS))
		require.NoError(t, st.MarkGenerated(id, sceneNum, domain.AssetTypeTTS))
		require.NoError(t, st.ApproveScene(id, sceneNum, domain.AssetTypeTTS))
	}

	// Create a running assembly job in DB
	runningJob := &domain.Job{
		ID:        "existing-assembly-job",
		ProjectID: id,
		Type:      "assembly",
		Status:    "running",
	}
	require.NoError(t, st.CreateJob(runningJob))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/assemble", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "existing-assembly-job", data["job_id"])
	assert.Equal(t, true, data["already_running"])
}

func TestAssemble_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupTestServerWithStore(t, allPluginsAvailable())
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/assemble", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// Ensure the import for config and domain are used
var _ = domain.ValidAssetTypes
