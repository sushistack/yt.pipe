package imagegen

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSiliconFlowProvider_Success(t *testing.T) {
	p, err := NewSiliconFlowProvider(SiliconFlowConfig{
		APIKey: "test-key",
	})
	require.NoError(t, err)
	assert.Equal(t, defaultSiliconFlowModel, p.model)
	assert.Equal(t, defaultSiliconFlowEndpoint, p.endpoint)
}

func TestNewSiliconFlowProvider_CustomModel(t *testing.T) {
	p, err := NewSiliconFlowProvider(SiliconFlowConfig{
		APIKey: "test-key",
		Model:  "stabilityai/stable-diffusion-3-5-large",
	})
	require.NoError(t, err)
	assert.Equal(t, "stabilityai/stable-diffusion-3-5-large", p.model)
}

func TestNewSiliconFlowProvider_NoAPIKey(t *testing.T) {
	_, err := NewSiliconFlowProvider(SiliconFlowConfig{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication")
}

func TestGenerate_Success_Base64(t *testing.T) {
	fakeImage := []byte("fake-png-image-data-for-testing")
	b64Image := base64.StdEncoding.EncodeToString(fakeImage)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/images/generations", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req sfImageRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, defaultSiliconFlowModel, req.Model)
		assert.Equal(t, "a dark figure", req.Prompt)
		assert.Equal(t, "1920x1080", req.ImageSize)
		assert.Equal(t, 1, req.BatchSize)

		resp := sfImageResponse{
			Images: []sfImage{{URL: b64Image}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewSiliconFlowProvider(SiliconFlowConfig{
		Endpoint: srv.URL + "/v1",
		APIKey:   "test-key",
	})
	require.NoError(t, err)

	result, err := p.Generate(context.Background(), "a dark figure", GenerateOptions{})
	require.NoError(t, err)
	assert.Equal(t, fakeImage, result.ImageData)
	assert.Equal(t, "png", result.Format)
	assert.Equal(t, 1920, result.Width)
	assert.Equal(t, 1080, result.Height)
}

func TestGenerate_Success_URL(t *testing.T) {
	fakeImage := []byte("url-based-image-data")

	// Image download server
	imgSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fakeImage)
	}))
	defer imgSrv.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := sfImageResponse{
			Images: []sfImage{{URL: imgSrv.URL + "/image.png"}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, err := NewSiliconFlowProvider(SiliconFlowConfig{
		Endpoint: srv.URL + "/v1",
		APIKey:   "test-key",
	})
	require.NoError(t, err)

	result, err := p.Generate(context.Background(), "test prompt", GenerateOptions{
		Width:  512,
		Height: 512,
	})
	require.NoError(t, err)
	assert.Equal(t, fakeImage, result.ImageData)
	assert.Equal(t, 512, result.Width)
	assert.Equal(t, 512, result.Height)
}

func TestGenerate_CustomDimensions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sfImageRequest
		json.NewDecoder(r.Body).Decode(&req)
		assert.Equal(t, "768x768", req.ImageSize)

		b64 := base64.StdEncoding.EncodeToString([]byte("img"))
		resp := sfImageResponse{Images: []sfImage{{URL: b64}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, _ := NewSiliconFlowProvider(SiliconFlowConfig{
		Endpoint: srv.URL + "/v1",
		APIKey:   "test-key",
	})

	result, err := p.Generate(context.Background(), "prompt", GenerateOptions{
		Width:  768,
		Height: 768,
	})
	require.NoError(t, err)
	assert.Equal(t, 768, result.Width)
}

func TestGenerate_WithSeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req sfImageRequest
		json.NewDecoder(r.Body).Decode(&req)
		require.NotNil(t, req.Seed)
		assert.Equal(t, int64(42), *req.Seed)

		b64 := base64.StdEncoding.EncodeToString([]byte("img"))
		resp := sfImageResponse{Images: []sfImage{{URL: b64}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, _ := NewSiliconFlowProvider(SiliconFlowConfig{
		Endpoint: srv.URL + "/v1",
		APIKey:   "test-key",
	})

	_, err := p.Generate(context.Background(), "prompt", GenerateOptions{Seed: 42})
	require.NoError(t, err)
}

func TestGenerate_RateLimit(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(sfErrorResponse{})
	}))
	defer srv.Close()

	p, _ := NewSiliconFlowProvider(SiliconFlowConfig{
		Endpoint: srv.URL + "/v1",
		APIKey:   "test-key",
	})

	_, err := p.Generate(context.Background(), "prompt", GenerateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
	// Should retry (default 3 attempts)
	assert.Equal(t, 3, attempts)
}

func TestGenerate_ServerError_Retries(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":{"message":"internal error"}}`))
			return
		}
		b64 := base64.StdEncoding.EncodeToString([]byte("img"))
		resp := sfImageResponse{Images: []sfImage{{URL: b64}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, _ := NewSiliconFlowProvider(SiliconFlowConfig{
		Endpoint: srv.URL + "/v1",
		APIKey:   "test-key",
	})

	result, err := p.Generate(context.Background(), "prompt", GenerateOptions{})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, attempts)
}

func TestGenerate_ClientError_NoRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"message":"bad prompt"}}`))
	}))
	defer srv.Close()

	p, _ := NewSiliconFlowProvider(SiliconFlowConfig{
		Endpoint: srv.URL + "/v1",
		APIKey:   "test-key",
	})

	_, err := p.Generate(context.Background(), "prompt", GenerateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad prompt")
	assert.Equal(t, 1, attempts) // No retry for 400
}

func TestGenerate_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(sfImageResponse{Images: []sfImage{}})
	}))
	defer srv.Close()

	p, _ := NewSiliconFlowProvider(SiliconFlowConfig{
		Endpoint: srv.URL + "/v1",
		APIKey:   "test-key",
	})

	_, err := p.Generate(context.Background(), "prompt", GenerateOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no images returned")
}

func TestGenerate_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response - won't reach here due to cancelled context
		b64 := base64.StdEncoding.EncodeToString([]byte("img"))
		resp := sfImageResponse{Images: []sfImage{{URL: b64}}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p, _ := NewSiliconFlowProvider(SiliconFlowConfig{
		Endpoint: srv.URL + "/v1",
		APIKey:   "test-key",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := p.Generate(ctx, "prompt", GenerateOptions{})
	require.Error(t, err)
}

func TestSiliconFlowFactory_Success(t *testing.T) {
	raw, err := SiliconFlowFactory(map[string]interface{}{
		"api_key": "test-key",
		"model":   "custom-model",
	})
	require.NoError(t, err)
	p, ok := raw.(*SiliconFlowProvider)
	require.True(t, ok)
	assert.Equal(t, "custom-model", p.model)
}

func TestSiliconFlowFactory_Defaults(t *testing.T) {
	raw, err := SiliconFlowFactory(map[string]interface{}{
		"api_key": "test-key",
	})
	require.NoError(t, err)
	p, ok := raw.(*SiliconFlowProvider)
	require.True(t, ok)
	assert.Equal(t, defaultSiliconFlowModel, p.model)
	assert.Equal(t, defaultSiliconFlowEndpoint, p.endpoint)
}

func TestSiliconFlowFactory_NoKey(t *testing.T) {
	_, err := SiliconFlowFactory(map[string]interface{}{})
	require.Error(t, err)
}

func TestAPIError_IsRetryable(t *testing.T) {
	tests := []struct {
		code      int
		retryable bool
	}{
		{0, true},     // network error
		{429, true},   // rate limit
		{500, true},   // server error
		{502, true},   // bad gateway
		{503, true},   // service unavailable
		{400, false},  // bad request
		{401, false},  // unauthorized
		{403, false},  // forbidden
		{404, false},  // not found
	}
	for _, tt := range tests {
		e := &APIError{Provider: "test", StatusCode: tt.code}
		assert.Equal(t, tt.retryable, e.IsRetryable(), "status %d", tt.code)
	}
}

func TestDecodeImageData_Base64(t *testing.T) {
	original := []byte("test image data")
	b64 := base64.StdEncoding.EncodeToString(original)

	data, err := decodeImageData(context.Background(), b64, http.DefaultClient)
	require.NoError(t, err)
	assert.Equal(t, original, data)
}

func TestDecodeImageData_DataURI(t *testing.T) {
	original := []byte("test image data")
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(original)

	data, err := decodeImageData(context.Background(), dataURI, http.DefaultClient)
	require.NoError(t, err)
	assert.Equal(t, original, data)
}
