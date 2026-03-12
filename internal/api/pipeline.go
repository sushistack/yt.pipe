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
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/pipeline"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// JobStatus constants
const (
	JobStatusRunning          = "running"
	JobStatusComplete         = "complete"
	JobStatusFailed           = "failed"
	JobStatusCancelled        = "cancelled"
	JobStatusWaitingApproval  = "waiting_approval"
)

// RunMode constants for POST /projects/{id}/run
const (
	RunModeScenario = "scenario" // Default: generate scenario + pause at scenario_review
	RunModeFull     = "full"     // Full pipeline execution
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
// The key is "projectID" for pipeline jobs, or "projectID:jobType" for typed jobs.
type jobManager struct {
	mu   sync.RWMutex
	jobs map[string]*runningJob // key -> runningJob
}

func newJobManager() *jobManager {
	return &jobManager{jobs: make(map[string]*runningJob)}
}

func (jm *jobManager) get(projectID string) *runningJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.jobs[projectID]
}

// getByType returns a running job for the given project and job type.
func (jm *jobManager) getByType(projectID, jobType string) *runningJob {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return jm.jobs[projectID+":"+jobType]
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

// startTyped creates and tracks a typed job (e.g., image_generate, tts_generate).
func (jm *jobManager) startTyped(projectID, jobType, jobID string, cancel context.CancelFunc) *runningJob {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	job := &runningJob{
		JobID:     jobID,
		ProjectID: projectID,
		Status:    JobStatusRunning,
		StartedAt: time.Now(),
		Cancel:    cancel,
	}
	jm.jobs[projectID+":"+jobType] = job
	return job
}

func (jm *jobManager) remove(projectID string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	delete(jm.jobs, projectID)
}

// removeTyped removes a typed job from the manager.
func (jm *jobManager) removeTyped(projectID, jobType string) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	delete(jm.jobs, projectID+":"+jobType)
}

// handleRunPipeline starts an async pipeline execution.
func (s *Server) handleRunPipeline(w http.ResponseWriter, r *http.Request) {
	if !s.requirePlugin(w, r, "llm") {
		return
	}

	projectID := chi.URLParam(r, "id")

	// Check project exists
	project, err := s.store.GetProject(projectID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	// Parse request body: mode and dryRun
	var body struct {
		Mode   string `json:"mode"`   // "scenario" (default) or "full"
		DryRun bool   `json:"dryRun"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body)
	}
	if body.Mode == "" {
		body.Mode = RunModeScenario
	}
	if body.Mode != RunModeScenario && body.Mode != RunModeFull {
		WriteError(w, r, http.StatusBadRequest, "BAD_REQUEST", "mode must be 'scenario' or 'full'")
		return
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

	go s.executePipeline(ctx, job, project, body.DryRun, body.Mode)

	WriteJSON(w, r, http.StatusAccepted, map[string]string{
		"job_id":     jobID,
		"project_id": projectID,
		"status":     JobStatusRunning,
	})
}

// executePipeline runs the pipeline in a background goroutine.
// mode is either RunModeScenario (default) or RunModeFull.
func (s *Server) executePipeline(ctx context.Context, job *runningJob, project *domain.Project, dryRun bool, mode string) {
	defer func() {
		// Keep job in manager for status queries; mark as done
		if r := recover(); r != nil {
			slog.Error("pipeline panic", "error", r, "project_id", project.ID)
			job.setStatus(JobStatusFailed)
			s.updateJobRecord(job.JobID, JobStatusFailed, 0, "", "panic in pipeline")
		}
	}()

	jobType := "pipeline_run"
	if dryRun {
		jobType = "dry_run"
		slog.Info("dry-run pipeline started", "project_id", project.ID)
		job.setStatus(JobStatusComplete)
		s.updateJobRecord(job.JobID, JobStatusComplete, 100, "dry-run complete", "")
		s.webhooks.NotifyJobComplete(project.ID, project.SCPID, job.JobID, jobType, "dry-run complete", project.Status, BuildReviewURL(project.ID, project.ReviewToken))
		return
	}

	slog.Info("pipeline execution started via API",
		"project_id", project.ID, "scp_id", project.SCPID, "mode", mode)

	switch mode {
	case RunModeFull:
		s.executeFullPipeline(ctx, job, project)
	default:
		s.executeScenarioOnly(ctx, job, project)
	}
}

// executeScenarioOnly generates the scenario and pauses at scenario_review.
// This is the default mode for n8n workflow orchestration.
func (s *Server) executeScenarioOnly(ctx context.Context, job *runningJob, project *domain.Project) {
	if s.scenarioSvc == nil {
		slog.Error("scenario service not available", "project_id", project.ID)
		job.setStatus(JobStatusFailed)
		s.updateJobRecord(job.JobID, JobStatusFailed, 0, "", "scenario service not configured")
		s.webhooks.NotifyJobFailed(project.ID, project.SCPID, job.JobID, "pipeline_run", "scenario service not configured", 0, project.Status, BuildReviewURL(project.ID, project.ReviewToken))
		return
	}

	// Load SCP data
	scpData, err := workspace.LoadSCPData(s.cfg.SCPDataPath, project.SCPID)
	if err != nil {
		slog.Error("failed to load SCP data", "project_id", project.ID, "error", err)
		job.setStatus(JobStatusFailed)
		s.updateJobRecord(job.JobID, JobStatusFailed, 0, "", "load SCP data: "+err.Error())
		s.webhooks.NotifyJobFailed(project.ID, project.SCPID, job.JobID, "pipeline_run", err.Error(), 0, project.Status, BuildReviewURL(project.ID, project.ReviewToken))
		return
	}

	// Check for cancellation
	select {
	case <-ctx.Done():
		job.setStatus(JobStatusCancelled)
		s.updateJobRecord(job.JobID, JobStatusCancelled, 0, "", "cancelled by user")
		return
	default:
	}

	// Generate scenario for the existing project
	previousState := project.Status
	_, err = s.scenarioSvc.GenerateScenarioForProject(ctx, project, scpData)
	if err != nil {
		slog.Error("scenario generation failed", "project_id", project.ID, "error", err)
		job.setStatus(JobStatusFailed)
		s.updateJobRecord(job.JobID, JobStatusFailed, 0, "", "scenario generation: "+err.Error())
		s.webhooks.NotifyJobFailed(project.ID, project.SCPID, job.JobID, "pipeline_run", err.Error(), 0, project.Status, BuildReviewURL(project.ID, project.ReviewToken))
		return
	}

	// Scenario generated successfully, project is now in scenario_review
	job.setStatus(JobStatusWaitingApproval)
	job.updateProgress(service.PipelineProgress{
		Stage:       "scenario_review",
		ProgressPct: 100,
	})
	s.updateJobRecord(job.JobID, JobStatusWaitingApproval, 100, "scenario_review", "")

	// Fire state_change webhook
	s.webhooks.NotifyStateChange(project.ID, project.SCPID, previousState, domain.StatusScenarioReview, BuildReviewURL(project.ID, project.ReviewToken))

	slog.Info("scenario generation complete, waiting for approval",
		"project_id", project.ID, "scp_id", project.SCPID)
}

// executeFullPipeline runs the complete pipeline using pipeline.Runner.
func (s *Server) executeFullPipeline(ctx context.Context, job *runningJob, project *domain.Project) {
	if s.pipelineRunner == nil {
		slog.Error("pipeline runner not available", "project_id", project.ID)
		job.setStatus(JobStatusFailed)
		s.updateJobRecord(job.JobID, JobStatusFailed, 0, "", "pipeline runner not configured")
		s.webhooks.NotifyJobFailed(project.ID, project.SCPID, job.JobID, "pipeline_run", "pipeline runner not configured", 0, project.Status, BuildReviewURL(project.ID, project.ReviewToken))
		return
	}

	// Set progress callback to update job record
	s.pipelineRunner.ProgressFunc = func(p service.PipelineProgress) {
		job.updateProgress(p)
		s.updateJobRecord(job.JobID, JobStatusRunning, int(p.ProgressPct), string(p.Stage), "")
	}

	// Run the full pipeline with auto-approve for API mode
	result, err := s.pipelineRunner.RunWithOptions(ctx, project.SCPID, pipeline.RunOptions{
		AutoApprove: true,
	})

	if err != nil {
		failedStage := ""
		if result != nil {
			failedStage = result.PausedAt
			for _, st := range result.Stages {
				if st.Status == "fail" {
					failedStage = st.Name
					break
				}
			}
		}
		slog.Error("full pipeline failed",
			"project_id", project.ID, "error", err, "failed_stage", failedStage)
		job.setStatus(JobStatusFailed)
		s.updateJobRecord(job.JobID, JobStatusFailed, 0, "", err.Error())
		s.webhooks.NotifyJobFailed(project.ID, project.SCPID, job.JobID, "pipeline_run", err.Error(), 0, project.Status, BuildReviewURL(project.ID, project.ReviewToken))
		return
	}

	job.setStatus(JobStatusComplete)
	s.updateJobRecord(job.JobID, JobStatusComplete, 100, result.Status, "")
	s.webhooks.NotifyJobComplete(project.ID, project.SCPID, job.JobID, "pipeline_run", result.Status, domain.StatusComplete, BuildReviewURL(project.ID, project.ReviewToken))

	slog.Info("full pipeline complete",
		"project_id", project.ID, "scp_id", project.SCPID,
		"status", result.Status, "elapsed", result.TotalElapsed)
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

	// Check if there's an active in-memory job
	if job := s.jobs.get(projectID); job != nil {
		progress := job.getProgress()
		resp["job_id"] = job.JobID
		resp["job_status"] = job.getStatus()
		resp["stage"] = progress.Stage
		resp["progress_pct"] = progress.ProgressPct
		resp["scenes_total"] = progress.ScenesTotal
		resp["scenes_complete"] = progress.ScenesComplete
		resp["elapsed_sec"] = time.Since(job.StartedAt).Seconds()
	} else {
		// Fallback to DB: return the most recent job record
		dbJob, err := s.store.GetLatestJobByProject(projectID)
		if err != nil {
			slog.Error("failed to get latest job from DB", "project_id", projectID, "error", err)
		} else if dbJob != nil {
			resp["job_id"] = dbJob.ID
			resp["job_status"] = dbJob.Status
			resp["progress_pct"] = dbJob.Progress
			if dbJob.Result != "" {
				resp["result"] = dbJob.Result
			}
			if dbJob.Error != "" {
				resp["error"] = dbJob.Error
			}
			elapsed := dbJob.UpdatedAt.Sub(dbJob.CreatedAt).Seconds()
			resp["elapsed_sec"] = elapsed
		}
	}

	WriteJSON(w, r, http.StatusOK, resp)
}

// handleGetJob returns full details of a specific job by ID.
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")

	j, err := s.store.GetJob(jobID)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	resp := map[string]interface{}{
		"job_id":       j.ID,
		"project_id":   j.ProjectID,
		"type":         j.Type,
		"status":       j.Status,
		"progress":     j.Progress,
		"started_at":   j.CreatedAt.Format(time.RFC3339),
		"elapsed_sec":  j.UpdatedAt.Sub(j.CreatedAt).Seconds(),
	}

	if j.Status == JobStatusComplete || j.Status == JobStatusFailed || j.Status == JobStatusCancelled {
		resp["completed_at"] = j.UpdatedAt.Format(time.RFC3339)
	}

	if j.Result != "" {
		resp["result"] = j.Result
	}
	if j.Error != "" {
		resp["error"] = j.Error
	}

	WriteJSON(w, r, http.StatusOK, resp)
}

// InitJobLifecycle performs startup job lifecycle tasks:
// 1. Marks stale "running" jobs as "failed" (server restart recovery)
// 2. Purges old completed/failed jobs beyond the retention period
func (s *Server) InitJobLifecycle() {
	// Mark stale jobs
	count, err := s.store.MarkStaleJobsFailed("server restarted")
	if err != nil {
		slog.Error("failed to mark stale jobs", "error", err)
	} else if count > 0 {
		slog.Info("marked stale jobs as failed", "count", count)
	}

	// Purge old jobs
	retentionDays := s.cfg.JobRetentionDays
	if retentionDays <= 0 {
		retentionDays = 7
	}
	purged, err := s.store.PurgeOldJobs(retentionDays)
	if err != nil {
		slog.Error("failed to purge old jobs", "error", err)
	} else if purged > 0 {
		slog.Info("purged old jobs", "count", purged, "retention_days", retentionDays)
	}
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
	s.webhooks.NotifyStateChange(projectID, project.SCPID, previousState, domain.StatusApproved, BuildReviewURL(projectID, project.ReviewToken))

	WriteJSON(w, r, http.StatusOK, toProjectResponse(updated))
}
