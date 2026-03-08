package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jay/youtube-pipeline/internal/config"
)

// WebhookEvent represents a webhook notification payload.
type WebhookEvent struct {
	Event         string `json:"event"`
	ProjectID     string `json:"project_id"`
	SCPID         string `json:"scp_id"`
	PreviousState string `json:"previous_state"`
	NewState      string `json:"new_state"`
	Timestamp     string `json:"timestamp"`
}

// WebhookNotifier sends webhook notifications on state changes.
type WebhookNotifier struct {
	urls       []string
	timeout    time.Duration
	maxRetries int
	client     *http.Client
}

// NewWebhookNotifier creates a webhook notifier from config.
// Returns nil if no URLs are configured (no-op).
func NewWebhookNotifier(cfg config.WebhookConfig) *WebhookNotifier {
	if len(cfg.URLs) == 0 {
		return nil
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	maxRetries := cfg.RetryMaxAttempts
	if maxRetries <= 0 {
		maxRetries = 3
	}

	return &WebhookNotifier{
		urls:       cfg.URLs,
		timeout:    timeout,
		maxRetries: maxRetries,
		client:     &http.Client{Timeout: timeout},
	}
}

// NotifyStateChange sends a state change event to all configured webhook URLs.
// Each URL is notified independently (fan-out). Failures are logged but don't
// block the caller.
func (wn *WebhookNotifier) NotifyStateChange(projectID, scpID, previousState, newState string) {
	if wn == nil {
		return
	}

	event := WebhookEvent{
		Event:         "state_change",
		ProjectID:     projectID,
		SCPID:         scpID,
		PreviousState: previousState,
		NewState:      newState,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("webhook: marshal event", "error", err)
		return
	}

	// Fan-out: send to each URL independently
	var wg sync.WaitGroup
	for _, url := range wn.urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			wn.sendWithRetry(u, payload)
		}(url)
	}
	// Don't wait — fire and forget so webhook failures never block the pipeline
}

func (wn *WebhookNotifier) sendWithRetry(url string, payload []byte) {
	backoff := time.Second // 1s, 2s, 4s exponential

	for attempt := 1; attempt <= wn.maxRetries; attempt++ {
		resp, err := wn.client.Post(url, "application/json", bytes.NewReader(payload))
		if err != nil {
			slog.Warn("webhook delivery failed",
				"url", url,
				"attempt", attempt,
				"error", err,
			)
			if attempt < wn.maxRetries {
				time.Sleep(backoff)
				backoff *= 2
			}
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			slog.Debug("webhook delivered", "url", url, "status", resp.StatusCode)
			return
		}

		slog.Warn("webhook delivery non-2xx",
			"url", url,
			"attempt", attempt,
			"status", resp.StatusCode,
		)
		if attempt < wn.maxRetries {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	slog.Error("webhook delivery exhausted retries", "url", url, "max_retries", wn.maxRetries)
}
