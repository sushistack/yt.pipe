package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/glossary"
	"github.com/jay/youtube-pipeline/internal/plugin/tts"
	"github.com/jay/youtube-pipeline/internal/retry"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/jay/youtube-pipeline/internal/workspace"
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
// Uses retry with exponential backoff for the TTS API call.
func (s *TTSService) SynthesizeScene(ctx context.Context, scene domain.SceneScript, projectID, projectPath, voice string) (*domain.Scene, error) {
	overrides := s.buildOverrides()

	var result *tts.SynthesisResult
	start := time.Now()

	err := retry.Do(ctx, 3, 1*time.Second, func() error {
		var synthErr error
		if len(overrides) > 0 {
			result, synthErr = s.tts.SynthesizeWithOverrides(ctx, scene.Narration, voice, overrides)
		} else {
			result, synthErr = s.tts.Synthesize(ctx, scene.Narration, voice)
		}
		return synthErr
	})
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

// SynthesizeAll synthesizes narration for all or selected scenes.
// If sceneNums is non-empty, only those scenes are synthesized (AC3: selective re-synthesis).
// On partial failure, continues processing remaining scenes.
func (s *TTSService) SynthesizeAll(ctx context.Context, scenes []domain.SceneScript, projectID, projectPath, voice string, sceneNums []int) ([]*domain.Scene, error) {
	filtered := filterSceneScripts(scenes, sceneNums)

	results := make([]*domain.Scene, 0, len(filtered))
	var errs []error
	for _, scene := range filtered {
		if err := ctx.Err(); err != nil {
			s.logger.Warn("tts synthesis cancelled", "project_id", projectID, "remaining", len(filtered)-len(results)-len(errs))
			errs = append(errs, fmt.Errorf("tts: cancelled: %w", err))
			break
		}

		result, err := s.SynthesizeScene(ctx, scene, projectID, projectPath, voice)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		results = append(results, result)
	}
	if len(errs) > 0 {
		return results, fmt.Errorf("tts: %d/%d scenes failed: %w", len(errs), len(filtered), errors.Join(errs...))
	}
	return results, nil
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
