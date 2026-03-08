package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/jay/youtube-pipeline/internal/workspace"
)

// PipelineStage represents a named stage in the pipeline.
type PipelineStage string

const (
	StageDataLoad          PipelineStage = "data_load"
	StageScenarioGenerate  PipelineStage = "scenario_generate"
	StageScenarioApproval  PipelineStage = "scenario_approval"
	StageImageGenerate     PipelineStage = "image_generate"
	StageTTSSynthesize     PipelineStage = "tts_synthesize"
	StageTimingResolve     PipelineStage = "timing_resolve"
	StageSubtitleGenerate  PipelineStage = "subtitle_generate"
	StageAssemble          PipelineStage = "assemble"
)

// PipelineProgress tracks current pipeline execution state.
type PipelineProgress struct {
	Stage          PipelineStage `json:"stage"`
	ScenesTotal    int           `json:"scenes_total"`
	ScenesComplete int           `json:"scenes_complete"`
	ProgressPct    float64       `json:"progress_pct"`
	ElapsedSec     float64       `json:"elapsed_sec"`
	StartedAt      time.Time     `json:"started_at"`
}

// StageCheckpoint records completion of a pipeline stage for resume support.
type StageCheckpoint struct {
	Stage       PipelineStage `json:"stage"`
	CompletedAt time.Time     `json:"completed_at"`
	ScenesDone  int           `json:"scenes_done"`
}

// PipelineCheckpoint holds the full checkpoint state for a project.
type PipelineCheckpoint struct {
	ProjectID  string            `json:"project_id"`
	Stages     []StageCheckpoint `json:"stages"`
	LastStage  PipelineStage     `json:"last_stage"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// SaveCheckpoint persists the pipeline checkpoint to the project workspace.
func SaveCheckpoint(projectPath string, cp *PipelineCheckpoint) error {
	cp.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("checkpoint: marshal: %w", err)
	}
	return workspace.WriteFileAtomic(filepath.Join(projectPath, "checkpoint.json"), data)
}

// LoadCheckpoint loads a pipeline checkpoint from the project workspace.
func LoadCheckpoint(projectPath string) (*PipelineCheckpoint, error) {
	data, err := workspace.ReadFile(filepath.Join(projectPath, "checkpoint.json"))
	if err != nil {
		return nil, err
	}
	var cp PipelineCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("checkpoint: unmarshal: %w", err)
	}
	return &cp, nil
}

// HasCompletedStage checks if a checkpoint has a specific stage completed.
func (cp *PipelineCheckpoint) HasCompletedStage(stage PipelineStage) bool {
	for _, s := range cp.Stages {
		if s.Stage == stage {
			return true
		}
	}
	return false
}

// RecordStage adds a completed stage to the checkpoint.
func (cp *PipelineCheckpoint) RecordStage(stage PipelineStage, scenesDone int) {
	cp.Stages = append(cp.Stages, StageCheckpoint{
		Stage:       stage,
		CompletedAt: time.Now().UTC(),
		ScenesDone:  scenesDone,
	})
	cp.LastStage = stage
}

// PipelineError provides detailed error information with recovery instructions.
type PipelineError struct {
	Stage      PipelineStage `json:"stage"`
	SceneNum   int           `json:"scene_num,omitempty"`
	Cause      string        `json:"cause"`
	RecoverCmd string        `json:"recover_cmd"`
	Err        error         `json:"-"`
}

func (e *PipelineError) Error() string {
	if e.SceneNum > 0 {
		return fmt.Sprintf("pipeline failed at %s (scene %d): %s\nRecover with: %s", e.Stage, e.SceneNum, e.Cause, e.RecoverCmd)
	}
	return fmt.Sprintf("pipeline failed at %s: %s\nRecover with: %s", e.Stage, e.Cause, e.RecoverCmd)
}

func (e *PipelineError) Unwrap() error {
	return e.Err
}

// ContentHash computes a SHA-256 hash of content for change detection.
func ContentHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SceneManifestEntry tracks asset hashes for incremental builds.
type SceneManifestEntry struct {
	SceneNum     int    `json:"scene_num"`
	NarrationHash string `json:"narration_hash"`
	PromptHash    string `json:"prompt_hash"`
	ImageHash     string `json:"image_hash"`
	AudioHash     string `json:"audio_hash"`
	SubtitleHash  string `json:"subtitle_hash"`
	Status        string `json:"status"` // "current", "stale"
}

// SceneManifest tracks all scene assets for incremental builds.
type SceneManifest struct {
	ProjectID string               `json:"project_id"`
	Entries   []SceneManifestEntry `json:"entries"`
	UpdatedAt time.Time            `json:"updated_at"`
}

// SaveManifest persists the scene manifest to the project workspace.
func SaveManifest(projectPath string, m *SceneManifest) error {
	m.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("manifest: marshal: %w", err)
	}
	return workspace.WriteFileAtomic(filepath.Join(projectPath, "manifest.json"), data)
}

// LoadManifest loads a scene manifest from the project workspace.
func LoadManifest(projectPath string) (*SceneManifest, error) {
	data, err := workspace.ReadFile(filepath.Join(projectPath, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m SceneManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: unmarshal: %w", err)
	}
	return &m, nil
}

// NeedsRegeneration checks if a scene's asset needs to be regenerated.
func (m *SceneManifest) NeedsRegeneration(sceneNum int, assetType, currentHash string) bool {
	for _, e := range m.Entries {
		if e.SceneNum == sceneNum {
			switch assetType {
			case "prompt":
				return e.PromptHash != currentHash
			case "image":
				return e.ImageHash != currentHash
			case "audio":
				return e.AudioHash != currentHash
			case "subtitle":
				return e.SubtitleHash != currentHash
			case "narration":
				return e.NarrationHash != currentHash
			}
		}
	}
	return true // not found = needs generation
}

// InvalidateDownstream marks downstream assets as stale when upstream changes.
func (m *SceneManifest) InvalidateDownstream(sceneNum int, changedAsset string) {
	for i, e := range m.Entries {
		if e.SceneNum != sceneNum {
			continue
		}

		switch changedAsset {
		case "narration":
			// Narration change invalidates: prompt, image, audio, subtitle
			m.Entries[i].PromptHash = ""
			m.Entries[i].ImageHash = ""
			m.Entries[i].AudioHash = ""
			m.Entries[i].SubtitleHash = ""
			m.Entries[i].Status = "stale"
		case "prompt":
			// Prompt change invalidates: image only
			m.Entries[i].ImageHash = ""
			m.Entries[i].Status = "stale"
		case "audio":
			// Audio change invalidates: subtitle (timing dependent)
			m.Entries[i].SubtitleHash = ""
			m.Entries[i].Status = "stale"
		}

		slog.Info("dependency invalidation",
			"scene", sceneNum,
			"changed", changedAsset,
			"status", m.Entries[i].Status)
	}
}
