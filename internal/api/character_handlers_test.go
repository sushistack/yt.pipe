package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

func setupCharacterTestServer(t *testing.T) (*api.Server, *store.Store, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	st, err := store.New(":memory:")
	require.NoError(t, err)

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API: config.APIConfig{
			Host: "localhost",
			Port: 8080,
			Auth: config.AuthConfig{Enabled: false},
		},
	}

	characterSvc := service.NewCharacterService(st)
	srv := api.NewServer(st, cfg,
		api.WithCharacterService(characterSvc),
		api.WithPluginStatus(map[string]bool{
			"llm": true, "imagegen": true, "tts": true, "output": true,
		}),
	)
	return srv, st, func() { st.Close() }
}

func createCharacterTestProject(t *testing.T, st *store.Store, id, scpID, status string) {
	t.Helper()
	require.NoError(t, st.CreateProject(&domain.Project{
		ID: id, SCPID: scpID, Status: status, WorkspacePath: t.TempDir(),
	}))
}

// --- handleGenerateCharacters ---

func TestGenerateCharacters_StageGuard_Pending(t *testing.T) {
	srv, st, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	createCharacterTestProject(t, st, "p1", "SCP-173", domain.StagePending)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/characters/generate", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_STAGE")
}

func TestGenerateCharacters_StageGuard_Images(t *testing.T) {
	srv, st, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	createCharacterTestProject(t, st, "p1", "SCP-173", domain.StageImages)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/characters/generate", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestGenerateCharacters_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/characters/generate", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGenerateCharacters_AcceptedFromScenarioStage(t *testing.T) {
	srv, st, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	createCharacterTestProject(t, st, "p1", "SCP-173", domain.StageScenario)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/characters/generate", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	assert.NotEmpty(t, data["job_id"])
	assert.Equal(t, "character_generate", data["type"])
}

// --- handleListCandidates ---

func TestListCandidates_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/nonexistent/characters/candidates", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListCandidates_Empty(t *testing.T) {
	srv, st, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	createCharacterTestProject(t, st, "p1", "SCP-173", domain.StageCharacter)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p1/characters/candidates", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "empty", data["status"])
}

func TestListCandidates_WithCandidates(t *testing.T) {
	srv, st, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	createCharacterTestProject(t, st, "p1", "SCP-173", domain.StageCharacter)
	require.NoError(t, st.CreateCandidateBatch("p1", "SCP-173", 4))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p1/characters/candidates", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	data := resp["data"].(map[string]interface{})
	assert.Equal(t, "generating", data["status"])
	candidates := data["candidates"].([]interface{})
	assert.Len(t, candidates, 4)
}

// --- handleSelectCharacter ---

func TestSelectCharacter_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"candidate_num": 1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/characters/select", body)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSelectCharacter_InvalidCandidateNum(t *testing.T) {
	srv, st, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	createCharacterTestProject(t, st, "p1", "SCP-173", domain.StageCharacter)

	body := strings.NewReader(`{"candidate_num": 0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/characters/select", body)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_CANDIDATE")
}

func TestSelectCharacter_StageGuard(t *testing.T) {
	srv, st, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	createCharacterTestProject(t, st, "p1", "SCP-173", domain.StageImages)

	body := strings.NewReader(`{"candidate_num": 1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/p1/characters/select", body)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_STAGE")
}

// --- handleGetCharacter ---

func TestGetCharacter_ProjectNotFound(t *testing.T) {
	srv, _, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/nonexistent/characters", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetCharacter_NoCharacter(t *testing.T) {
	srv, st, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	createCharacterTestProject(t, st, "p1", "SCP-173", domain.StageCharacter)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p1/characters", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetCharacter_Exists(t *testing.T) {
	srv, st, cleanup := setupCharacterTestServer(t)
	defer cleanup()

	createCharacterTestProject(t, st, "p1", "SCP-173", domain.StageCharacter)
	require.NoError(t, st.CreateCharacter(&domain.Character{
		ID: "c1", SCPID: "SCP-173", CanonicalName: "SCP-173",
		Aliases: []string{}, SelectedImagePath: "/img/char.png",
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/p1/characters", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "SCP-173")
}
