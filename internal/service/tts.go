package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/plugin/tts"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// TTSService handles TTS narration synthesis for scenes.
type TTSService struct {
	tts      tts.TTS
	glossary *glossary.Glossary
	store    *store.Store
	logger   *slog.Logger
}

// NewTTSService creates a new TTSService.
func NewTTSService(t tts.TTS, g *glossary.Glossary, s *store.Store, logger *slog.Logger) *TTSService {
	return &TTSService{tts: t, glossary: g, store: s, logger: logger}
}

// SynthesizeScene synthesizes narration for a single scene and saves audio to the scene directory.
// The TTS plugin handles its own retry logic, so the service layer does not add additional retries.
func (s *TTSService) SynthesizeScene(ctx context.Context, scene domain.SceneScript, projectID, projectPath, voice string) (*domain.Scene, error) {
	overrides := s.buildOverrides()

	// Look up confirmed mood assignment for this scene
	opts := s.buildTTSOptions(projectID, scene.SceneNum)

	var result *tts.SynthesisResult
	var err error
	start := time.Now()

	if len(overrides) > 0 {
		result, err = s.tts.SynthesizeWithOverrides(ctx, scene.Narration, voice, overrides, opts)
	} else {
		result, err = s.tts.Synthesize(ctx, scene.Narration, voice, opts)
	}
	if err != nil {
		s.logger.Error("tts synthesis failed",
			"project_id", projectID,
			"scene_num", scene.SceneNum,
			"err", err,
		)
		s.markSceneTTSFailed(projectID, scene.SceneNum)
		return nil, fmt.Errorf("tts: scene %d: %w", scene.SceneNum, err)
	}

	elapsed := time.Since(start).Milliseconds()

	// Save audio to scene directory
	sceneDir, err := workspace.InitSceneDir(projectPath, scene.SceneNum)
	if err != nil {
		return nil, fmt.Errorf("tts: init scene dir %d: %w", scene.SceneNum, err)
	}

	audioPath := filepath.Join(sceneDir, "audio.mp3")

	// Backup existing audio file before overwriting (AC3)
	backupAudioFile(audioPath)

	if err := workspace.WriteFileAtomic(audioPath, result.AudioData); err != nil {
		return nil, fmt.Errorf("tts: save audio %d: %w", scene.SceneNum, err)
	}

	// Save word timings as timing.json
	if len(result.WordTimings) > 0 {
		timingPath := filepath.Join(sceneDir, "timing.json")
		timingData, err := json.MarshalIndent(result.WordTimings, "", "  ")
		if err == nil {
			if writeErr := workspace.WriteFileAtomic(timingPath, timingData); writeErr != nil {
				s.logger.Warn("failed to save timing.json", "scene_num", scene.SceneNum, "err", writeErr)
			}
		}
	}

	// Update scene manifest with audio hash + duration (AC4)
	audioHash := hashBytes(result.AudioData)
	s.updateManifestAudioHash(projectID, scene.SceneNum, audioHash)

	s.logger.Info("scene tts synthesized",
		"project_id", projectID,
		"scene_num", scene.SceneNum,
		"duration_sec", result.DurationSec,
		"duration_ms", elapsed,
		"audio_hash", audioHash[:16],
		"word_count", len(result.WordTimings),
	)

	return &domain.Scene{
		SceneNum:      scene.SceneNum,
		Narration:     scene.Narration,
		AudioPath:     audioPath,
		AudioDuration: result.DurationSec,
		WordTimings:   result.WordTimings,
	}, nil
}

// ShouldSkipScene checks if a scene already has audio generated.
func (s *TTSService) ShouldSkipScene(projectID string, sceneNum int) bool {
	manifest, err := s.store.GetManifest(projectID, sceneNum)
	if err != nil {
		return false
	}
	return manifest.Status == "audio_generated" && manifest.AudioHash != ""
}

// SynthesizeAllOpts configures the SynthesizeAll behavior.
type SynthesizeAllOpts struct {
	SceneNums []int // Selective scene regeneration; empty = all scenes
	Force     bool  // Bypass skip logic
}

// SynthesizeAll synthesizes narration for all or selected scenes.
// If sceneNums is non-empty, only those scenes are synthesized (AC3: selective re-synthesis).
// On partial failure, continues processing remaining scenes.
func (s *TTSService) SynthesizeAll(ctx context.Context, scenes []domain.SceneScript, projectID, projectPath, voice string, sceneNums []int) ([]*domain.Scene, error) {
	return s.SynthesizeAllWithOpts(ctx, scenes, projectID, projectPath, voice, SynthesizeAllOpts{SceneNums: sceneNums})
}

// SynthesizeAllWithOpts synthesizes narration with extended options including skip logic.
func (s *TTSService) SynthesizeAllWithOpts(ctx context.Context, scenes []domain.SceneScript, projectID, projectPath, voice string, opts SynthesizeAllOpts) ([]*domain.Scene, error) {
	filtered := filterSceneScripts(scenes, opts.SceneNums)
	total := len(filtered)

	results := make([]*domain.Scene, 0, total)
	var errs []error
	var totalDuration float64

	for i, scene := range filtered {
		if err := ctx.Err(); err != nil {
			s.logger.Warn("tts synthesis cancelled", "project_id", projectID, "remaining", total-len(results)-len(errs))
			errs = append(errs, fmt.Errorf("tts: cancelled: %w", err))
			break
		}

		// Skip logic: check if scene already has audio (unless --force)
		if !opts.Force && s.ShouldSkipScene(projectID, scene.SceneNum) {
			s.logger.Info("scene tts skipped (already generated)",
				"project_id", projectID,
				"scene_num", scene.SceneNum,
				"progress", fmt.Sprintf("%d/%d", i+1, total),
			)
			continue
		}

		s.logger.Info("scene tts progress",
			"project_id", projectID,
			"scene_num", scene.SceneNum,
			"progress", fmt.Sprintf("%d/%d", i+1, total),
			"status", "synthesizing",
		)

		result, err := s.SynthesizeScene(ctx, scene, projectID, projectPath, voice)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		results = append(results, result)
		totalDuration += result.AudioDuration
	}

	s.logger.Info("tts synthesis batch complete",
		"project_id", projectID,
		"synthesized", len(results),
		"failed", len(errs),
		"total_duration_sec", totalDuration,
	)

	if len(errs) > 0 {
		return results, fmt.Errorf("tts: %d/%d scenes failed: %w", len(errs), total, errors.Join(errs...))
	}
	return results, nil
}

// buildTTSOptions looks up a confirmed mood preset for the scene and returns TTSOptions.
func (s *TTSService) buildTTSOptions(projectID string, sceneNum int) *tts.TTSOptions {
	assignment, err := s.store.GetSceneMoodAssignment(projectID, sceneNum)
	if err != nil || !assignment.Confirmed {
		return nil
	}
	preset, err := s.store.GetMoodPreset(assignment.PresetID)
	if err != nil {
		s.logger.Warn("mood preset not found for scene assignment",
			"project_id", projectID,
			"scene_num", sceneNum,
			"preset_id", assignment.PresetID,
		)
		return nil
	}
	return &tts.TTSOptions{
		MoodPreset: &tts.MoodPreset{
			Speed:   preset.Speed,
			Emotion: preset.Emotion,
			Pitch:   preset.Pitch,
			Params:  preset.ParamsJSON,
		},
	}
}

func (s *TTSService) buildOverrides() map[string]string {
	if s.glossary == nil {
		return nil
	}
	entries := s.glossary.Entries()
	if len(entries) == 0 {
		return nil
	}
	overrides := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.Pronunciation != "" {
			overrides[e.Term] = e.Pronunciation
		}
	}
	if len(overrides) == 0 {
		return nil
	}
	return overrides
}

// filterSceneScripts filters scene scripts to only include specified scene numbers.
func filterSceneScripts(scenes []domain.SceneScript, sceneNums []int) []domain.SceneScript {
	if len(sceneNums) == 0 {
		return scenes
	}
	wanted := make(map[int]bool, len(sceneNums))
	for _, n := range sceneNums {
		wanted[n] = true
	}
	filtered := make([]domain.SceneScript, 0, len(sceneNums))
	for _, sc := range scenes {
		if wanted[sc.SceneNum] {
			filtered = append(filtered, sc)
		}
	}
	return filtered
}

// backupAudioFile creates a .bak backup of existing audio before overwriting.
func backupAudioFile(audioPath string) {
	if _, err := os.Stat(audioPath); err == nil {
		backupPath := audioPath + ".bak"
		_ = os.Rename(audioPath, backupPath)
	}
}

// updateManifestAudioHash updates the scene manifest with the audio hash.
func (s *TTSService) updateManifestAudioHash(projectID string, sceneNum int, audioHash string) {
	manifest, err := s.store.GetManifest(projectID, sceneNum)
	if err != nil {
		manifest = &domain.SceneManifest{
			ProjectID: projectID,
			SceneNum:  sceneNum,
			AudioHash: audioHash,
			Status:    "audio_generated",
		}
		if createErr := s.store.CreateManifest(manifest); createErr != nil {
			s.logger.Error("failed to create manifest", "project_id", projectID, "scene_num", sceneNum, "err", createErr)
		}
		return
	}
	manifest.AudioHash = audioHash
	manifest.Status = "audio_generated"
	if updateErr := s.store.UpdateManifest(manifest); updateErr != nil {
		s.logger.Error("failed to update manifest", "project_id", projectID, "scene_num", sceneNum, "err", updateErr)
	}
}

// markSceneTTSFailed marks a scene as TTS failed in the manifest.
func (s *TTSService) markSceneTTSFailed(projectID string, sceneNum int) {
	manifest, err := s.store.GetManifest(projectID, sceneNum)
	if err != nil {
		manifest = &domain.SceneManifest{
			ProjectID: projectID,
			SceneNum:  sceneNum,
			Status:    "audio_failed",
		}
		if createErr := s.store.CreateManifest(manifest); createErr != nil {
			s.logger.Error("failed to create failed manifest", "project_id", projectID, "scene_num", sceneNum, "err", createErr)
		}
		return
	}
	manifest.Status = "audio_failed"
	if updateErr := s.store.UpdateManifest(manifest); updateErr != nil {
		s.logger.Error("failed to update failed manifest", "project_id", projectID, "scene_num", sceneNum, "err", updateErr)
	}
}
