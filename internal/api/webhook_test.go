package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jay/youtube-pipeline/internal/api"
	"github.com/jay/youtube-pipeline/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookNotifier_NilSafe(t *testing.T) {
	// Nil notifier should be a no-op
	var wn *api.WebhookNotifier
	wn.NotifyStateChange("proj-1", "SCP-173", "pending", "approved")
	// No panic = pass
}

func TestWebhookNotifier_NoURLs(t *testing.T) {
	wn := api.NewWebhookNotifier(config.WebhookConfig{})
	assert.Nil(t, wn)
}

func TestWebhookNotifier_SendsPayload(t *testing.T) {
	var received atomic.Int32
	var lastEvent api.WebhookEvent

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		json.NewDecoder(r.Body).Decode(&lastEvent)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	wn := api.NewWebhookNotifier(config.WebhookConfig{
		URLs:            []string{ts.URL},
		TimeoutSeconds:  5,
		RetryMaxAttempts: 1,
	})
	require.NotNil(t, wn)

	wn.NotifyStateChange("proj-1", "SCP-173", "pending", "scenario_review")

	// Wait briefly for async delivery
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(1), received.Load())
	assert.Equal(t, "state_change", lastEvent.Event)
	assert.Equal(t, "proj-1", lastEvent.ProjectID)
	assert.Equal(t, "SCP-173", lastEvent.SCPID)
	assert.Equal(t, "pending", lastEvent.PreviousState)
	assert.Equal(t, "scenario_review", lastEvent.NewState)
}

func TestWebhookNotifier_FanOut(t *testing.T) {
	var count1, count2 atomic.Int32

	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count1.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts1.Close()

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count2.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts2.Close()

	wn := api.NewWebhookNotifier(config.WebhookConfig{
		URLs:            []string{ts1.URL, ts2.URL},
		TimeoutSeconds:  5,
		RetryMaxAttempts: 1,
	})

	wn.NotifyStateChange("proj-1", "SCP-173", "pending", "approved")
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, int32(1), count1.Load())
	assert.Equal(t, int32(1), count2.Load())
}

func TestWebhookNotifier_RetryOnFailure(t *testing.T) {
	var attempts atomic.Int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := attempts.Add(1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	wn := api.NewWebhookNotifier(config.WebhookConfig{
		URLs:            []string{ts.URL},
		TimeoutSeconds:  5,
		RetryMaxAttempts: 3,
	})

	wn.NotifyStateChange("proj-1", "SCP-173", "pending", "approved")
	time.Sleep(10 * time.Second) // Allow retries with backoff

	assert.Equal(t, int32(3), attempts.Load())
}

func TestWebhookNotifier_FailureDoesNotBlock(t *testing.T) {
	// Webhook to unreachable host should not block
	wn := api.NewWebhookNotifier(config.WebhookConfig{
		URLs:            []string{"http://192.0.2.1:9999/unreachable"}, // TEST-NET, should timeout
		TimeoutSeconds:  1,
		RetryMaxAttempts: 1,
	})

	start := time.Now()
	wn.NotifyStateChange("proj-1", "SCP-173", "pending", "approved")
	elapsed := time.Since(start)

	// NotifyStateChange should return nearly instantly (fire-and-forget)
	assert.Less(t, elapsed, 100*time.Millisecond)
}
