package tts

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// makeWAV creates a minimal valid WAV file with the given PCM sample count.
// 24kHz, 16-bit, mono.
func makeWAV(numSamples int) []byte {
	const (
		sampleRate    = 24000
		bitsPerSample = 16
		numChannels   = 1
		blockAlign    = numChannels * bitsPerSample / 8
		byteRate      = sampleRate * blockAlign
	)
	dataSize := numSamples * blockAlign
	fileSize := 36 + dataSize // RIFF chunk size = file size - 8, but here it's 36 + dataSize

	buf := make([]byte, 44+dataSize)
	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:20], 16) // fmt chunk size
	binary.LittleEndian.PutUint16(buf[20:22], 1)  // PCM
	binary.LittleEndian.PutUint16(buf[22:24], numChannels)
	binary.LittleEndian.PutUint32(buf[24:28], sampleRate)
	binary.LittleEndian.PutUint32(buf[28:32], byteRate)
	binary.LittleEndian.PutUint16(buf[32:34], blockAlign)
	binary.LittleEndian.PutUint16(buf[34:36], bitsPerSample)
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:44], uint32(dataSize))
	// PCM data is zero-filled (silence)
	return buf
}

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

// newTestServer creates a test HTTP server that handles both the TTS API call
// and the audio download. Returns the server and the WAV audio bytes it serves.
func newTestServer(t *testing.T, wavData []byte, handler func(w http.ResponseWriter, r *http.Request, audioURL string)) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case qwenTTSAPIPath:
			audioURL := server.URL + "/audio/download"
			if handler != nil {
				handler(w, r, audioURL)
				return
			}
			// Return response matching real API format (expires_at is a number)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"request_id":"test-req-1","output":{"audio":{"url":"%s","expires_at":1773490349},"finish_reason":"stop"}}`, audioURL)
		case "/audio/download":
			w.Header().Set("Content-Type", "audio/wav")
			w.Write(wavData)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server
}

func TestSynthesize_Success(t *testing.T) {
	// 2.5 seconds at 24kHz mono 16-bit = 60000 samples
	wavData := makeWAV(60000)

	server := newTestServer(t, wavData, func(w http.ResponseWriter, r *http.Request, audioURL string) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var req qwenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != defaultDashScopeModel {
			t.Errorf("expected model %s, got %s", defaultDashScopeModel, req.Model)
		}
		if req.Input.Voice != "longxiaochun" {
			t.Errorf("expected voice longxiaochun, got %s", req.Input.Voice)
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"request_id":"test-req-1","output":{"audio":{"url":"%s","expires_at":1773490349},"finish_reason":"stop"}}`, audioURL)
	})
	defer server.Close()

	p, err := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	result, err := p.Synthesize(context.Background(), "안녕하세요", "longxiaochun", nil)
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}

	if len(result.AudioData) != len(wavData) {
		t.Errorf("audio data length mismatch: got %d, want %d", len(result.AudioData), len(wavData))
	}
	// 60000 samples / 24000 Hz = 2.5 sec
	if result.DurationSec < 2.4 || result.DurationSec > 2.6 {
		t.Errorf("expected duration ~2.5, got %f", result.DurationSec)
	}
	// Qwen3 TTS does not return word timings
	if len(result.WordTimings) != 0 {
		t.Errorf("expected 0 word timings, got %d", len(result.WordTimings))
	}
}

func TestSynthesize_WithMoodPreset(t *testing.T) {
	wavData := makeWAV(24000) // 1 second

	server := newTestServer(t, wavData, nil)
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	opts := &TTSOptions{
		MoodPreset: &MoodPreset{
			Speed:   1.2,
			Emotion: "fearful",
			Pitch:   0.9,
			Params:  map[string]any{"intensity": 0.8},
		},
	}

	result, err := p.Synthesize(context.Background(), "공포의 순간", "longxiaochun", opts)
	if err != nil {
		t.Fatalf("synthesize with mood: %v", err)
	}
	if result.DurationSec < 0.9 || result.DurationSec > 1.1 {
		t.Errorf("expected duration ~1.0, got %f", result.DurationSec)
	}
}

func TestSynthesize_NilOpts(t *testing.T) {
	wavData := makeWAV(24000)

	server := newTestServer(t, wavData, nil)
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	result, err := p.Synthesize(context.Background(), "test", "longxiaochun", nil)
	if err != nil {
		t.Fatalf("synthesize with nil opts: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestSynthesize_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(qwenErrorResponse{
			Code:    "Throttling",
			Message: "rate limited",
		})
	}))
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	_, err := p.Synthesize(context.Background(), "test", "voice", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSynthesizeWithOverrides(t *testing.T) {
	var receivedText string
	wavData := makeWAV(24000)

	server := newTestServer(t, wavData, func(w http.ResponseWriter, r *http.Request, audioURL string) {
		var req qwenRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedText = req.Input.Text

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"output":{"audio":{"url":"%s","expires_at":1773490349}}}`, audioURL)
	})
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	overrides := map[string]string{
		"SCP": "에스씨피",
	}
	_, err := p.SynthesizeWithOverrides(context.Background(), "SCP-173 문서", "voice", overrides, nil)
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
		"model":   "qwen3-tts-flash",
	}
	raw, err := DashScopeFactory(cfg)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	p, ok := raw.(*DashScopeProvider)
	if !ok {
		t.Fatalf("expected *DashScopeProvider, got %T", raw)
	}
	if p.model != "qwen3-tts-flash" {
		t.Errorf("expected model qwen3-tts-flash, got %s", p.model)
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

func TestWavDuration(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected float64
	}{
		{"2.5 seconds", makeWAV(60000), 2.5},
		{"1 second", makeWAV(24000), 1.0},
		{"empty", []byte{}, 0},
		{"too short", []byte("hello"), 0},
		{"not wav", make([]byte, 100), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wavDuration(tt.data)
			if got < tt.expected-0.01 || got > tt.expected+0.01 {
				t.Errorf("wavDuration() = %f, want %f", got, tt.expected)
			}
		})
	}
}

func TestSynthesize_CloneVoiceModelSwitch(t *testing.T) {
	wavData := makeWAV(24000)
	var receivedModel string

	server := newTestServer(t, wavData, func(w http.ResponseWriter, r *http.Request, audioURL string) {
		var req qwenRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedModel = req.Model

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"request_id":"test","output":{"audio":{"url":"%s","expires_at":1773490349}}}`, audioURL)
	})
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint:   server.URL,
		APIKey:     "test-key",
		CloneModel: "qwen3-tts-vc-2026-01-22",
	})

	// Clone voice should use clone model
	_, err := p.Synthesize(context.Background(), "test", "cosyvoice-clone-abc123", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedModel != "qwen3-tts-vc-2026-01-22" {
		t.Errorf("expected clone model, got %s", receivedModel)
	}

	// Regular voice should use default model
	_, err = p.Synthesize(context.Background(), "test", "Cherry", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedModel != defaultDashScopeModel {
		t.Errorf("expected default model %s, got %s", defaultDashScopeModel, receivedModel)
	}
}

func TestCreateVoice_Success(t *testing.T) {
	// Create a temp audio file
	tmpDir := t.TempDir()
	audioPath := tmpDir + "/voice.mp3"
	if err := os.WriteFile(audioPath, []byte("fake-audio-data"), 0o644); err != nil {
		t.Fatalf("write temp audio: %v", err)
	}

	var receivedBody voiceEnrollmentRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != qwenVoiceEnrollmentPath {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"request_id":"enroll-1","output":{"voice":"cosyvoice-clone-test123"}}`)
	}))
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	voiceID, err := p.CreateVoice(context.Background(), audioPath, "narrator")
	if err != nil {
		t.Fatalf("create voice: %v", err)
	}
	if voiceID != "cosyvoice-clone-test123" {
		t.Errorf("expected voice ID cosyvoice-clone-test123, got %s", voiceID)
	}
	if receivedBody.Model != qwenVoiceEnrollmentModel {
		t.Errorf("expected model %s, got %s", qwenVoiceEnrollmentModel, receivedBody.Model)
	}
	if receivedBody.Input.Action != "create" {
		t.Errorf("expected action create, got %s", receivedBody.Input.Action)
	}
	if receivedBody.Input.PreferredName != "narrator" {
		t.Errorf("expected preferred_name narrator, got %s", receivedBody.Input.PreferredName)
	}
	// Verify audio is base64 data URI
	if !strings.HasPrefix(receivedBody.Input.Audio.Data, "data:audio/mpeg;base64,") {
		t.Errorf("expected data URI with audio/mpeg, got prefix: %s", receivedBody.Input.Audio.Data[:40])
	}
}

func TestCreateVoice_APIError(t *testing.T) {
	tmpDir := t.TempDir()
	audioPath := tmpDir + "/voice.mp3"
	os.WriteFile(audioPath, []byte("fake"), 0o644)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"code":"InvalidInput","message":"bad audio"}`)
	}))
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	_, err := p.CreateVoice(context.Background(), audioPath, "test")
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", apiErr.StatusCode)
	}
}

func TestSynthesize_EmptyAudioURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := qwenResponse{
			Output: qwenOutput{Audio: nil},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p, _ := NewDashScopeProvider(DashScopeConfig{
		Endpoint: server.URL,
		APIKey:   "test-key",
	})

	_, err := p.Synthesize(context.Background(), "test", "voice", nil)
	if err == nil {
		t.Fatal("expected error for nil audio")
	}
}
