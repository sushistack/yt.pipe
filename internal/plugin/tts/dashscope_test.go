package tts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewDashScopeProvider_NoAPIKey(t *testing.T) {
	_, err := NewDashScopeProvider(DashScopeConfig{})
	if err == nil {
		t.Fatal("expected error for empty API key")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("expected status 401, got %d", apiErr.StatusCode)
	}
}

func TestNewDashScopeProvider_Defaults(t *testing.T) {
	p, err := NewDashScopeProvider(DashScopeConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.endpoint != defaultDashScopeEndpoint {
		t.Errorf("expected endpoint %q, got %q", defaultDashScopeEndpoint, p.endpoint)
	}
	if p.model != defaultDashScopeModel {
		t.Errorf("expected model %q, got %q", defaultDashScopeModel, p.model)
	}
	if p.format != defaultDashScopeFormat {
		t.Errorf("expected format %q, got %q", defaultDashScopeFormat, p.format)
	}
	if p.voice != defaultDashScopeVoice {
		t.Errorf("expected voice %q, got %q", defaultDashScopeVoice, p.voice)
	}
}

func TestSynthesize_Success(t *testing.T) {
	audioBytes := []byte("fake-audio-data")
	audioB64 := base64.StdEncoding.EncodeToString(audioBytes)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/services/aigc/text2audio/generation" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var req dsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "cosyvoice-v1" {
			t.Errorf("expected model cosyvoice-v1, got %s", req.Model)
		}

		resp := dsResponse{
			RequestID: "test-req-1",
			Output: dsOutput{
				Audio:      audioB64,
				DurationMs: 2500,
				WordTimings: []dsWordTiming{
					{Word: "안녕", StartMs: 0, EndMs: 500},
					{Word: "하세요", StartMs: 500, EndMs: 1200},
				},
			},
			Usage: dsUsage{Characters: 5},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, err := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	result, err := p.Synthesize(context.Background(), "안녕하세요", "longxiaochun")
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}

	if string(result.AudioData) != string(audioBytes) {
		t.Errorf("audio data mismatch")
	}
	if result.DurationSec != 2.5 {
		t.Errorf("expected duration 2.5, got %f", result.DurationSec)
	}
	if len(result.WordTimings) != 2 {
		t.Fatalf("expected 2 word timings, got %d", len(result.WordTimings))
	}
	if result.WordTimings[0].Word != "안녕" {
		t.Errorf("expected first word '안녕', got %q", result.WordTimings[0].Word)
	}
	if result.WordTimings[0].StartSec != 0.0 || result.WordTimings[0].EndSec != 0.5 {
		t.Errorf("unexpected timing: start=%f end=%f", result.WordTimings[0].StartSec, result.WordTimings[0].EndSec)
	}
}

func TestSynthesize_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(dsErrorResponse{
			Code:    "Throttling",
			Message: "rate limited",
		})
	}))
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	_, err := p.Synthesize(context.Background(), "test", "voice")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSynthesizeWithOverrides(t *testing.T) {
	var receivedText string
	audioB64 := base64.StdEncoding.EncodeToString([]byte("audio"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dsRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedText = req.Input.Text

		resp := dsResponse{
			Output: dsOutput{Audio: audioB64, DurationMs: 1000},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	overrides := map[string]string{
		"SCP": "에스씨피",
	}
	_, err := p.SynthesizeWithOverrides(context.Background(), "SCP-173 문서", "voice", overrides)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedText != "에스씨피-173 문서" {
		t.Errorf("expected '에스씨피-173 문서', got %q", receivedText)
	}
}

func TestIsCloneVoice(t *testing.T) {
	tests := []struct {
		voice    string
		expected bool
	}{
		{"longxiaochun", false},
		{"cosyvoice-clone-abc123", true},
		{"cosyvoice-clone-", true},
		{"", false},
	}
	for _, tt := range tests {
		if got := isCloneVoice(tt.voice); got != tt.expected {
			t.Errorf("isCloneVoice(%q) = %v, want %v", tt.voice, got, tt.expected)
		}
	}
}

func TestVoiceCloneFlag(t *testing.T) {
	var receivedCloneFlag bool
	audioB64 := base64.StdEncoding.EncodeToString([]byte("audio"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req dsRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedCloneFlag = req.Parameters.VoiceClone

		resp := dsResponse{
			Output: dsOutput{Audio: audioB64, DurationMs: 1000},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	// Test with clone voice
	p.Synthesize(context.Background(), "text", "cosyvoice-clone-myvoice")
	if !receivedCloneFlag {
		t.Error("expected voice_clone=true for clone voice")
	}

	// Test with standard voice
	p.Synthesize(context.Background(), "text", "longxiaochun")
	if receivedCloneFlag {
		t.Error("expected voice_clone=false for standard voice")
	}
}

func TestApplyOverrides(t *testing.T) {
	text := "SCP-173은 Euclid 등급입니다"
	overrides := map[string]string{
		"SCP":    "에스씨피",
		"Euclid": "유클리드",
	}
	result := applyOverrides(text, overrides)
	expected := "에스씨피-173은 유클리드 등급입니다"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestApplyOverrides_Nil(t *testing.T) {
	text := "no changes"
	result := applyOverrides(text, nil)
	if result != text {
		t.Errorf("expected %q, got %q", text, result)
	}
}

func TestDashScopeFactory(t *testing.T) {
	cfg := map[string]interface{}{
		"api_key": "test-key",
		"model":   "cosyvoice-v1-flash",
	}
	raw, err := DashScopeFactory(cfg)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	p, ok := raw.(*DashScopeProvider)
	if !ok {
		t.Fatalf("expected *DashScopeProvider, got %T", raw)
	}
	if p.model != "cosyvoice-v1-flash" {
		t.Errorf("expected model cosyvoice-v1-flash, got %s", p.model)
	}
}

func TestAPIError_IsRetryable(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{429, true},
		{500, true},
		{502, true},
		{503, true},
		{0, true},
		{400, false},
		{401, false},
		{404, false},
	}
	for _, tt := range tests {
		err := &APIError{Provider: "test", StatusCode: tt.code, Message: "test"}
		if got := err.IsRetryable(); got != tt.expected {
			t.Errorf("APIError{StatusCode: %d}.IsRetryable() = %v, want %v", tt.code, got, tt.expected)
		}
	}
}
