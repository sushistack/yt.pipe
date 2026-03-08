package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jay/youtube-pipeline/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateProject(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{"scp_id":"SCP-173"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Success)

	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "SCP-173", data["scp_id"])
	assert.Equal(t, "pending", data["status"])
	assert.NotEmpty(t, data["id"])
}

func TestCreateProject_MissingSCPID(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateProject_InvalidJSON(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListProjects(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create two projects
	for _, scp := range []string{"SCP-173", "SCP-682"} {
		body := `{"scp_id":"` + scp + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	}

	// List all
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	projects := data["projects"].([]interface{})
	assert.Len(t, projects, 2)
	assert.Equal(t, float64(2), data["total"])
}

func TestListProjects_FilterByState(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a project
	body := `{"scp_id":"SCP-173"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	// Filter by pending
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects?state=pending", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, float64(1), data["total"])

	// Filter by nonexistent state
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects?state=complete", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data = resp.Data.(map[string]interface{})
	assert.Equal(t, float64(0), data["total"])
}

func TestGetProject(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a project
	body := `{"scp_id":"SCP-173"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var createResp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	projectID := createResp.Data.(map[string]interface{})["id"].(string)

	// Get it
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, projectID, data["id"])
	assert.Equal(t, "SCP-173", data["scp_id"])
}

func TestGetProject_NotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteProject_Pending(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create a project
	body := `{"scp_id":"SCP-173"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var createResp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &createResp))
	projectID := createResp.Data.(map[string]interface{})["id"].(string)

	// Delete it (pending status should be allowed)
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify it's gone
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteProject_NotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListProjects_Pagination(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Create 3 projects
	for _, scp := range []string{"SCP-173", "SCP-682", "SCP-999"} {
		body := `{"scp_id":"` + scp + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
		w := httptest.NewRecorder()
		srv.Router().ServeHTTP(w, req)
		require.Equal(t, http.StatusCreated, w.Code)
	}

	// Page 1 with limit 2
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects?limit=2&offset=0", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	projects := data["projects"].([]interface{})
	assert.Len(t, projects, 2)
	assert.Equal(t, float64(3), data["total"])
	assert.Equal(t, float64(2), data["limit"])

	// Page 2
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects?limit=2&offset=2", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data = resp.Data.(map[string]interface{})
	projects = data["projects"].([]interface{})
	assert.Len(t, projects, 1)
}
