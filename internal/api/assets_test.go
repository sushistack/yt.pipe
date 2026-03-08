package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func setupTestServerWithStore(t *testing.T) (*api.Server, *store.Store, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	st, err := store.New(":memory:")
	require.NoError(t, err)

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API:           config.APIConfig{Host: "localhost", Port: 8080},
	}
	srv := api.NewServer(st, cfg)
	return srv, st, func() { st.Close() }
}

func TestGenerateImages(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 10)

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
}

func TestGenerateImages_InvalidScene(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 5)

	body := `{"scenes":[1,99]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/images/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGenerateTTS(t *testing.T) {
	srv, st, cleanup := setupTestServerWithStore(t)
	defer cleanup()

	id := createProjectWithScenes(t, srv, st, "SCP-173", 10)

	body := `{"scenes":[5]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+id+"/tts/generate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
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

// Ensure the import for config and domain are used
var _ = domain.ValidAssetTypes
