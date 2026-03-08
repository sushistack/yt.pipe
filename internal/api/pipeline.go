package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/service"
)

// JobStatus constants
const (
	JobStatusRunning   = "running"
	JobStatusComplete  = "complete"
	JobStatusFailed    = "failed"
	JobStatusCancelled = "cancelled"
)

// runningJob tracks a pipeline execution in progress.
type runningJob struct {
	JobID     string
	ProjectID string
	Status    string
	Progress  service.PipelineProgress
	StartedAt time.Time
	Cancel    context.CancelFunc
	mu        sync.RWMutex
}

func (j *runningJob) updateProgress(p service.PipelineProgress) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Progress = p
}

func (j *runningJob) getProgress() service.PipelineProgress {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Progress
}

func (j *runningJob) setStatus(status string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = status
}

func (j *runningJob) getStatus() string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.Status
}

// jobManager tracks running pipeline jobs.
type jobManager struct {
	mu   sync.RWMutex
	jobs map[string]*runningJob // projectID -> runningJob
}

func newJobManager() *jobManager {
	return &jobManager{jobs: make(map[string]*runningJob)}
}

func (jm *jobManager) get(projectID string) *runningJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.jobs[projectID]
}

func (jm *jobManager) start(projectID string, cancel context.CancelFunc) *runningJob {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	job := &runningJob{
		JobID:     uuid.New().String(),
		ProjectID: projectID,
		Status:    JobStatusRunning,
		StartedAt: time.Now(),
		Cancel:    cancel,
	}
	jm.jobs[projectID] = job
	return job
}

func (jm *jobManager) remove(projectID string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	delete(jm.jobs, projectID)
}

// handleRunPipeline starts an async pipeline execution.
func (s *Server) handleRunPipeline(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	// Check project exists
	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	// Parse optional dryRun flag
	var body struct {
		DryRun bool `json:"dryRun"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body)
	}

	// Check for duplicate execution
	if existing := s.jobs.get(projectID); existing != nil && existing.getStatus() == JobStatusRunning {
		WriteError(w, r, http.StatusConflict, "CONFLICT", "pipeline is already running for this project")
		return
	}

	// Create a job record in the database
	jobID := uuid.New().String()
	dbJob := &domain.Job{
		ID:        jobID,
		ProjectID: projectID,
		Type:      "pipeline_run",
		Status:    JobStatusRunning,
	}
	if body.DryRun {
		dbJob.Type = "dry_run"
	}
	if err := s.store.CreateJob(dbJob); err != nil {
		WriteError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create job")
		return
	}

	// Start background execution
	ctx, cancel := context.WithCancel(context.Background())
	job := s.jobs.start(projectID, cancel)
	job.JobID = jobID

	go s.executePipeline(ctx, job, project, body.DryRun)

	WriteJSON(w, r, http.StatusAccepted, map[string]string{
		"job_id":     jobID,
		"project_id": projectID,
		"status":     JobStatusRunning,
	})
}

// executePipeline runs the pipeline in a background goroutine.
func (s *Server) executePipeline(ctx context.Context, job *runningJob, project *domain.Project, dryRun bool) {
	defer func() {
		// Keep job in manager for status queries; mark as done
		if r := recover(); r != nil {
			slog.Error("pipeline panic", "error", r, "project_id", project.ID)
			job.setStatus(JobStatusFailed)
			s.updateJobRecord(job.JobID, JobStatusFailed, 0, "", "panic in pipeline")
		}
	}()

	if dryRun {
		slog.Info("dry-run pipeline started", "project_id", project.ID)
		job.setStatus(JobStatusComplete)
		s.updateJobRecord(job.JobID, JobStatusComplete, 100, "dry-run complete", "")
		return
	}

	slog.Info("pipeline execution started via API", "project_id", project.ID, "scp_id", project.SCPID)

	// For now, we signal that the pipeline has been accepted.
	// Full pipeline execution requires plugin instances which are created
	// in the CLI layer. The API records the job for tracking.
	// In production, this would call pipeline.Runner.Run() with injected plugins.

	select {
	case <-ctx.Done():
		job.setStatus(JobStatusCancelled)
		s.updateJobRecord(job.JobID, JobStatusCancelled, 0, "", "cancelled by user")
		slog.Info("pipeline cancelled", "project_id", project.ID)
	case <-time.After(100 * time.Millisecond):
		// Pipeline execution would happen here
		job.setStatus(JobStatusComplete)
		s.updateJobRecord(job.JobID, JobStatusComplete, 100, "pipeline accepted", "")
	}
}

func (s *Server) updateJobRecord(jobID, status string, progress int, result, errMsg string) {
	j, err := s.store.GetJob(jobID)
	if err != nil {
		slog.Error("failed to get job for update", "job_id", jobID, "error", err)
		return
	}
	j.Status = status
	j.Progress = progress
	j.Result = result
	j.Error = errMsg
	if err := s.store.UpdateJob(j); err != nil {
		slog.Error("failed to update job", "job_id", jobID, "error", err)
	}
}

// handleGetStatus returns real-time pipeline status.
func (s *Server) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	resp := map[string]interface{}{
		"project_id": project.ID,
		"scp_id":     project.SCPID,
		"state":      project.Status,
	}

	// Check if there's an active job
	if job := s.jobs.get(projectID); job != nil {
		progress := job.getProgress()
		resp["job_id"] = job.JobID
		resp["job_status"] = job.getStatus()
		resp["stage"] = progress.Stage
		resp["progress_pct"] = progress.ProgressPct
		resp["scenes_total"] = progress.ScenesTotal
		resp["scenes_complete"] = progress.ScenesComplete
		resp["elapsed_sec"] = time.Since(job.StartedAt).Seconds()
	}

	WriteJSON(w, r, http.StatusOK, resp)
}

// handleCancelPipeline cancels a running pipeline.
func (s *Server) handleCancelPipeline(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	// Verify project exists
	if _, err := s.store.GetProject(projectID); err != nil {
		writeServiceError(w, r, err)
		return
	}

	job := s.jobs.get(projectID)
	if job == nil || job.getStatus() != JobStatusRunning {
		WriteError(w, r, http.StatusConflict, "CONFLICT", "no running pipeline to cancel")
		return
	}

	job.Cancel()
	job.setStatus(JobStatusCancelled)

	WriteJSON(w, r, http.StatusOK, map[string]string{
		"project_id": projectID,
		"status":     JobStatusCancelled,
	})
}

// handleApprovePipeline approves the scenario and allows pipeline to continue.
func (s *Server) handleApprovePipeline(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	if project.Status != domain.StatusScenarioReview {
		WriteError(w, r, http.StatusConflict, "CONFLICT",
			"project is in '"+project.Status+"' state; must be in 'scenario_review' to approve")
		return
	}

	// Transition to approved
	previousState := project.Status
	updated, err := s.projectSvc.TransitionProject(r.Context(), projectID, domain.StatusApproved)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	// Fire webhook notification
	s.webhooks.NotifyStateChange(projectID, project.SCPID, previousState, domain.StatusApproved)

	WriteJSON(w, r, http.StatusOK, toProjectResponse(updated))
}
