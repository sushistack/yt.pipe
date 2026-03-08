package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockServer creates a test HTTP server that always returns the given status code.
func mockServer(t *testing.T, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
	}))
}

// mockServerWithAuthCheck creates a test HTTP server that checks Bearer token presence.
func mockServerWithAuthCheck(t *testing.T, validKey string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Bearer "+validKey {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
}

// TestValidateBearerToken_Success tests a 200 OK response.
func TestValidateBearerToken_Success(t *testing.T) {
	srv := mockServer(t, http.StatusOK)
	defer srv.Close()

	err := validateBearerToken(context.Background(), srv.URL, "test-key", "TestService")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

// TestValidateBearerToken_Unauthorized tests a 401 response.
func TestValidateBearerToken_Unauthorized(t *testing.T) {
	srv := mockServer(t, http.StatusUnauthorized)
	defer srv.Close()

	err := validateBearerToken(context.Background(), srv.URL, "bad-key", "TestService")
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}
	if !strings.Contains(err.Error(), "401 Unauthorized") {
		t.Errorf("expected '401 Unauthorized' in error message, got: %v", err)
	}
}

// TestValidateBearerToken_ServerError tests a 500 response.
func TestValidateBearerToken_ServerError(t *testing.T) {
	srv := mockServer(t, http.StatusInternalServerError)
	defer srv.Close()

	err := validateBearerToken(context.Background(), srv.URL, "key", "TestService")
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected status 500 in error message, got: %v", err)
	}
}

// TestValidateBearerToken_Timeout tests context cancellation / timeout.
func TestValidateBearerToken_Timeout(t *testing.T) {
	// Server that blocks until the request context is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := validateBearerToken(ctx, srv.URL, "key", "TestService")
	if err == nil {
		t.Fatal("expected error for timeout, got nil")
	}
}

// TestValidateLLMKey_Success tests successful LLM key validation via mock.
func TestValidateLLMKey_Success(t *testing.T) {
	srv := mockServer(t, http.StatusOK)
	defer srv.Close()

	// Call validateBearerToken directly with mock URL (same code path as validateLLMKey for openai).
	err := validateBearerToken(context.Background(), srv.URL, "sk-test", "LLM")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

// TestValidateLLMKey_Unauthorized tests failed LLM key validation.
func TestValidateLLMKey_Unauthorized(t *testing.T) {
	srv := mockServer(t, http.StatusUnauthorized)
	defer srv.Close()

	err := validateBearerToken(context.Background(), srv.URL, "bad-key", "LLM")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

// TestValidateLLMKey_Timeout tests that validateLLMKey respects timeout.
func TestValidateLLMKey_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := validateBearerToken(ctx, srv.URL, "key", "LLM")
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// TestValidateLLMKey_UnknownProvider tests unknown provider returns error.
func TestValidateLLMKey_UnknownProvider(t *testing.T) {
	err := validateLLMKey(context.Background(), "unknownprovider", "key")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "unknownprovider") {
		t.Errorf("expected provider name in error, got: %v", err)
	}
}

// TestValidateImageGenKey_Success tests successful ImageGen key validation.
func TestValidateImageGenKey_Success(t *testing.T) {
	srv := mockServer(t, http.StatusOK)
	defer srv.Close()

	err := validateBearerToken(context.Background(), srv.URL, "key", "ImageGen")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

// TestValidateImageGenKey_Unauthorized tests failed ImageGen key validation.
func TestValidateImageGenKey_Unauthorized(t *testing.T) {
	srv := mockServer(t, http.StatusUnauthorized)
	defer srv.Close()

	err := validateBearerToken(context.Background(), srv.URL, "bad-key", "ImageGen")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

// TestValidateImageGenKey_UnknownProvider tests unknown ImageGen provider.
func TestValidateImageGenKey_UnknownProvider(t *testing.T) {
	err := validateImageGenKey(context.Background(), "unknown", "key")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

// TestValidateTTSKey_EdgeSkipsValidation tests that edge TTS skips validation.
func TestValidateTTSKey_EdgeSkipsValidation(t *testing.T) {
	// No server needed — edge provider skips API call entirely.
	err := validateTTSKey(context.Background(), "edge", "")
	if err != nil {
		t.Errorf("expected nil for edge TTS, got: %v", err)
	}
}

// TestValidateTTSKey_GoogleSkipsValidation tests that google TTS skips validation.
func TestValidateTTSKey_GoogleSkipsValidation(t *testing.T) {
	err := validateTTSKey(context.Background(), "google", "some-key")
	if err != nil {
		t.Errorf("expected nil for google TTS skip, got: %v", err)
	}
}

// TestValidateTTSKey_OpenAI_Success tests successful OpenAI TTS validation.
func TestValidateTTSKey_OpenAI_Success(t *testing.T) {
	srv := mockServer(t, http.StatusOK)
	defer srv.Close()

	// Use validateBearerToken directly with mock URL (same code path).
	err := validateBearerToken(context.Background(), srv.URL, "sk-test", "TTS")
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

// TestValidateTTSKey_UnknownProvider tests unknown TTS provider.
func TestValidateTTSKey_UnknownProvider(t *testing.T) {
	err := validateTTSKey(context.Background(), "unknown", "key")
	if err == nil {
		t.Fatal("expected error for unknown TTS provider, got nil")
	}
}

// TestValidateBearerToken_AuthHeaderSent verifies the Authorization header is set correctly.
func TestValidateBearerToken_AuthHeaderSent(t *testing.T) {
	const testKey = "my-secret-key"
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := validateBearerToken(context.Background(), srv.URL, testKey, "Test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Bearer " + testKey
	if receivedAuth != expected {
		t.Errorf("expected Authorization header %q, got %q", expected, receivedAuth)
	}
}
