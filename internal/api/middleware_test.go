package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetRequestID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey, "test-id-123")
	assert.Equal(t, "test-id-123", GetRequestID(ctx))
}

func TestGetRequestID_Missing(t *testing.T) {
	assert.Equal(t, "", GetRequestID(context.Background()))
}

func TestRequestIDMiddleware(t *testing.T) {
	var capturedID string
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should set X-Request-ID header
	headerID := rr.Header().Get("X-Request-ID")
	assert.NotEmpty(t, headerID)

	// Should be a valid UUID format (36 chars with hyphens)
	assert.Len(t, headerID, 36)

	// Context should contain the same ID
	assert.Equal(t, headerID, capturedID)
}

func TestRequestIDMiddleware_UniquePerRequest(t *testing.T) {
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	assert.NotEqual(t, rr1.Header().Get("X-Request-ID"), rr2.Header().Get("X-Request-ID"))
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "ok", rr.Body.String())
}

func TestRecoveryMiddleware_WithPanic(t *testing.T) {
	handler := RecoveryMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	// Should not panic
	require.NotPanics(t, func() {
		handler.ServeHTTP(rr, req)
	})

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestLoggingMiddleware(t *testing.T) {
	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/projects", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should pass through the status code
	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestLoggingMiddleware_DefaultStatusCode(t *testing.T) {
	handler := LoggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No explicit WriteHeader call → default 200
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestResponseRecorder_WriteHeader(t *testing.T) {
	inner := httptest.NewRecorder()
	rr := &responseRecorder{ResponseWriter: inner, statusCode: http.StatusOK}

	rr.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, rr.statusCode)
	assert.Equal(t, http.StatusNotFound, inner.Code)
}

func TestMiddlewareChain(t *testing.T) {
	var capturedID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	// Chain: RequestID → Logging → Recovery → handler
	handler := RequestIDMiddleware(LoggingMiddleware(RecoveryMiddleware(inner)))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.NotEmpty(t, capturedID)
	assert.NotEmpty(t, rr.Header().Get("X-Request-ID"))
}
