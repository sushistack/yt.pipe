package api

import (
	"net/http"
	"os"
)

// handleHealth returns server health status including plugin availability.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	for _, available := range s.pluginStatus {
		if !available {
			status = "degraded"
			break
		}
	}

	WriteJSON(w, r, http.StatusOK, map[string]interface{}{
		"status":  status,
		"version": s.version,
		"plugins": s.pluginStatus,
	})
}

// handleReady checks if the server is ready to serve requests.
// Verifies SQLite connectivity and workspace directory existence.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	// Check database connectivity
	if _, err := s.store.SchemaVersion(); err != nil {
		WriteError(w, r, http.StatusServiceUnavailable, "DB_UNAVAILABLE", "database connection failed")
		return
	}

	// Check workspace directory
	if info, err := os.Stat(s.workspacePath); err != nil || !info.IsDir() {
		WriteError(w, r, http.StatusServiceUnavailable, "WORKSPACE_UNAVAILABLE", "workspace directory not accessible")
		return
	}

	WriteJSON(w, r, http.StatusOK, map[string]string{
		"status": "ready",
	})
}
