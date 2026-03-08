package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

// AuthMiddleware creates an authentication middleware using Bearer token.
// When auth is disabled, all requests pass through with a startup warning.
// /health and /ready are always exempt from authentication.
func AuthMiddleware(enabled bool, apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health/ready endpoints
			if r.URL.Path == "/health" || r.URL.Path == "/ready" {
				next.ServeHTTP(w, r)
				return
			}

			// Skip if auth is disabled
			if !enabled {
				next.ServeHTTP(w, r)
				return
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
