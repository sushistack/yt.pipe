package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// reviewScopedRoutes lists chi route patterns that review tokens may access.
// Token validation happens in the handlers themselves; this only controls
// which routes bypass Bearer auth when ?token= is present.
var reviewScopedRoutes = map[string]bool{
	// Review page
	"/review/{project_id}": true,
	// Scene asset serving
	"/api/v1/projects/{id}/scenes/{num}/image":                true,
	"/api/v1/projects/{id}/scenes/{num}/shots/{shotNum}/image": true,
	"/api/v1/projects/{id}/scenes/{num}/audio":                true,
	// Scene approval/reject
	"/api/v1/projects/{id}/scenes/{num}/approve": true,
	"/api/v1/projects/{id}/scenes/{num}/reject":  true,
	// Narration edit
	"/api/v1/projects/{id}/scenes/{num}/narration": true,
	// Scene CRUD
	"/api/v1/projects/{id}/scenes": true,
	// Scenario approve
	"/api/v1/projects/{id}/approve": true,
	// Bulk approve
	"/api/v1/projects/{id}/approve-all": true,
	// Batch preview & approve
	"/api/v1/projects/{id}/preview":       true,
	"/api/v1/projects/{id}/batch-approve": true,
}

// AuthMiddleware creates an authentication middleware using Bearer token.
// When auth is disabled, all requests pass through with a startup warning.
// /health and /ready are always exempt from authentication.
// Review-scoped paths with ?token= query parameter bypass Bearer auth
// (actual token validation is done in handlers).
func AuthMiddleware(enabled bool, apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health/ready endpoints and static files
			if r.URL.Path == "/health" || r.URL.Path == "/ready" || r.URL.Path == "/favicon.ico" || strings.HasPrefix(r.URL.Path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			// Skip if auth is disabled
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Check if this is a review-scoped path with a token query parameter.
			// Uses chi's RouteContext to get the matched route pattern for safe comparison.
			if r.URL.Query().Get("token") != "" {
				if isReviewScopedRequest(r) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Extract Bearer token
			auth := r.Header.Get("Authorization")
			if auth == "" {
				logAuthFailure(r, "missing authorization header")
				WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "missing authorization header")
				return
			}

			if !strings.HasPrefix(auth, "Bearer ") {
				logAuthFailure(r, "invalid authorization format")
				WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid authorization format; use Bearer token")
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			if subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
				logAuthFailure(r, "invalid api key")
				WriteError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "invalid api key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isReviewScopedRequest checks if the request matches a review-scoped route pattern.
// Uses chi's RouteContext when available; falls back to path-prefix matching
// for cases where the middleware runs before route resolution.
func isReviewScopedRequest(r *http.Request) bool {
	rctx := chi.RouteContext(r.Context())
	if rctx != nil {
		if pattern := rctx.RoutePattern(); pattern != "" {
			return reviewScopedRoutes[pattern]
		}
	}

	// Fallback: review page
	if strings.HasPrefix(r.URL.Path, "/review/") {
		return true
	}

	// Fallback: API review-scoped paths.
	// Match pattern: /api/v1/projects/{id}/scenes/... or /api/v1/projects/{id}/approve-all
	path := r.URL.Path
	if !strings.HasPrefix(path, "/api/v1/projects/") {
		return false
	}
	// Extract sub-path after /api/v1/projects/{id}/
	rest := path[len("/api/v1/projects/"):]
	if idx := strings.IndexByte(rest, '/'); idx >= 0 {
		suffix := rest[idx:]
		// Check known review-scoped suffixes
		return strings.HasPrefix(suffix, "/scenes") ||
			suffix == "/approve" ||
			suffix == "/approve-all"
	}
	return false
}

func logAuthFailure(r *http.Request, reason string) {
	// Extract client IP, preferring X-Forwarded-For
	clientIP := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		clientIP = strings.Split(xff, ",")[0]
	}

	// Log the key prefix only (never the full key)
	keyPrefix := ""
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		token := strings.TrimPrefix(auth, "Bearer ")
		if len(token) > 4 {
			keyPrefix = token[:4] + "..."
		} else if len(token) > 0 {
			keyPrefix = "***"
		}
	}

	slog.Warn("auth failure",
		"reason", reason,
		"client_ip", clientIP,
		"path", r.URL.Path,
		"method", r.Method,
		"key_prefix", keyPrefix,
		"request_id", GetRequestID(r.Context()),
	)
}
