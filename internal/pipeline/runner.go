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
	AutoApprove bool
	Force       bool // Clear checkpoints and start from scratch
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

	r.logger.Info("pipeline started", "scp_id", scpID, "auto_approve", opts.AutoApprove, "force", opts.Force)

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

		// Determine where to resume
		if checkpoint.HasCompletedStage(service.StageScenarioGenerate) &&
			(existingProject.Status == domain.StatusApproved || existingProject.Status == domain.StatusGeneratingAssets) {
			scenario, err := service.LoadScenarioFromFile(existingProject.WorkspacePath + "/scenario.json")
			if err != nil {
				return nil, fmt.Errorf("pipeline: load scenario for resume: %w", err)
			}
			resumeResult, err := r.resumeFromApproval(ctx, existingProject, scenario, start)
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
		resumeResult, err := r.resumeFromApproval(ctx, project, scenario, start)
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

// Resume continues the pipeline after scenario approval.
// It expects the project to be in "approved" state.
func (r *Runner) Resume(ctx context.Context, projectID string) (*RunResult, error) {
	start := time.Now()

	projectSvc := service.NewProjectService(r.store)
	project, err := projectSvc.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pipeline: get project: %w", err)
	}

	if project.Status != domain.StatusApproved {
		return nil, fmt.Errorf("pipeline: project %s is in %q state, expected %q. Approve with: yt-pipe scenario approve %s",
			projectID, project.Status, domain.StatusApproved, project.SCPID)
	}

	// Load scenario from workspace
	scenario, err := service.LoadScenarioFromFile(project.WorkspacePath + "/scenario.json")
	if err != nil {
		return nil, fmt.Errorf("pipeline: load scenario: %w", err)
	}

	result, err := r.resumeFromApproval(ctx, project, scenario, start)
	if err != nil {
		return result, err
	}
	result.APICalls = countAPICalls(result)
	result.EstimatedCost = estimateCost(result)
	return result, nil
}

// resumeFromApproval runs all post-approval stages. Shared by Resume() and RunWithOptions() auto-approve.
func (r *Runner) resumeFromApproval(ctx context.Context, project *domain.Project, scenario *domain.ScenarioOutput, start time.Time) (*RunResult, error) {
	result := &RunResult{
		ProjectID:  project.ID,
		SCPID:      project.SCPID,
		SceneCount: project.SceneCount,
		Stages:     make([]StageResult, 0, 6),
	}

	projectSvc := service.NewProjectService(r.store)

	// Transition to generating_assets
	if _, err := projectSvc.TransitionProject(ctx, project.ID, domain.StatusGeneratingAssets); err != nil {
		return nil, fmt.Errorf("pipeline: transition to generating_assets: %w", err)
	}

	r.logger.Info("pipeline resumed", "project_id", project.ID, "scp_id", project.SCPID)

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
	// Update subtitle paths on merged scenes
	for _, s := range mergedScenes {
		if s.SubtitlePath == "" {
			s.SubtitlePath = fmt.Sprintf("%s/scenes/%d/subtitle.json", project.WorkspacePath, s.SceneNum)
		}
	}
	result.Stages = append(result.Stages, stageResult(string(service.StageSubtitleGenerate), stageStart, nil))

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
	scenarioSvc := service.NewScenarioService(r.store, r.llm, projectSvc)

	projectPath, err := workspace.InitProject(r.workspacePath, scpData.SCPID)
	if err != nil {
		return nil, nil, fmt.Errorf("scenario generate: init workspace: %w", err)
	}

	scenario, project, err := scenarioSvc.GenerateScenario(ctx, scpData, projectPath)
	if err != nil {
		return nil, project, fmt.Errorf("scenario generate: %w", err)
	}

	r.logger.Info("stage complete", "stage", service.StageScenarioGenerate,
		"scp_id", scpData.SCPID, "scenes", len(scenario.Scenes),
		"duration_ms", time.Since(start).Milliseconds())

	return scenario, project, nil
}

// runParallelGeneration runs image generation and TTS synthesis in parallel using goroutines.
func (r *Runner) runParallelGeneration(ctx context.Context, scenario *domain.ScenarioOutput, project *domain.Project) ([]*domain.Scene, []*domain.Scene, error) {
	r.logger.Info("parallel generation started",
		"scp_id", project.SCPID,
		"scene_count", len(scenario.Scenes))

	// Generate image prompts first
	prompts, err := service.GenerateImagePrompts(scenario, nil)
	if err != nil {
		return nil, nil, &service.PipelineError{
			Stage:      service.StageImageGenerate,
			Cause:      fmt.Sprintf("generate image prompts: %s", err),
			RecoverCmd: fmt.Sprintf("yt-pipe run %s", project.SCPID),
			Err:        err,
		}
	}

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
		imageScenes, imageErr = imgSvc.GenerateAllImages(ctx, prompts, project.ID, project.WorkspacePath, r.imageOpts, nil)
	}()

	// TTS synthesis goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		ttsSvc := service.NewTTSService(r.tts, r.glossary, r.store, r.logger)
		ttsScenes, ttsErr = ttsSvc.SynthesizeAll(ctx, scenario.Scenes, project.ID, project.WorkspacePath, r.voice, nil)
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
	prompts, err := service.GenerateImagePrompts(scenario, nil)
	if err != nil {
		return fmt.Errorf("image generate: prompts: %w", err)
	}

	imgSvc := service.NewImageGenService(r.imageGen, r.store, r.logger)

	total := len(prompts)
	for i, p := range prompts {
		r.logger.Info("image generation progress",
			"scene", fmt.Sprintf("%d/%d", i+1, total),
			"scene_num", p.SceneNum,
			"status", "generating",
		)
		_, err := imgSvc.GenerateSceneImage(ctx, p, project.ID, project.WorkspacePath, r.imageOpts)
		if err != nil {
			r.logger.Error("scene image failed", "scene_num", p.SceneNum, "err", err)
			continue
		}
	}
	return nil
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
	prompts, err := service.GenerateImagePrompts(scenario, nil)
	if err != nil {
		return fmt.Errorf("image regenerate: prompts: %w", err)
	}

	imgSvc := service.NewImageGenService(r.imageGen, r.store, r.logger)

	// Backup existing images before regeneration
	for _, num := range sceneNums {
		service.BackupSceneImage(project.WorkspacePath, num)
	}

	_, err = imgSvc.GenerateAllImages(ctx, prompts, project.ID, project.WorkspacePath, r.imageOpts, sceneNums)
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
	return scenes
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

func parseSceneManifest(data []byte) (*domain.Scene, error) {
	var m struct {
		SceneNum      int                 `json:"scene_num"`
		Narration     string              `json:"narration"`
		ImagePath     string              `json:"image_path"`
		AudioPath     string              `json:"audio_path"`
		AudioDuration float64             `json:"audio_duration"`
		SubtitlePath  string              `json:"subtitle_path"`
		WordTimings   []domain.WordTiming `json:"word_timings"`
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
	}, nil
}
