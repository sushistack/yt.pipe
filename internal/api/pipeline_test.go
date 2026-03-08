package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jay/youtube-pipeline/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestProject(t *testing.T, srv *api.Server, scpID string) string {
	t.Helper()
	body := `{"scp_id":"` + scpID + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp.Data.(map[string]interface{})["id"].(string)
}

func TestRunPipeline(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, projectID, data["project_id"])
	assert.Equal(t, "running", data["status"])
	assert.NotEmpty(t, data["job_id"])
}

func TestRunPipeline_DryRun(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	body := `{"dryRun":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestRunPipeline_NotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/run", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRunPipeline_DuplicateExecution(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	// First run
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)

	// Wait briefly for the goroutine to start but not finish
	// Then try running again immediately (may or may not conflict depending on timing)
	// The test primarily ensures no crash

	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	// Could be 202 (if previous job already completed) or 409 (if still running)
	assert.Contains(t, []int{http.StatusAccepted, http.StatusConflict}, w.Code)
}

func TestGetStatus(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	// Get status before any run
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/status", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, projectID, data["project_id"])
	assert.Equal(t, "pending", data["state"])
}

func TestGetStatus_NotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/nonexistent/status", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCancelPipeline(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	// Start a run
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusAccepted, w.Code)

	// Give goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel
	req = httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/cancel", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Could succeed or conflict if job already completed
	assert.Contains(t, []int{http.StatusOK, http.StatusConflict}, w.Code)
}

func TestCancelPipeline_NoRunning(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/cancel", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestApprovePipeline_WrongState(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	// Try to approve a pending project (should fail)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/approve", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestApprovePipeline_NotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/approve", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
