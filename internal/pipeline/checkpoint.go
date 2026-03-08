package pipeline

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/sushistack/yt.pipe/internal/service"
)

// CheckpointManager handles saving and loading pipeline checkpoints.
type CheckpointManager struct {
	logger *slog.Logger
}

// NewCheckpointManager creates a new CheckpointManager.
func NewCheckpointManager(logger *slog.Logger) *CheckpointManager {
	return &CheckpointManager{logger: logger}
}

// SaveStageCheckpoint records completion of a pipeline stage and persists the checkpoint.
func (cm *CheckpointManager) SaveStageCheckpoint(projectPath, projectID string, stage service.PipelineStage, scenesDone int) error {
	cp, err := service.LoadCheckpoint(projectPath)
	if err != nil {
		// No existing checkpoint — create new
		cp = &service.PipelineCheckpoint{
			ProjectID: projectID,
		}
	}

	cp.RecordStage(stage, scenesDone)

	if err := service.SaveCheckpoint(projectPath, cp); err != nil {
		return fmt.Errorf("checkpoint: save after %s: %w", stage, err)
	}

	cm.logger.Info("checkpoint saved",
		"project_id", projectID,
		"stage", stage,
		"scenes_done", scenesDone)

	return nil
}

// LoadCheckpoint loads the pipeline checkpoint for a project.
// Returns nil if no checkpoint exists.
func (cm *CheckpointManager) LoadCheckpoint(projectPath string) *service.PipelineCheckpoint {
	cp, err := service.LoadCheckpoint(projectPath)
	if err != nil {
		return nil
	}
	return cp
}

// GetResumeStage determines which stage to resume from based on the checkpoint.
// Returns the next stage after the last completed stage.
func (cm *CheckpointManager) GetResumeStage(cp *service.PipelineCheckpoint) service.PipelineStage {
	if cp == nil || len(cp.Stages) == 0 {
		return service.StageDataLoad
	}

	stageOrder := []service.PipelineStage{
		service.StageDataLoad,
		service.StageScenarioGenerate,
		service.StageScenarioApproval,
		service.StageImageGenerate,
		service.StageTTSSynthesize,
		service.StageTimingResolve,
		service.StageSubtitleGenerate,
		service.StageAssemble,
	}

	lastCompleted := cp.LastStage
	for i, stage := range stageOrder {
		if stage == lastCompleted && i+1 < len(stageOrder) {
			return stageOrder[i+1]
		}
	}

	return service.StageAssemble
}

// ShouldSkipStage checks if a stage has already been completed according to the checkpoint.
func (cm *CheckpointManager) ShouldSkipStage(cp *service.PipelineCheckpoint, stage service.PipelineStage) bool {
	if cp == nil {
		return false
	}
	return cp.HasCompletedStage(stage)
}

// BuildRecoveryCommand creates a CLI recovery command for a failed stage.
func BuildRecoveryCommand(scpID string, stage service.PipelineStage, sceneNum int) string {
	switch stage {
	case service.StageDataLoad, service.StageScenarioGenerate:
		return fmt.Sprintf("yt-pipe run %s", scpID)
	case service.StageImageGenerate:
		if sceneNum > 0 {
			return fmt.Sprintf("yt-pipe image generate %s --scene %d", scpID, sceneNum)
		}
		return fmt.Sprintf("yt-pipe image generate %s", scpID)
	case service.StageTTSSynthesize:
		if sceneNum > 0 {
			return fmt.Sprintf("yt-pipe tts generate %s --scene %d", scpID, sceneNum)
		}
		return fmt.Sprintf("yt-pipe tts generate %s", scpID)
	case service.StageSubtitleGenerate:
		return fmt.Sprintf("yt-pipe subtitle generate %s", scpID)
	case service.StageAssemble:
		return fmt.Sprintf("yt-pipe assemble %s", scpID)
	default:
		return fmt.Sprintf("yt-pipe run %s", scpID)
	}
}

// CheckProjectIntegrity verifies that project files are intact after abnormal termination.
func CheckProjectIntegrity(projectPath string) error {
	// Check essential files exist
	essentialFiles := []string{
		"scenario.json",
	}

	for _, f := range essentialFiles {
		path := projectPath + "/" + f
		info, err := os.Stat(path)
		if err != nil {
			continue // File may not exist yet if early stage
		}
		if info.Size() == 0 {
			return fmt.Errorf("integrity check: %s is empty (possibly corrupted)", f)
		}
	}

	return nil
}
