package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/sushistack/yt.pipe/internal/config"
)

// WebhookEvent represents a webhook notification payload.
type WebhookEvent struct {
	Event         string `json:"event"`
	ProjectID     string `json:"project_id"`
	SCPID         string `json:"scp_id"`
	PreviousState string `json:"previous_state,omitempty"`
	NewState      string `json:"new_state,omitempty"`
	ReviewURL     string `json:"review_url,omitempty"`
	Timestamp     string `json:"timestamp"`
}

// JobCompleteEvent represents a job_complete webhook payload (flat JSON for n8n).
type JobCompleteEvent struct {
	Event     string `json:"event"`
	ProjectID string `json:"project_id"`
	SCPID     string `json:"scp_id"`
	JobID     string `json:"job_id"`
	JobType   string `json:"job_type"`
	Result    string `json:"result"`
	NewState  string `json:"new_state,omitempty"`
	ReviewURL string `json:"review_url,omitempty"`
	Timestamp string `json:"timestamp"`
}

// JobFailedEvent represents a job_failed webhook payload (flat JSON for n8n).
type JobFailedEvent struct {
	Event       string `json:"event"`
	ProjectID   string `json:"project_id"`
	SCPID       string `json:"scp_id"`
	JobID       string `json:"job_id"`
	JobType     string `json:"job_type"`
	Error       string `json:"error"`
	FailedScene int    `json:"failed_scene"`
	NewState    string `json:"new_state,omitempty"`
	ReviewURL   string `json:"review_url,omitempty"`
	Timestamp   string `json:"timestamp"`
}

// SceneApprovedEvent represents a scene_approved webhook payload (flat JSON for n8n).
type SceneApprovedEvent struct {
	Event     string `json:"event"`
	ProjectID string `json:"project_id"`
	SCPID     string `json:"scp_id"`
	SceneNum  int    `json:"scene_num"`
	AssetType string `json:"asset_type"`
	ReviewURL string `json:"review_url,omitempty"`
	Timestamp string `json:"timestamp"`
}

// AllApprovedEvent represents an all_approved webhook payload (flat JSON for n8n).
type AllApprovedEvent struct {
	Event     string `json:"event"`
	ProjectID string `json:"project_id"`
	SCPID     string `json:"scp_id"`
	AssetType string `json:"asset_type"`
	ReviewURL string `json:"review_url,omitempty"`
	Timestamp string `json:"timestamp"`
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
func (wn *WebhookNotifier) NotifyStateChange(projectID, scpID, previousState, newState, reviewURL string) {
	if wn == nil {
		return
	}

	event := WebhookEvent{
		Event:         "state_change",
		ProjectID:     projectID,
		SCPID:         scpID,
		PreviousState: previousState,
		NewState:      newState,
		ReviewURL:     reviewURL,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
	wn.fanOut(event)
}

// NotifyJobComplete sends a job_complete event to all configured webhook URLs.
func (wn *WebhookNotifier) NotifyJobComplete(projectID, scpID, jobID, jobType, result, newState, reviewURL string) {
	if wn == nil {
		return
	}
	event := JobCompleteEvent{
		Event:     "job_complete",
		ProjectID: projectID,
		SCPID:     scpID,
		JobID:     jobID,
		JobType:   jobType,
		Result:    result,
		NewState:  newState,
		ReviewURL: reviewURL,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	wn.fanOut(event)
}

// NotifyJobFailed sends a job_failed event to all configured webhook URLs.
func (wn *WebhookNotifier) NotifyJobFailed(projectID, scpID, jobID, jobType, errMsg string, failedScene int, newState, reviewURL string) {
	if wn == nil {
		return
	}
	event := JobFailedEvent{
		Event:       "job_failed",
		ProjectID:   projectID,
		SCPID:       scpID,
		JobID:       jobID,
		JobType:     jobType,
		Error:       errMsg,
		FailedScene: failedScene,
		NewState:    newState,
		ReviewURL:   reviewURL,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	wn.fanOut(event)
}

// NotifySceneApproved sends a scene_approved event to all configured webhook URLs.
func (wn *WebhookNotifier) NotifySceneApproved(projectID, scpID string, sceneNum int, assetType, reviewURL string) {
	if wn == nil {
		return
	}
	event := SceneApprovedEvent{
		Event:     "scene_approved",
		ProjectID: projectID,
		SCPID:     scpID,
		SceneNum:  sceneNum,
		AssetType: assetType,
		ReviewURL: reviewURL,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	wn.fanOut(event)
}

// NotifyAllApproved sends an all_approved event to all configured webhook URLs.
func (wn *WebhookNotifier) NotifyAllApproved(projectID, scpID, assetType, reviewURL string) {
	if wn == nil {
		return
	}
	event := AllApprovedEvent{
		Event:     "all_approved",
		ProjectID: projectID,
		SCPID:     scpID,
		AssetType: assetType,
		ReviewURL: reviewURL,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	wn.fanOut(event)
}

// BuildReviewURL constructs the review URL for a project.
func BuildReviewURL(projectID, reviewToken string) string {
	if reviewToken == "" {
		return ""
	}
	return fmt.Sprintf("/review/%s?token=%s", projectID, reviewToken)
}

// fanOut marshals the event and sends to all configured URLs independently.
func (wn *WebhookNotifier) fanOut(event interface{}) {
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("webhook: marshal event", "error", err)
		return
	}

	var wg sync.WaitGroup
	for _, url := range wn.urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			wn.sendWithRetry(u, payload)
		}(url)
	}
	// Fire and forget — don't wait
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
