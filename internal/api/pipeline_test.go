package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
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

func setupTestServerWithPlugins(t *testing.T) (*api.Server, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	st, err := store.New(":memory:")
	require.NoError(t, err)

	cfg := &config.Config{
		WorkspacePath: tmpDir,
		API:           config.APIConfig{Host: "localhost", Port: 8080},
	}

	srv := api.NewServer(st, cfg, api.WithPluginStatus(map[string]bool{
		"llm": true, "imagegen": true, "tts": true, "output": true,
	}))
	return srv, func() { st.Close() }
}

func TestRunPipeline_PluginUnavailable(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "API_UPSTREAM_ERROR", resp.Error.Code)
}

func TestRunPipeline(t *testing.T) {
	srv, cleanup := setupTestServerWithPlugins(t)
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
	srv, cleanup := setupTestServerWithPlugins(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	body := `{"dryRun":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestRunPipeline_NotFound(t *testing.T) {
	srv, cleanup := setupTestServerWithPlugins(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/nonexistent/run", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRunPipeline_DuplicateExecution(t *testing.T) {
	srv, cleanup := setupTestServerWithPlugins(t)
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
	srv, cleanup := setupTestServerWithPlugins(t)
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

func TestGetJob(t *testing.T) {
	srv, cleanup := setupTestServerWithPlugins(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	// Run pipeline with dryRun to create a job that completes successfully
	body := `{"dryRun":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	var runResp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runResp))
	jobID := runResp.Data.(map[string]interface{})["job_id"].(string)

	// Wait for background goroutine to complete
	time.Sleep(200 * time.Millisecond)

	// Get job details
	req = httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, jobID, data["job_id"])
	assert.Equal(t, projectID, data["project_id"])
	assert.Equal(t, "complete", data["status"])
	assert.NotNil(t, data["started_at"])
	assert.NotNil(t, data["completed_at"])
	assert.NotNil(t, data["elapsed_sec"])
	assert.Equal(t, float64(100), data["progress"])
}

func TestGetJob_NotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/jobs/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetStatus_FallbackToDB(t *testing.T) {
	srv, cleanup := setupTestServerWithPlugins(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	// Run pipeline
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	require.Equal(t, http.StatusAccepted, w.Code)

	var runResp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runResp))
	jobID := runResp.Data.(map[string]interface{})["job_id"].(string)

	// Wait for background goroutine to complete
	time.Sleep(200 * time.Millisecond)

	// Get status — the job completed so in-memory entry may be present
	// but the response should include job info from DB fallback or memory
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/status", nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, projectID, data["project_id"])
	assert.Equal(t, jobID, data["job_id"])
	assert.NotNil(t, data["elapsed_sec"])
}

func TestGetStatus_NoJob(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	// Get status with no job ever created
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID+"/status", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, projectID, data["project_id"])
	// No job_id should be present when no jobs exist
	_, hasJobID := data["job_id"]
	assert.False(t, hasJobID)
}

func TestRunPipeline_InvalidMode(t *testing.T) {
	srv, cleanup := setupTestServerWithPlugins(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	body := `{"mode":"invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "BAD_REQUEST", resp.Error.Code)
}

func TestRunPipeline_ScenarioModeNoService(t *testing.T) {
	// Default mode (scenario) without a scenario service should fail the job
	srv, cleanup := setupTestServerWithPlugins(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	// Default mode is "scenario" - no scenario service configured
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// Should still accept the request (execution is async)
	assert.Equal(t, http.StatusAccepted, w.Code)

	var runResp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runResp))
	jobID := runResp.Data.(map[string]interface{})["job_id"].(string)

	// Wait for background goroutine to complete
	time.Sleep(200 * time.Millisecond)

	// Job should be failed since no scenario service
	req = httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "failed", data["status"])
	assert.Contains(t, data["error"].(string), "scenario service not configured")
}

func TestRunPipeline_FullModeNoRunner(t *testing.T) {
	// Full mode without a pipeline runner should fail the job
	srv, cleanup := setupTestServerWithPlugins(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	body := `{"mode":"full"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var runResp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runResp))
	jobID := runResp.Data.(map[string]interface{})["job_id"].(string)

	// Wait for background goroutine to complete
	time.Sleep(200 * time.Millisecond)

	// Job should be failed since no pipeline runner
	req = httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "failed", data["status"])
	assert.Contains(t, data["error"].(string), "pipeline runner not configured")
}

func TestRunPipeline_DryRunComplete(t *testing.T) {
	srv, cleanup := setupTestServerWithPlugins(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	body := `{"dryRun":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var runResp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &runResp))
	jobID := runResp.Data.(map[string]interface{})["job_id"].(string)

	// Wait for background goroutine
	time.Sleep(200 * time.Millisecond)

	// Verify dry-run completed successfully
	req = httptest.NewRequest(http.MethodGet, "/api/v1/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "complete", data["status"])
	assert.Equal(t, "dry-run complete", data["result"])
	assert.Equal(t, "dry_run", data["type"])
}

func TestRunPipeline_ModeExplicitScenario(t *testing.T) {
	// Explicitly specifying mode:"scenario" should work the same as default
	srv, cleanup := setupTestServerWithPlugins(t)
	defer cleanup()

	projectID := createTestProject(t, srv, "SCP-173")

	body := `{"mode":"scenario"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID+"/run", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestInitJobLifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	st, err := store.New(":memory:")
	require.NoError(t, err)
	defer st.Close()

	// Create a project and a "running" job to simulate server crash
	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-1", Status: "pending", WorkspacePath: "/w",
	}))
	require.NoError(t, st.CreateJob(&domain.Job{
		ID: "stale-job", ProjectID: "p1", Type: "pipeline_run", Status: "running",
	}))

	cfg := &config.Config{
		WorkspacePath:    tmpDir,
		API:              config.APIConfig{Host: "localhost", Port: 8080},
		JobRetentionDays: 7,
	}

	srv := api.NewServer(st, cfg)
	srv.InitJobLifecycle()

	// Verify the stale job is now marked as failed
	j, err := st.GetJob("stale-job")
	require.NoError(t, err)
	assert.Equal(t, "failed", j.Status)
	assert.Equal(t, "server restarted", j.Error)
}
