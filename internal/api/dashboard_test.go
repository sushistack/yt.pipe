package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDashboardServer(t *testing.T, authEnabled bool, apiKey string) (*api.Server, *store.Store, func()) {
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
	return srv, st, func() { st.Close() }
}

// --- Dashboard List Page ---

func TestDashboard_List_ReturnsHTML(t *testing.T) {
	srv, _, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "<!DOCTYPE html>")
	assert.Contains(t, w.Body.String(), "Projects")
}

func TestDashboard_List_ShowsProjects(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w/1",
	}))
	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p2", SCPID: "SCP-682", Status: domain.StageComplete, WorkspacePath: "/w/2",
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "SCP-173")
	assert.Contains(t, body, "SCP-682")
}

func TestDashboard_List_FilterByStage(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w/1",
	}))
	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p2", SCPID: "SCP-682", Status: domain.StageComplete, WorkspacePath: "/w/2",
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/?stage=pending", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "SCP-173")
	assert.NotContains(t, body, "SCP-682")
}

func TestDashboard_List_FilterBySCP(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w/1",
	}))
	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p2", SCPID: "SCP-682", Status: domain.StagePending, WorkspacePath: "/w/2",
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/?scp=SCP-173", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "SCP-173")
	assert.NotContains(t, body, "SCP-682")
}

func TestDashboard_List_EmptyState(t *testing.T) {
	srv, _, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "No projects found")
}

func TestDashboard_List_HTMXPartial(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w/1",
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/?stage=pending", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// HTMX partial should NOT contain full HTML layout
	assert.NotContains(t, body, "<!DOCTYPE html>")
	// But should contain project data
	assert.Contains(t, body, "SCP-173")
}

// --- Project Detail Page ---

func TestDashboard_Detail_ReturnsHTML(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageScenario, WorkspacePath: "/w/1", SceneCount: 3,
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/projects/p1", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	body := w.Body.String()
	assert.Contains(t, body, "SCP-173")
	assert.Contains(t, body, "scenario")
}

func TestDashboard_Detail_NotFound(t *testing.T) {
	srv, _, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/dashboard/projects/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDashboard_Detail_ProgressBar(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageImages, WorkspacePath: "/w/1",
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/projects/p1", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// Progress bar should show all stages
	assert.Contains(t, body, "pending")
	assert.Contains(t, body, "scenario")
	assert.Contains(t, body, "images")
	assert.Contains(t, body, "tts")
	assert.Contains(t, body, "complete")
}

func TestDashboard_Detail_PipelineButtons(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageScenario, WorkspacePath: "/w/1",
	}))

	req := httptest.NewRequest(http.MethodGet, "/dashboard/projects/p1", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Generate Images")
	assert.Contains(t, body, "Generate TTS")
	assert.Contains(t, body, "Assemble")
	assert.Contains(t, body, "Delete")
}

// --- Auth ---

func TestDashboard_Auth_RequiresBearerToken(t *testing.T) {
	srv, _, cleanup := setupDashboardServer(t, true, "my-secret-key")
	defer cleanup()

	// No auth → 401
	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// With auth → 200
	req = httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	req.Header.Set("Authorization", "Bearer my-secret-key")
	w = httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDashboard_Auth_ReviewTokenNotAccepted(t *testing.T) {
	srv, _, cleanup := setupDashboardServer(t, true, "my-secret-key")
	defer cleanup()

	// Review token should not grant dashboard access
	req := httptest.NewRequest(http.MethodGet, "/dashboard/?token=some-review-token", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDashboard_Static_AuthExempt(t *testing.T) {
	srv, _, cleanup := setupDashboardServer(t, true, "my-secret-key")
	defer cleanup()

	// Static files should be accessible without auth
	req := httptest.NewRequest(http.MethodGet, "/static/htmx.min.js", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "htmx")
}

// --- Stage API (HTMX + JSON) ---

func TestSetStage_JSON(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w/1",
	}))

	body := `{"stage":"scenario"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/p1/stage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "scenario", data["status"])
}

func TestSetStage_InvalidStage(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w/1",
	}))

	body := `{"stage":"foo"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/p1/stage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSetStage_BackwardTransition(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StageImages, WorkspacePath: "/w/1",
	}))

	// Set stage backward to scenario — should succeed (no state-machine gating)
	body := `{"stage":"scenario"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/p1/stage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp api.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data := resp.Data.(map[string]interface{})
	assert.Equal(t, "scenario", data["status"])
}

func TestSetStage_HTMX_ReturnsPartial(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	require.NoError(t, st.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w/1",
	}))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/p1/stage", strings.NewReader("stage=scenario"))
	req.Header.Set("HX-Request", "true")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// Should return HTML partial, not JSON
	assert.NotContains(t, body, `"success"`)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

// --- Pagination ---

func TestDashboard_List_Pagination(t *testing.T) {
	srv, st, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	// Create 25 projects (exceeds default page size of 20)
	for i := 0; i < 25; i++ {
		require.NoError(t, st.CreateProject(&domain.Project{
			ID: fmt.Sprintf("p%d", i), SCPID: fmt.Sprintf("SCP-%d", i),
			Status: domain.StagePending, WorkspacePath: fmt.Sprintf("/w/%d", i),
		}))
	}

	// Page 1 should have "Load more" button
	req := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Load more")
	assert.Contains(t, w.Body.String(), "25 SCP groups")
}

// --- Dashboard without trailing slash ---

func TestDashboard_NoTrailingSlash_404(t *testing.T) {
	srv, _, cleanup := setupDashboardServer(t, false, "")
	defer cleanup()

	// /dashboard (no trailing slash) is a different route
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	// chi may redirect or 404 — either is acceptable
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusMovedPermanently)
}
