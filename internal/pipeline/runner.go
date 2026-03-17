// Package pipeline provides pipeline orchestration for the youtube content pipeline.
package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/plugin/output"
	"github.com/sushistack/yt.pipe/internal/plugin/tts"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// Runner executes the full content generation pipeline.
type Runner struct {
	store        *store.Store
	llm          llm.LLM
	imageGen     imagegen.ImageGen
	tts          tts.TTS
	assembler    output.Assembler
	glossary     *glossary.Glossary
	logger       *slog.Logger
	scpDataPath  string
	workspacePath string
	voice        string
	imageOpts    imagegen.GenerateOptions
	canvas       output.CanvasConfig
	templatePath         string
	metaPath             string
	templatesPath        string
	defaultSceneDuration float64

	characterSvc               *service.CharacterService
	selectedCharacterImagePath string
	styleConfig                domain.StyleConfig

	// ProgressFunc is called on stage transitions for real-time feedback.
	ProgressFunc func(service.PipelineProgress)
}

// RunnerConfig holds configuration for the pipeline runner.
type RunnerConfig struct {
	SCPDataPath          string
	WorkspacePath        string
	Voice                string
	ImageOpts            imagegen.GenerateOptions
	Canvas               output.CanvasConfig
	TemplatePath         string
	MetaPath             string
	TemplatesPath        string
	DefaultSceneDuration float64
	CharacterSvc         *service.CharacterService
}

// NewRunner creates a new pipeline Runner.
func NewRunner(
	s *store.Store,
	l llm.LLM,
	ig imagegen.ImageGen,
	t tts.TTS,
	a output.Assembler,
	g *glossary.Glossary,
	logger *slog.Logger,
	cfg RunnerConfig,
) *Runner {
	return &Runner{
		store:         s,
		llm:           l,
		imageGen:      ig,
		tts:           t,
		assembler:     a,
		glossary:      g,
		logger:        logger,
		scpDataPath:   cfg.SCPDataPath,
		workspacePath: cfg.WorkspacePath,
		voice:         cfg.Voice,
		imageOpts:     cfg.ImageOpts,
		canvas:        cfg.Canvas,
		templatePath:  cfg.TemplatePath,
		metaPath:      cfg.MetaPath,
		templatesPath:        cfg.TemplatesPath,
		defaultSceneDuration: cfg.DefaultSceneDuration,
		characterSvc:         cfg.CharacterSvc,
	}
}

// RunResult contains the result of a full pipeline run.
type RunResult struct {
	ProjectID    string        `json:"project_id"`
	SCPID        string        `json:"scp_id"`
	Status       string        `json:"status"`
	SceneCount   int           `json:"scene_count"`
	Stages       []StageResult `json:"stages"`
	TotalElapsed time.Duration `json:"total_elapsed"`
	PausedAt     string        `json:"paused_at,omitempty"`
	APICalls     int           `json:"api_calls,omitempty"`
	EstimatedCost float64      `json:"estimated_cost,omitempty"`
}

// RunOptions configures how the pipeline executes.
type RunOptions struct {
	AutoApprove  bool
	SkipApproval bool // Skip image/TTS per-scene approval (auto-approve all)
	Force        bool // Clear checkpoints and start from scratch
}

// Run executes the full pipeline for a given SCP ID.
// It runs stages sequentially: data_load → scenario_generate → (pause for approval) →
// image_generate + tts_synthesize (parallel) → timing_resolve → subtitle_generate → assemble.
func (r *Runner) Run(ctx context.Context, scpID string) (*RunResult, error) {
	return r.RunWithOptions(ctx, scpID, RunOptions{})
}

// RunWithOptions executes the full pipeline with configurable options.
func (r *Runner) RunWithOptions(ctx context.Context, scpID string, opts RunOptions) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{SCPID: scpID, Stages: make([]StageResult, 0, 8)}

	// When AutoApprove is set, also skip image/TTS approval for backward compatibility
	skipApproval := opts.SkipApproval || opts.AutoApprove

	r.logger.Info("pipeline started", "scp_id", scpID, "auto_approve", opts.AutoApprove, "skip_approval", skipApproval, "force", opts.Force)

	// If --force, backup and clear existing checkpoints
	if opts.Force {
		if project, _ := r.findProject(ctx, scpID); project != nil {
			if err := BackupAndClearCheckpoints(project.WorkspacePath); err != nil {
				r.logger.Warn("failed to backup checkpoints", "err", err)
			} else {
				r.logger.Info("cleared checkpoints for fresh run", "scp_id", scpID)
			}
		}
	}

	// Check for existing project with checkpoint (resume support)
	existingProject, checkpoint := r.findExistingCheckpoint(scpID, opts.Force)

	if existingProject != nil && checkpoint != nil {
		skipped := 0
		for _, sc := range checkpoint.Stages {
			result.Stages = append(result.Stages, StageResult{
				Name:   string(sc.Stage),
				Status: "skipped",
			})
			skipped++
		}
		r.logger.Info("resuming from checkpoint",
			"scp_id", scpID,
			"project_id", existingProject.ID,
			"stages_completed", skipped,
			"last_stage", checkpoint.LastStage)

		result.ProjectID = existingProject.ID
		result.SceneCount = existingProject.SceneCount

		// Determine where to resume based on asset existence (dependency-based)
		if checkpoint.HasCompletedStage(service.StageScenarioGenerate) {
			scenario, err := service.LoadScenarioFromFile(existingProject.WorkspacePath + "/scenario.json")
			if err != nil {
				return nil, fmt.Errorf("pipeline: load scenario for resume: %w", err)
			}
			resumeResult, err := r.resumeFromApproval(ctx, existingProject, scenario, start, skipApproval)
			if err != nil {
				return resumeResult, err
			}
			result.Stages = append(result.Stages, resumeResult.Stages...)
			result.Status = resumeResult.Status
			result.TotalElapsed = time.Since(start)
			result.APICalls = countAPICalls(result)
			result.EstimatedCost = estimateCost(result)
			return result, nil
		}
	}

	// Stage 1: Load SCP Data
	r.reportProgress(service.PipelineProgress{Stage: service.StageDataLoad, StartedAt: start})
	stageStart := time.Now()
	scpData, err := r.runDataLoad(ctx, scpID)
	result.Stages = append(result.Stages, stageResult(string(service.StageDataLoad), stageStart, err))
	if err != nil {
		result.Status = "failed"
		return result, r.pipelineError(service.StageDataLoad, 0, err, scpID)
	}

	// Stage 2: Generate Scenario
	r.reportProgress(service.PipelineProgress{Stage: service.StageScenarioGenerate, StartedAt: start})
	stageStart = time.Now()
	scenario, project, err := r.runScenarioGenerate(ctx, scpData)
	result.Stages = append(result.Stages, stageResult(string(service.StageScenarioGenerate), stageStart, err))
	if err != nil {
		result.Status = "failed"
		return result, r.pipelineError(service.StageScenarioGenerate, 0, err, scpID)
	}
	result.ProjectID = project.ID
	result.SceneCount = len(scenario.Scenes)

	// Save checkpoint after scenario generation
	r.saveCheckpointAfterStage(project.WorkspacePath, project.ID, service.StageScenarioGenerate, len(scenario.Scenes))

	// Stage 3: Approval gate
	if opts.AutoApprove {
		r.logger.Warn("auto-approve enabled: scenario review skipped",
			"scp_id", scpID, "project_id", project.ID)
		result.Stages = append(result.Stages, StageResult{
			Name:       string(service.StageScenarioApproval),
			Status:     "auto-approved",
			DurationMs: 0,
		})

		// Auto-approve: transition to approved and continue
		projectSvc := service.NewProjectService(r.store)
		scenarioSvc := service.NewScenarioService(r.store, nil, projectSvc)
		project, err = scenarioSvc.ApproveScenario(ctx, project.ID)
		if err != nil {
			result.Status = "failed"
			return result, fmt.Errorf("pipeline: auto-approve: %w", err)
		}

		// Continue with remaining stages (same as Resume)
		resumeResult, err := r.resumeFromApproval(ctx, project, scenario, start, skipApproval)
		if err != nil {
			return resumeResult, err
		}
		// Merge stages
		result.Stages = append(result.Stages, resumeResult.Stages...)
		result.Status = resumeResult.Status
		result.TotalElapsed = time.Since(start)
		result.APICalls = countAPICalls(result)
		result.EstimatedCost = estimateCost(result)
		return result, nil
	}

	// Normal flow: Pause for approval
	r.reportProgress(service.PipelineProgress{Stage: service.StageScenarioApproval, StartedAt: start})
	result.Stages = append(result.Stages, StageResult{
		Name:       string(service.StageScenarioApproval),
		Status:     "paused",
		DurationMs: time.Since(stageStart).Milliseconds(),
	})
	result.Status = "awaiting_approval"
	result.PausedAt = string(service.StageScenarioApproval)
	result.TotalElapsed = time.Since(start)

	r.logger.Info("pipeline paused for scenario approval",
		"scp_id", scpID,
		"project_id", project.ID,
		"scene_count", len(scenario.Scenes))

	return result, nil
}

// Resume continues the pipeline after scenario/image/TTS approval.
// Handles projects in "approved", "image_review", or "tts_review" state.
func (r *Runner) Resume(ctx context.Context, projectID string) (*RunResult, error) {
	return r.ResumeWithOptions(ctx, projectID, false)
}

// ResumeWithOptions continues the pipeline with configurable skip-approval.
func (r *Runner) ResumeWithOptions(ctx context.Context, projectID string, skipApproval bool) (*RunResult, error) {
	start := time.Now()

	projectSvc := service.NewProjectService(r.store)
	project, err := projectSvc.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: get project: %w", err)
	}

	// Load scenario from workspace — the runner determines what to do based on asset existence
	scenario, err := service.LoadScenarioFromFile(project.WorkspacePath + "/scenario.json")
	if err != nil {
		return nil, fmt.Errorf("pipeline: load scenario: %w", err)
	}

	result, err := r.resumeFromApproval(ctx, project, scenario, start, skipApproval)
	if err != nil {
		return result, err
	}
	result.APICalls = countAPICalls(result)
	result.EstimatedCost = estimateCost(result)
	return result, nil
}

// resumeFromApproval runs all post-approval stages. Shared by Resume() and RunWithOptions() auto-approve.
// When skipApproval is true, uses the legacy parallel generation path (backward compatible).
// When skipApproval is false, pauses at image_review and tts_review for per-scene approval.
func (r *Runner) resumeFromApproval(ctx context.Context, project *domain.Project, scenario *domain.ScenarioOutput, start time.Time, skipApproval bool) (*RunResult, error) {
	result := &RunResult{
		ProjectID:  project.ID,
		SCPID:      project.SCPID,
		SceneCount: project.SceneCount,
		Stages:     make([]StageResult, 0, 6),
	}

	projectSvc := service.NewProjectService(r.store)
	approvalSvc := service.NewApprovalService(r.store, r.logger)
	sceneCount := len(scenario.Scenes)
	if sceneCount == 0 {
		sceneCount = project.SceneCount
	}

	r.logger.Info("pipeline resumed", "project_id", project.ID, "scp_id", project.SCPID, "skip_approval", skipApproval, "current_status", project.Status)

	// Character gate: must have selected character before image generation
	if r.characterSvc != nil {
		char, err := r.characterSvc.CheckExistingCharacter(project.SCPID)
		if err != nil {
			return nil, fmt.Errorf("pipeline: check character: %w", err)
		}
		if char == nil || char.SelectedImagePath == "" {
			return nil, &domain.DependencyError{
				Action: "image_generate", Missing: []string{"character"},
			}
		}
		r.selectedCharacterImagePath = char.SelectedImagePath
	}

	// --- Skip-Approval Path: Use legacy parallel generation (backward compatible) ---
	if skipApproval {
		return r.runSkipApprovalPath(ctx, project, scenario, start, result, projectSvc, approvalSvc, sceneCount)
	}

	// --- Approval Path: Sequential with pause at image_review and tts_review ---
	return r.runApprovalPath(ctx, project, scenario, start, result, projectSvc, approvalSvc, sceneCount)
}

// runSkipApprovalPath executes the legacy parallel generation path with auto-approval.
func (r *Runner) runSkipApprovalPath(ctx context.Context, project *domain.Project, scenario *domain.ScenarioOutput, start time.Time, result *RunResult, projectSvc *service.ProjectService, approvalSvc *service.ApprovalService, sceneCount int) (*RunResult, error) {
	// Initialize and auto-approve all image/TTS approvals
	_ = approvalSvc.InitApprovals(project.ID, sceneCount, domain.AssetTypeImage)
	_ = approvalSvc.AutoApproveAll(project.ID, domain.AssetTypeImage)
	_ = approvalSvc.InitApprovals(project.ID, sceneCount, domain.AssetTypeTTS)
	_ = approvalSvc.AutoApproveAll(project.ID, domain.AssetTypeTTS)

	// Stage 4 & 5: Image generation + TTS synthesis (parallel)
	r.reportProgress(service.PipelineProgress{
		Stage:       service.StageImageGenerate,
		ScenesTotal: project.SceneCount,
		StartedAt:   start,
	})

	parallelStart := time.Now()
	imageScenes, ttsScenes, err := r.runParallelGeneration(ctx, scenario, project)
	parallelDur := time.Since(parallelStart).Milliseconds()
	if err != nil {
		result.Status = "failed"
		return result, err
	}
	result.Stages = append(result.Stages,
		StageResult{Name: string(service.StageImageGenerate), Status: "pass", DurationMs: parallelDur},
		StageResult{Name: string(service.StageTTSSynthesize), Status: "pass", DurationMs: parallelDur},
	)
	r.saveCheckpointAfterStage(project.WorkspacePath, project.ID, service.StageImageGenerate, len(imageScenes))
	r.saveCheckpointAfterStage(project.WorkspacePath, project.ID, service.StageTTSSynthesize, len(ttsScenes))

	return r.runPostGenerationStages(ctx, project, imageScenes, ttsScenes, start, result, projectSvc)
}

// runApprovalPath executes the per-scene approval flow with pauses at image_review and tts_review.
func (r *Runner) runApprovalPath(ctx context.Context, project *domain.Project, scenario *domain.ScenarioOutput, start time.Time, result *RunResult, projectSvc *service.ProjectService, approvalSvc *service.ApprovalService, sceneCount int) (*RunResult, error) {
	// Dependency-based flow: determine what to do based on asset existence
	hasImages := r.workspaceHasImages(project.WorkspacePath, sceneCount)
	hasTTS := r.workspaceHasTTS(project.WorkspacePath, sceneCount)

	// If images not yet generated, generate them and pause for approval
	if !hasImages {
		// Set stage to images
		if _, err := projectSvc.SetProjectStage(ctx, project.ID, domain.StageImages); err != nil {
			return nil, fmt.Errorf("pipeline: set stage to images: %w", err)
		}

		// Initialize image approval records
		if err := approvalSvc.InitApprovals(project.ID, sceneCount, domain.AssetTypeImage); err != nil {
			return nil, fmt.Errorf("pipeline: init image approvals: %w", err)
		}

		// Generate all images
		r.reportProgress(service.PipelineProgress{
			Stage:       service.StageImageGenerate,
			ScenesTotal: project.SceneCount,
			StartedAt:   start,
		})
		stageStart := time.Now()
		imgSvc := service.NewImageGenService(r.imageGen, r.store, r.logger)
		r.wireCharacterToImageSvc(imgSvc)
		_, genErr := r.generateShotImages(ctx, scenario, imgSvc, project)
		result.Stages = append(result.Stages, stageResult(string(service.StageImageGenerate), stageStart, genErr))
		if genErr != nil {
			result.Status = "failed"
			return result, r.pipelineError(service.StageImageGenerate, 0, genErr, project.SCPID)
		}
		r.saveCheckpointAfterStage(project.WorkspacePath, project.ID, service.StageImageGenerate, sceneCount)

		// Mark all images as generated
		for i := 1; i <= sceneCount; i++ {
			_ = approvalSvc.MarkGenerated(project.ID, i, domain.AssetTypeImage)
		}

		// Pause for image approval
		result.Status = "awaiting_image_approval"
		result.PausedAt = "image_review"
		result.TotalElapsed = time.Since(start)
		r.logger.Info("pipeline paused for image approval",
			"project_id", project.ID, "scene_count", sceneCount)
		return result, nil
	}

	// Images exist — check if all images are approved before proceeding
	allImgApproved, err := approvalSvc.AllApproved(project.ID, domain.AssetTypeImage)
	if err != nil {
		// No approval records means images were auto-approved or from skip-approval path
		allImgApproved = true
	}
	if !allImgApproved {
		imgStatus, _ := approvalSvc.GetApprovalStatus(project.ID, domain.AssetTypeImage)
		return nil, fmt.Errorf("pipeline: not all images approved (%d/%d). Use: yt-pipe scenes approve %s --type image --scene <num>",
			imgStatus.Approved, imgStatus.Total, project.ID)
	}

	// If TTS not yet generated, generate it and pause for approval
	if !hasTTS {
		// Set stage to TTS
		if _, err := projectSvc.SetProjectStage(ctx, project.ID, domain.StageTTS); err != nil {
			return nil, fmt.Errorf("pipeline: set stage to tts: %w", err)
		}

		// Initialize TTS approval records
		if err := approvalSvc.InitApprovals(project.ID, sceneCount, domain.AssetTypeTTS); err != nil {
			return nil, fmt.Errorf("pipeline: init tts approvals: %w", err)
		}

		// Generate all TTS
		r.reportProgress(service.PipelineProgress{
			Stage:       service.StageTTSSynthesize,
			ScenesTotal: project.SceneCount,
			StartedAt:   start,
		})
		stageStart := time.Now()
		ttsSvc := service.NewTTSService(r.tts, r.glossary, r.store, r.logger)
		_, err = ttsSvc.SynthesizeAll(ctx, scenario.Scenes, project.ID, project.WorkspacePath, r.voice, nil)
		result.Stages = append(result.Stages, stageResult(string(service.StageTTSSynthesize), stageStart, err))
		if err != nil {
			result.Status = "failed"
			return result, r.pipelineError(service.StageTTSSynthesize, 0, err, project.SCPID)
		}
		r.saveCheckpointAfterStage(project.WorkspacePath, project.ID, service.StageTTSSynthesize, sceneCount)

		// Mark all TTS as generated
		for i := 1; i <= sceneCount; i++ {
			_ = approvalSvc.MarkGenerated(project.ID, i, domain.AssetTypeTTS)
		}

		// Pause for TTS approval
		result.Status = "awaiting_tts_approval"
		result.PausedAt = "tts_review"
		result.TotalElapsed = time.Since(start)
		r.logger.Info("pipeline paused for TTS approval",
			"project_id", project.ID, "scene_count", sceneCount)
		return result, nil
	}

	// TTS exists — check if all TTS are approved before proceeding
	allTTSApproved, err := approvalSvc.AllApproved(project.ID, domain.AssetTypeTTS)
	if err != nil {
		allTTSApproved = true
	}
	if !allTTSApproved {
		ttsStatus, _ := approvalSvc.GetApprovalStatus(project.ID, domain.AssetTypeTTS)
		return nil, fmt.Errorf("pipeline: not all TTS approved (%d/%d). Use: yt-pipe scenes approve %s --type tts --scene <num>",
			ttsStatus.Approved, ttsStatus.Total, project.ID)
	}

	// Both images and TTS exist and are approved — proceed to post-generation stages
	imageScenes, err := loadScenesFromDir(project.WorkspacePath)
	if err != nil {
		imageScenes = make([]*domain.Scene, 0)
	}
	ttsScenes, err := loadScenesFromDir(project.WorkspacePath)
	if err != nil {
		ttsScenes = make([]*domain.Scene, 0)
	}

	return r.runPostGenerationStages(ctx, project, imageScenes, ttsScenes, start, result, projectSvc)
}

// runPostGenerationStages runs timing, subtitle, and assembly stages after all assets are approved.
func (r *Runner) runPostGenerationStages(ctx context.Context, project *domain.Project, imageScenes, ttsScenes []*domain.Scene, start time.Time, result *RunResult, projectSvc *service.ProjectService) (*RunResult, error) {
	// Stage 6: Timing resolution
	r.reportProgress(service.PipelineProgress{
		Stage:       service.StageTimingResolve,
		ScenesTotal: project.SceneCount,
		StartedAt:   start,
	})
	stageStart := time.Now()
	timingResolver := service.NewTimingResolver(r.logger).WithDefaultSceneDuration(r.defaultSceneDuration)
	timings := timingResolver.ResolveTimings(ttsScenes)
	if err := timingResolver.SaveTimingFiles(timings, project.WorkspacePath); err != nil {
		result.Stages = append(result.Stages, stageResult(string(service.StageTimingResolve), stageStart, err))
		result.Status = "failed"
		return result, r.pipelineError(service.StageTimingResolve, 0, err, project.SCPID)
	}
	result.Stages = append(result.Stages, stageResult(string(service.StageTimingResolve), stageStart, nil))

	// Merge TTS word timings into scenes for subtitle generation
	mergedScenes := mergeSceneData(imageScenes, ttsScenes, timings)

	// Stage 7: Subtitle generation
	r.reportProgress(service.PipelineProgress{
		Stage:       service.StageSubtitleGenerate,
		ScenesTotal: project.SceneCount,
		StartedAt:   start,
	})
	stageStart = time.Now()
	subtitleSvc := service.NewSubtitleService(r.glossary, r.store, r.logger)
	if err := subtitleSvc.SaveAllSubtitles(mergedScenes, project.ID, project.WorkspacePath, 8, nil); err != nil {
		result.Stages = append(result.Stages, stageResult(string(service.StageSubtitleGenerate), stageStart, err))
		result.Status = "failed"
		return result, r.pipelineError(service.StageSubtitleGenerate, 0, err, project.SCPID)
	}
	for _, s := range mergedScenes {
		if s.SubtitlePath == "" {
			s.SubtitlePath = fmt.Sprintf("%s/scenes/%d/subtitle.json", project.WorkspacePath, s.SceneNum)
		}
	}
	result.Stages = append(result.Stages, stageResult(string(service.StageSubtitleGenerate), stageStart, nil))

	// Set stage to complete (assembly service also does this, but keep explicit for the runner path)
	if _, err := projectSvc.SetProjectStage(ctx, project.ID, domain.StageComplete); err != nil {
		r.logger.Warn("set stage to complete failed (may already be in correct state)", "err", err)
	}

	// Stage 8: Assembly
	r.reportProgress(service.PipelineProgress{
		Stage:       service.StageAssemble,
		ScenesTotal: project.SceneCount,
		StartedAt:   start,
	})
	stageStart = time.Now()
	assemblerSvc := service.NewAssemblerService(r.assembler, projectSvc)
	assemblerSvc.WithConfig(r.templatePath, r.metaPath, r.canvas)

	domainScenes := toDomainScenes(mergedScenes)
	assembleResult, err := assemblerSvc.Assemble(ctx, project.ID, domainScenes)
	result.Stages = append(result.Stages, stageResult(string(service.StageAssemble), stageStart, err))
	if err != nil {
		result.Status = "failed"
		return result, r.pipelineError(service.StageAssemble, 0, err, project.SCPID)
	}

	// Generate copyright notice and check special conditions
	r.generateCopyright(project, assemblerSvc)

	result.Status = "complete"
	result.TotalElapsed = time.Since(start)

	r.logger.Info("pipeline complete",
		"project_id", project.ID,
		"scp_id", project.SCPID,
		"scene_count", project.SceneCount,
		"duration_sec", assembleResult.TotalDuration,
		"elapsed", result.TotalElapsed)

	return result, nil
}

// countAPICalls estimates the number of API calls from stage results.
func countAPICalls(result *RunResult) int {
	calls := 0
	for _, s := range result.Stages {
		if s.Status != "pass" && s.Status != "auto-approved" {
			continue
		}
		switch s.Name {
		case string(service.StageScenarioGenerate):
			calls += 4 // 4-stage scenario pipeline
		case string(service.StageImageGenerate):
			calls += result.SceneCount
		case string(service.StageTTSSynthesize):
			calls += result.SceneCount
		}
	}
	return calls
}

// estimateCost provides a rough cost estimate based on API calls.
func estimateCost(result *RunResult) float64 {
	var cost float64
	for _, s := range result.Stages {
		if s.Status != "pass" && s.Status != "auto-approved" {
			continue
		}
		switch s.Name {
		case string(service.StageScenarioGenerate):
			cost += 0.02 // ~$0.02 for 4-stage LLM calls
		case string(service.StageImageGenerate):
			cost += float64(result.SceneCount) * 0.003 // ~$0.003 per image
		case string(service.StageTTSSynthesize):
			cost += float64(result.SceneCount) * 0.001 // ~$0.001 per TTS
		}
	}
	return cost
}

// findExistingCheckpoint looks for an existing project with checkpoint data for resume.
func (r *Runner) findExistingCheckpoint(scpID string, force bool) (*domain.Project, *service.PipelineCheckpoint) {
	if force {
		return nil, nil
	}

	projects, err := r.store.ListProjects()
	if err != nil {
		return nil, nil
	}
	for _, p := range projects {
		if p.SCPID != scpID {
			continue
		}
		cp, err := service.LoadCheckpoint(p.WorkspacePath)
		if err != nil {
			continue
		}
		if len(cp.Stages) > 0 {
			return p, cp
		}
	}
	return nil, nil
}

// saveCheckpointAfterStage saves checkpoint after a pipeline stage completes.
func (r *Runner) saveCheckpointAfterStage(projectPath, projectID string, stage service.PipelineStage, scenesDone int) {
	cm := NewCheckpointManager(r.logger)
	if err := cm.SaveStageCheckpoint(projectPath, projectID, stage, scenesDone); err != nil {
		r.logger.Warn("failed to save checkpoint", "stage", stage, "err", err)
	}
}

// backupAndClearCheckpoints backs up existing artifacts and clears checkpoint for --force mode.
func BackupAndClearCheckpoints(projectPath string) error {
	backupDir := filepath.Join(projectPath, "backup", time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("backup: create dir: %w", err)
	}
	// Move checkpoint file to backup
	cpPath := filepath.Join(projectPath, "checkpoint.json")
	if _, err := os.Stat(cpPath); err == nil {
		if err := os.Rename(cpPath, filepath.Join(backupDir, "checkpoint.json")); err != nil {
			return fmt.Errorf("backup: move checkpoint: %w", err)
		}
	}
	return nil
}

// RunStage executes a single pipeline stage by name.
func (r *Runner) RunStage(ctx context.Context, scpID string, stage service.PipelineStage) error {
	r.logger.Info("running single stage", "scp_id", scpID, "stage", stage)

	switch stage {
	case service.StageDataLoad:
		_, err := r.runDataLoad(ctx, scpID)
		return err
	case service.StageScenarioGenerate:
		scpData, err := r.runDataLoad(ctx, scpID)
		if err != nil {
			return err
		}
		_, _, err = r.runScenarioGenerate(ctx, scpData)
		return err
	case service.StageImageGenerate:
		// Character gate for single-stage execution
		if r.characterSvc != nil {
			char, err := r.characterSvc.CheckExistingCharacter(scpID)
			if err != nil {
				return fmt.Errorf("pipeline: check character: %w", err)
			}
			if char == nil || char.SelectedImagePath == "" {
				return &domain.DependencyError{
					Action: "image_generate", Missing: []string{"character"},
				}
			}
			r.selectedCharacterImagePath = char.SelectedImagePath
		}
		return r.runImageGenerateStage(ctx, scpID)
	case service.StageTTSSynthesize:
		return r.runTTSSynthesizeStage(ctx, scpID)
	case service.StageSubtitleGenerate:
		return r.runSubtitleGenerateStage(ctx, scpID)
	case service.StageAssemble:
		return r.runAssembleStage(ctx, scpID)
	default:
		return fmt.Errorf("pipeline: unknown stage %q", stage)
	}
}

func (r *Runner) runDataLoad(ctx context.Context, scpID string) (*workspace.SCPData, error) {
	r.logger.Info("stage started", "stage", service.StageDataLoad, "scp_id", scpID)
	start := time.Now()

	scpData, err := workspace.LoadSCPData(r.scpDataPath, scpID)
	if err != nil {
		return nil, fmt.Errorf("data load: %w", err)
	}

	r.logger.Info("stage complete", "stage", service.StageDataLoad, "scp_id", scpID,
		"duration_ms", time.Since(start).Milliseconds())
	return scpData, nil
}

func (r *Runner) runScenarioGenerate(ctx context.Context, scpData *workspace.SCPData) (*domain.ScenarioOutput, *domain.Project, error) {
	r.logger.Info("stage started", "stage", service.StageScenarioGenerate, "scp_id", scpData.SCPID)
	start := time.Now()

	projectSvc := service.NewProjectService(r.store)

	projectPath, err := workspace.InitProject(r.workspacePath, scpData.SCPID)
	if err != nil {
		return nil, nil, fmt.Errorf("scenario generate: init workspace: %w", err)
	}

	scenarioSvc := service.NewScenarioService(r.store, r.llm, projectSvc)
	if r.templatesPath != "" {
		scenarioSvc.SetTemplatesDir(r.templatesPath)
	}
	scenarioSvc.SetGlossary(r.glossary)
	scenario, project, err := scenarioSvc.GenerateScenario(ctx, scpData, projectPath)
	if err != nil {
		return nil, project, fmt.Errorf("scenario generate: %w", err)
	}

	r.logger.Info("stage complete", "stage", service.StageScenarioGenerate,
		"scp_id", scpData.SCPID, "scenes", len(scenario.Scenes),
		"mode", "legacy",
		"duration_ms", time.Since(start).Milliseconds())

	return scenario, project, nil
}

// runParallelGeneration runs image generation and TTS synthesis in parallel using goroutines.
func (r *Runner) runParallelGeneration(ctx context.Context, scenario *domain.ScenarioOutput, project *domain.Project) ([]*domain.Scene, []*domain.Scene, error) {
	r.logger.Info("parallel generation started",
		"scp_id", project.SCPID,
		"scene_count", len(scenario.Scenes))

	var (
		imageScenes []*domain.Scene
		ttsScenes   []*domain.Scene
		imageErr    error
		ttsErr      error
		wg          sync.WaitGroup
	)

	// Image generation goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		imgSvc := service.NewImageGenService(r.imageGen, r.store, r.logger)
		r.wireCharacterToImageSvc(imgSvc)
		imageScenes, imageErr = r.generateShotImages(ctx, scenario, imgSvc, project)
	}()

	// TTS synthesis goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		ttsSvc := service.NewTTSService(r.tts, r.glossary, r.store, r.logger)

		// Wire voice cloner if TTS provider supports it
		if vc, ok := r.tts.(tts.VoiceCloner); ok {
			ttsSvc.SetVoiceCloner(vc)
		}

		// Resolve voice — auto-enroll clone voice if configured
		voice := r.voice
		resolvedVoice, ensureErr := ttsSvc.EnsureVoiceID(ctx, project.ID, voice)
		if ensureErr != nil {
			r.logger.Warn("voice enrollment failed, using default voice",
				"project_id", project.ID, "err", ensureErr)
		} else {
			voice = resolvedVoice
		}

		ttsScenes, ttsErr = ttsSvc.SynthesizeAll(ctx, scenario.Scenes, project.ID, project.WorkspacePath, voice, nil)
	}()

	wg.Wait()

	if imageErr != nil {
		return imageScenes, ttsScenes, &service.PipelineError{
			Stage:      service.StageImageGenerate,
			Cause:      imageErr.Error(),
			RecoverCmd: fmt.Sprintf("yt-pipe image generate %s", project.SCPID),
			Err:        imageErr,
		}
	}
	if ttsErr != nil {
		return imageScenes, ttsScenes, &service.PipelineError{
			Stage:      service.StageTTSSynthesize,
			Cause:      ttsErr.Error(),
			RecoverCmd: fmt.Sprintf("yt-pipe tts generate %s", project.SCPID),
			Err:        ttsErr,
		}
	}

	r.logger.Info("parallel generation complete",
		"images", len(imageScenes), "audio", len(ttsScenes))

	return imageScenes, ttsScenes, nil
}

func (r *Runner) runImageGenerateStage(ctx context.Context, scpID string) error {
	project, err := r.findProject(ctx, scpID)
	if err != nil {
		return err
	}
	scenario, err := service.LoadScenarioFromFile(project.WorkspacePath + "/scenario.json")
	if err != nil {
		return fmt.Errorf("image generate: load scenario: %w", err)
	}

	imgSvc := service.NewImageGenService(r.imageGen, r.store, r.logger)
	r.wireCharacterToImageSvc(imgSvc)

	_, err = r.generateShotImages(ctx, scenario, imgSvc, project)
	return err
}

// RunImageRegenerate regenerates images for specific scenes with backup.
func (r *Runner) RunImageRegenerate(ctx context.Context, scpID string, sceneNums []int) error {
	project, err := r.findProject(ctx, scpID)
	if err != nil {
		return err
	}
	scenario, err := service.LoadScenarioFromFile(project.WorkspacePath + "/scenario.json")
	if err != nil {
		return fmt.Errorf("image regenerate: load scenario: %w", err)
	}

	imgSvc := service.NewImageGenService(r.imageGen, r.store, r.logger)
	r.wireCharacterToImageSvc(imgSvc)

	// Backup existing images before regeneration
	for _, num := range sceneNums {
		service.BackupSceneImage(project.WorkspacePath, num)
	}

	_, err = r.generateShotImages(ctx, scenario, imgSvc, project)
	if err != nil {
		return fmt.Errorf("image regenerate: %w", err)
	}

	r.logger.Info("image regeneration complete",
		"scp_id", scpID,
		"scenes_regenerated", len(sceneNums),
	)
	return nil
}

func (r *Runner) runTTSSynthesizeStage(ctx context.Context, scpID string) error {
	project, err := r.findProject(ctx, scpID)
	if err != nil {
		return err
	}
	scenario, err := service.LoadScenarioFromFile(project.WorkspacePath + "/scenario.json")
	if err != nil {
		return fmt.Errorf("tts synthesize: load scenario: %w", err)
	}
	ttsSvc := service.NewTTSService(r.tts, r.glossary, r.store, r.logger)
	ttsScenes, err := ttsSvc.SynthesizeAll(ctx, scenario.Scenes, project.ID, project.WorkspacePath, r.voice, nil)
	if err != nil {
		return err
	}

	// Generate subtitles from TTS word timings
	subtitleSvc := service.NewSubtitleService(r.glossary, r.store, r.logger)
	if subErr := subtitleSvc.SaveAllSubtitles(ttsScenes, project.ID, project.WorkspacePath, 8, nil); subErr != nil {
		r.logger.Warn("subtitle generation after TTS failed", "err", subErr)
	}

	return nil
}

// RunTTSGenerate runs TTS generation with --force and --scenes flags.
func (r *Runner) RunTTSGenerate(ctx context.Context, scpID string, sceneNums []int, force bool) error {
	project, err := r.findProject(ctx, scpID)
	if err != nil {
		return err
	}
	scenario, err := service.LoadScenarioFromFile(project.WorkspacePath + "/scenario.json")
	if err != nil {
		return fmt.Errorf("tts generate: load scenario: %w", err)
	}

	ttsSvc := service.NewTTSService(r.tts, r.glossary, r.store, r.logger)
	ttsScenes, err := ttsSvc.SynthesizeAllWithOpts(ctx, scenario.Scenes, project.ID, project.WorkspacePath, r.voice, service.SynthesizeAllOpts{
		SceneNums: sceneNums,
		Force:     force,
	})

	// Generate subtitles even if some TTS scenes failed
	if len(ttsScenes) > 0 {
		subtitleSvc := service.NewSubtitleService(r.glossary, r.store, r.logger)
		if subErr := subtitleSvc.SaveAllSubtitles(ttsScenes, project.ID, project.WorkspacePath, 8, nil); subErr != nil {
			r.logger.Warn("subtitle generation after TTS failed", "err", subErr)
		}
	}

	// Display summary
	var totalDuration float64
	for _, s := range ttsScenes {
		totalDuration += s.AudioDuration
	}
	r.logger.Info("tts generation complete",
		"scp_id", scpID,
		"scenes_generated", len(ttsScenes),
		"total_audio_sec", totalDuration,
	)

	return err
}

func (r *Runner) runSubtitleGenerateStage(ctx context.Context, scpID string) error {
	project, err := r.findProject(ctx, scpID)
	if err != nil {
		return err
	}
	scenes, err := loadScenesFromDir(project.WorkspacePath)
	if err != nil {
		return fmt.Errorf("subtitle generate: %w", err)
	}
	subtitleSvc := service.NewSubtitleService(r.glossary, r.store, r.logger)
	return subtitleSvc.SaveAllSubtitles(scenes, project.ID, project.WorkspacePath, 8, nil)
}

func (r *Runner) runAssembleStage(ctx context.Context, scpID string) error {
	project, err := r.findProject(ctx, scpID)
	if err != nil {
		return err
	}
	scenes, err := loadScenesFromDir(project.WorkspacePath)
	if err != nil {
		return fmt.Errorf("assemble: %w", err)
	}
	projectSvc := service.NewProjectService(r.store)
	assemblerSvc := service.NewAssemblerService(r.assembler, projectSvc)
	assemblerSvc.WithConfig(r.templatePath, r.metaPath, r.canvas)
	domainScenes := toDomainScenes(scenes)
	_, err = assemblerSvc.Assemble(ctx, project.ID, domainScenes)
	if err != nil {
		return err
	}

	// Generate copyright notice and check special conditions
	r.generateCopyright(project, assemblerSvc)

	return nil
}

// generateCopyright generates copyright notice and checks for special conditions.
func (r *Runner) generateCopyright(project *domain.Project, assemblerSvc *service.AssemblerService) {
	scpData, err := workspace.LoadSCPData(r.scpDataPath, project.SCPID)
	if err != nil {
		r.logger.Warn("copyright: could not load SCP data for copyright", "error", err)
		// Generate with unknown author
		if err := assemblerSvc.GenerateCopyrightNotice(project.WorkspacePath, project.SCPID, ""); err != nil {
			r.logger.Error("copyright: failed to generate notice", "error", err)
		}
		return
	}

	author := ""
	if scpData.Meta != nil {
		author = scpData.Meta.Author
	}
	if err := assemblerSvc.GenerateCopyrightNotice(project.WorkspacePath, project.SCPID, author); err != nil {
		r.logger.Error("copyright: failed to generate notice", "error", err)
	}
	if scpData.Meta != nil {
		if err := service.LogSpecialCopyright(project.WorkspacePath, project.SCPID, scpData.Meta); err != nil {
			r.logger.Error("copyright: failed to log special conditions", "error", err)
		}
	}
}

// workspaceHasImages checks if image assets exist for at least one scene in the workspace.
func (r *Runner) workspaceHasImages(projectPath string, sceneCount int) bool {
	for i := 1; i <= sceneCount; i++ {
		sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", i))
		// Check for shot-based images (shot_1.png, etc.) or legacy single image (image.png)
		for _, pattern := range []string{"shot_1.png", "shot_1.jpg", "shot_1.webp", "image.png"} {
			if _, err := os.Stat(filepath.Join(sceneDir, pattern)); err == nil {
				return true
			}
		}
	}
	return false
}

// workspaceHasTTS checks if TTS audio assets exist for at least one scene in the workspace.
func (r *Runner) workspaceHasTTS(projectPath string, sceneCount int) bool {
	for i := 1; i <= sceneCount; i++ {
		audioPath := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", i), "audio.mp3")
		if _, err := os.Stat(audioPath); err == nil {
			return true
		}
	}
	return false
}

func (r *Runner) findProject(ctx context.Context, scpID string) (*domain.Project, error) {
	projects, err := r.store.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("pipeline: list projects: %w", err)
	}
	// Find the most recent project for this SCP ID
	for _, p := range projects {
		if p.SCPID == scpID {
			return p, nil
		}
	}
	return nil, &domain.NotFoundError{Resource: "project", ID: scpID}
}

func (r *Runner) reportProgress(p service.PipelineProgress) {
	if r.ProgressFunc != nil {
		p.ElapsedSec = time.Since(p.StartedAt).Seconds()
		r.ProgressFunc(p)
	}
}

func (r *Runner) pipelineError(stage service.PipelineStage, sceneNum int, err error, scpID string) *service.PipelineError {
	recoverCmd := fmt.Sprintf("yt-pipe run %s", scpID)
	return &service.PipelineError{
		Stage:      stage,
		SceneNum:   sceneNum,
		Cause:      err.Error(),
		RecoverCmd: recoverCmd,
		Err:        err,
	}
}

func stageResult(name string, start time.Time, err error) StageResult {
	sr := StageResult{
		Name:       name,
		DurationMs: time.Since(start).Milliseconds(),
	}
	if err != nil {
		sr.Status = "fail"
		sr.Error = err.Error()
	} else {
		sr.Status = "pass"
	}
	return sr
}

// mergeSceneData combines image and TTS scene data with timing info.
func mergeSceneData(imageScenes, ttsScenes []*domain.Scene, timings []service.SceneTiming) []*domain.Scene {
	byNum := make(map[int]*domain.Scene)
	for _, s := range imageScenes {
		byNum[s.SceneNum] = &domain.Scene{
			SceneNum:  s.SceneNum,
			ImagePath: s.ImagePath,
			Shots:     s.Shots,
		}
	}
	for _, s := range ttsScenes {
		if merged, ok := byNum[s.SceneNum]; ok {
			merged.Narration = s.Narration
			merged.AudioPath = s.AudioPath
			merged.AudioDuration = s.AudioDuration
			merged.WordTimings = s.WordTimings
		} else {
			byNum[s.SceneNum] = s
		}
	}
	// Apply timing offsets to word timings
	for _, t := range timings {
		if merged, ok := byNum[t.SceneNum]; ok {
			if len(t.WordTimings) > 0 {
				merged.WordTimings = toDomainWordTimings(t.WordTimings)
			}
		}
	}

	scenes := make([]*domain.Scene, 0, len(byNum))
	for _, s := range byNum {
		scenes = append(scenes, s)
	}
	sort.Slice(scenes, func(i, j int) bool {
		return scenes[i].SceneNum < scenes[j].SceneNum
	})

	// Resolve shot timings from WordTimings
	for _, scene := range scenes {
		if len(scene.Shots) > 0 && len(scene.WordTimings) > 0 {
			resolveShotTimings(scene)
		}
	}

	return scenes
}

// resolveShotTimings maps sentence-based shots to WordTiming timestamps.
func resolveShotTimings(scene *domain.Scene) {
	sentences := domain.SplitNarrationSentences(scene.Narration)
	if len(sentences) != len(scene.Shots) {
		// Mismatch — assign equal duration fallback
		if len(scene.Shots) > 0 && scene.AudioDuration > 0 {
			equalDur := scene.AudioDuration / float64(len(scene.Shots))
			for i := range scene.Shots {
				scene.Shots[i].StartSec = float64(i) * equalDur
				scene.Shots[i].EndSec = float64(i+1) * equalDur
			}
		}
		return
	}

	// Build sentence → time range mapping from WordTimings
	sentenceTimings := mapSentencesToTimings(sentences, scene.WordTimings)
	for i := range scene.Shots {
		if i < len(sentenceTimings) {
			scene.Shots[i].StartSec = sentenceTimings[i].Start
			scene.Shots[i].EndSec = sentenceTimings[i].End
		}
	}
}

type sentenceTiming struct {
	Start float64
	End   float64
}

// mapSentencesToTimings accumulates word timings to find sentence boundaries.
func mapSentencesToTimings(sentences []string, wordTimings []domain.WordTiming) []sentenceTiming {
	if len(wordTimings) == 0 {
		return make([]sentenceTiming, len(sentences))
	}

	timings := make([]sentenceTiming, len(sentences))
	wordIdx := 0

	for si, sentence := range sentences {
		if wordIdx < len(wordTimings) {
			timings[si].Start = wordTimings[wordIdx].StartSec
		}

		sentenceWords := countKoreanWords(sentence)
		endWordIdx := wordIdx + sentenceWords - 1
		if endWordIdx >= len(wordTimings) {
			endWordIdx = len(wordTimings) - 1
		}
		if endWordIdx >= 0 {
			timings[si].End = wordTimings[endWordIdx].EndSec
		}
		wordIdx = endWordIdx + 1
	}

	// Ensure last sentence extends to audio end
	if len(timings) > 0 && len(wordTimings) > 0 {
		timings[len(timings)-1].End = wordTimings[len(wordTimings)-1].EndSec
	}

	return timings
}

// countKoreanWords counts words in a Korean/mixed sentence by splitting on whitespace.
func countKoreanWords(sentence string) int {
	words := strings.Fields(sentence)
	return len(words)
}

func toDomainWordTimings(timings []domain.WordTiming) []domain.WordTiming {
	return timings
}

func toDomainScenes(scenes []*domain.Scene) []domain.Scene {
	result := make([]domain.Scene, len(scenes))
	for i, s := range scenes {
		result[i] = *s
	}
	return result
}

// loadScenesFromDir loads scene data from workspace scene directories.
func loadScenesFromDir(projectPath string) ([]*domain.Scene, error) {
	// Delegate to the workspace package for reading scene manifests
	scenesDir := projectPath + "/scenes"
	entries, err := os.ReadDir(scenesDir)
	if err != nil {
		return nil, fmt.Errorf("read scenes directory: %w", err)
	}

	var scenes []*domain.Scene
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestData, err := workspace.ReadFile(scenesDir + "/" + entry.Name() + "/manifest.json")
		if err != nil {
			continue
		}
		scene, err := parseSceneManifest(manifestData)
		if err != nil {
			continue
		}
		scenes = append(scenes, scene)
	}
	return scenes, nil
}

// generateShotImages runs the cut decomposition pipeline and generates images per cut.
func (r *Runner) generateShotImages(ctx context.Context, scenario *domain.ScenarioOutput, imgSvc *service.ImageGenService, project *domain.Project) ([]*domain.Scene, error) {
	cutPipeline, err := service.NewShotBreakdownPipeline(r.llm, service.ShotBreakdownConfig{
		TemplatesDir: r.templatesPath,
	})
	if err != nil {
		return nil, &service.PipelineError{
			Stage:      service.StageImageGenerate,
			Cause:      fmt.Sprintf("init cut pipeline: %s", err),
			RecoverCmd: fmt.Sprintf("yt-pipe run %s", project.SCPID),
			Err:        err,
		}
	}

	frozenDesc := ""
	visualIdentity := ""
	styleGuide := ""
	if r.characterSvc != nil && project.SCPID != "" {
		char, _ := r.characterSvc.CheckExistingCharacter(project.SCPID)
		if char != nil {
			frozenDesc = char.VisualDescriptor
			visualIdentity = char.ImagePromptBase
			styleGuide = char.StyleGuide
		}
	}

	// Build SceneCutInput per scene with visual metadata + merged style
	inputs := make([]service.SceneCutInput, 0, len(scenario.Scenes))
	for _, scene := range scenario.Scenes {
		mergedStyle := domain.MergeSceneStyle(r.styleConfig, domain.SceneVisualMeta{
			Location:          scene.Location,
			CharactersPresent: scene.CharactersPresent,
			ColorPalette:      scene.ColorPalette,
			Atmosphere:        scene.Atmosphere,
		})

		inputs = append(inputs, service.SceneCutInput{
			SceneNum:             scene.SceneNum,
			Narration:            scene.Narration,
			Mood:                 scene.Mood,
			Location:             scene.Location,
			CharactersPresent:    scene.CharactersPresent,
			ColorPalette:         mergedStyle.ColorPalette,
			Atmosphere:           mergedStyle.Mood,
			EntityVisualIdentity: visualIdentity,
			FrozenDescriptor:     frozenDesc,
			StyleGuide:           styleGuide,
			StyleConfig:          mergedStyle,
		})
	}

	sceneCuts, err := cutPipeline.GenerateAllSceneCuts(ctx, inputs)
	if err != nil {
		return nil, &service.PipelineError{
			Stage:      service.StageImageGenerate,
			Cause:      fmt.Sprintf("cut decomposition: %s", err),
			RecoverCmd: fmt.Sprintf("yt-pipe run %s", project.SCPID),
			Err:        err,
		}
	}

	// Incremental: skip unchanged cuts
	skipChecker := NewSceneSkipChecker(r.store, r.logger)
	_, toSkip := skipChecker.FilterCutsForImageGen(project.ID, sceneCuts)
	skipMap := make(map[domain.ShotKey]bool, len(toSkip))
	for _, k := range toSkip {
		skipMap[k] = true
	}

	return imgSvc.GenerateAllCutImages(ctx, sceneCuts, project.ID, project.WorkspacePath, project.SCPID, r.imageOpts, skipMap)
}

// wireCharacterToImageSvc wires the character service and selected image into an ImageGenService.
func (r *Runner) wireCharacterToImageSvc(imgSvc *service.ImageGenService) {
	if r.characterSvc != nil {
		imgSvc.SetCharacterService(r.characterSvc)
		if r.selectedCharacterImagePath != "" {
			_ = imgSvc.SetSelectedCharacterImage(r.selectedCharacterImagePath)
		}
	}
}

func parseSceneManifest(data []byte) (*domain.Scene, error) {
	var m struct {
		SceneNum      int                 `json:"scene_num"`
		Narration     string              `json:"narration"`
		ImagePath     string              `json:"image_path"`
		AudioPath     string              `json:"audio_path"`
		AudioDuration float64             `json:"audio_duration"`
		SubtitlePath  string              `json:"subtitle_path"`
		WordTimings   []domain.WordTiming `json:"word_timings"`
		Shots         []domain.Shot       `json:"shots,omitempty"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &domain.Scene{
		SceneNum:      m.SceneNum,
		Narration:     m.Narration,
		ImagePath:     m.ImagePath,
		AudioPath:     m.AudioPath,
		AudioDuration: m.AudioDuration,
		SubtitlePath:  m.SubtitlePath,
		WordTimings:   m.WordTimings,
		Shots:         m.Shots,
	}, nil
}
