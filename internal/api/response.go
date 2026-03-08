package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Response is the standard API response envelope.
type Response struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data"`
	Error     *APIError   `json:"error"`
	Timestamp string      `json:"timestamp"`
	RequestID string      `json:"request_id"`
}

// APIError describes an error in the standard response.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON writes a standard success response.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	resp := Response{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: GetRequestID(r.Context()),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// WriteError writes a standard error response.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	resp := Response{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: GetRequestID(r.Context()),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}
