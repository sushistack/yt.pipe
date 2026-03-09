package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sushistack/yt.pipe/internal/api"
	"github.com/sushistack/yt.pipe/internal/config"
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

func TestWebhookNotifier_NilSafe_AllMethods(t *testing.T) {
	// All Notify methods on nil notifier should be no-ops (no panic)
	var wn *api.WebhookNotifier
	wn.NotifyStateChange("proj-1", "SCP-173", "pending", "approved")
	wn.NotifyJobComplete("proj-1", "SCP-173", "job-1", "image_generate", "ok")
	wn.NotifyJobFailed("proj-1", "SCP-173", "job-1", "tts_generate", "err", 3)
	wn.NotifySceneApproved("proj-1", "SCP-173", 1, "image")
	wn.NotifyAllApproved("proj-1", "SCP-173", "image")
}

func TestWebhookNotifier_JobComplete(t *testing.T) {
	var lastBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	wn := api.NewWebhookNotifier(config.WebhookConfig{
		URLs:             []string{ts.URL},
		TimeoutSeconds:   5,
		RetryMaxAttempts: 1,
	})
	require.NotNil(t, wn)

	wn.NotifyJobComplete("proj-1", "SCP-173", "job-42", "image_generate", "/path/to/output")
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "job_complete", lastBody["event"])
	assert.Equal(t, "proj-1", lastBody["project_id"])
	assert.Equal(t, "SCP-173", lastBody["scp_id"])
	assert.Equal(t, "job-42", lastBody["job_id"])
	assert.Equal(t, "image_generate", lastBody["job_type"])
	assert.Equal(t, "/path/to/output", lastBody["result"])
	assert.NotEmpty(t, lastBody["timestamp"])
}

func TestWebhookNotifier_JobFailed(t *testing.T) {
	var lastBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	wn := api.NewWebhookNotifier(config.WebhookConfig{
		URLs:             []string{ts.URL},
		TimeoutSeconds:   5,
		RetryMaxAttempts: 1,
	})

	wn.NotifyJobFailed("proj-1", "SCP-173", "job-99", "tts_generate", "scene 3: timeout", 3)
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "job_failed", lastBody["event"])
	assert.Equal(t, "proj-1", lastBody["project_id"])
	assert.Equal(t, "SCP-173", lastBody["scp_id"])
	assert.Equal(t, "job-99", lastBody["job_id"])
	assert.Equal(t, "tts_generate", lastBody["job_type"])
	assert.Equal(t, "scene 3: timeout", lastBody["error"])
	assert.Equal(t, float64(3), lastBody["failed_scene"]) // JSON numbers are float64
	assert.NotEmpty(t, lastBody["timestamp"])
}

func TestWebhookNotifier_SceneApproved(t *testing.T) {
	var lastBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	wn := api.NewWebhookNotifier(config.WebhookConfig{
		URLs:             []string{ts.URL},
		TimeoutSeconds:   5,
		RetryMaxAttempts: 1,
	})

	wn.NotifySceneApproved("proj-1", "SCP-173", 5, "image")
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "scene_approved", lastBody["event"])
	assert.Equal(t, "proj-1", lastBody["project_id"])
	assert.Equal(t, "SCP-173", lastBody["scp_id"])
	assert.Equal(t, float64(5), lastBody["scene_num"])
	assert.Equal(t, "image", lastBody["asset_type"])
	assert.NotEmpty(t, lastBody["timestamp"])
}

func TestWebhookNotifier_AllApproved(t *testing.T) {
	var lastBody map[string]interface{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	wn := api.NewWebhookNotifier(config.WebhookConfig{
		URLs:             []string{ts.URL},
		TimeoutSeconds:   5,
		RetryMaxAttempts: 1,
	})

	wn.NotifyAllApproved("proj-1", "SCP-173", "tts")
	time.Sleep(100 * time.Millisecond)

	assert.Equal(t, "all_approved", lastBody["event"])
	assert.Equal(t, "proj-1", lastBody["project_id"])
	assert.Equal(t, "SCP-173", lastBody["scp_id"])
	assert.Equal(t, "tts", lastBody["asset_type"])
	assert.NotEmpty(t, lastBody["timestamp"])
}

func TestWebhookNotifier_FlatJSON(t *testing.T) {
	// Verify all event payloads are flat JSON (no nested objects) — n8n compatibility
	var lastBody map[string]json.RawMessage

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&lastBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	wn := api.NewWebhookNotifier(config.WebhookConfig{
		URLs:             []string{ts.URL},
		TimeoutSeconds:   5,
		RetryMaxAttempts: 1,
	})

	// Test each event type for flat structure
	events := []func(){
		func() { wn.NotifyJobComplete("p", "s", "j", "t", "r") },
		func() { wn.NotifyJobFailed("p", "s", "j", "t", "e", 1) },
		func() { wn.NotifySceneApproved("p", "s", 1, "image") },
		func() { wn.NotifyAllApproved("p", "s", "image") },
	}

	for i, fire := range events {
		lastBody = nil
		fire()
		time.Sleep(100 * time.Millisecond)
		require.NotNil(t, lastBody, "event %d: no payload received", i)

		// Each value must be a scalar (string, number, bool) — not object or array
		for key, raw := range lastBody {
			s := string(raw)
			assert.False(t, len(s) > 0 && (s[0] == '{' || s[0] == '['),
				"event %d: field %q is nested: %s", i, key, s)
		}
	}
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
