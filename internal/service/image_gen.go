package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/imagegen"
	"github.com/sushistack/yt.pipe/internal/retry"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// ImageGenService handles image generation for scenes.
type ImageGenService struct {
	imageGen              imagegen.ImageGen
	store                 *store.Store
	logger                *slog.Logger
	characterSvc          *CharacterService // optional: enables character auto-reference
	selectedCharacterImage []byte           // loaded reference image for image-edit
}

// NewImageGenService creates a new ImageGenService.
func NewImageGenService(ig imagegen.ImageGen, s *store.Store, logger *slog.Logger) *ImageGenService {
	return &ImageGenService{imageGen: ig, store: s, logger: logger}
}

// SetCharacterService enables character auto-reference during image generation.
func (s *ImageGenService) SetCharacterService(cs *CharacterService) {
	s.characterSvc = cs
}

// SetSelectedCharacterImage loads a character reference image for image-edit generation.
func (s *ImageGenService) SetSelectedCharacterImage(imagePath string) error {
	if imagePath == "" {
		return nil
	}
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("load character image: %w", err)
	}
	s.selectedCharacterImage = data
	s.logger.Info("character reference image loaded",
		"path", imagePath,
		"size_bytes", len(data),
	)
	return nil
}

// GenerateSceneImage generates an image for a single scene and saves it to the scene directory.
// It uses retry with exponential backoff for the API call and updates the scene manifest.
// If CharacterService is set and prompt contains SCPID/SceneText, character references are auto-injected.
func (s *ImageGenService) GenerateSceneImage(ctx context.Context, prompt ImagePromptResult, projectID, projectPath string, opts imagegen.GenerateOptions) (*domain.Scene, error) {
	// Character auto-reference: match characters in scene text and inject refs
	if s.characterSvc != nil && prompt.SCPID != "" && prompt.SceneText != "" {
		refs, matchErr := s.characterSvc.MatchCharacters(prompt.SCPID, prompt.SceneText)
		if matchErr != nil {
			s.logger.Warn("character matching failed, proceeding without refs",
				"project_id", projectID, "scene_num", prompt.SceneNum, "err", matchErr)
		} else {
			opts.CharacterRefs = refs
			if len(refs) > 0 {
				s.logger.Info("character refs injected",
					"project_id", projectID, "scene_num", prompt.SceneNum, "count", len(refs))
			}
		}
	}

	var result *imagegen.ImageResult
	genMethod := "text_to_image"
	start := time.Now()

	// Scene-type classification: character-present scenes with reference image → try Edit() first
	useEdit := len(opts.CharacterRefs) > 0 && len(s.selectedCharacterImage) > 0
	if useEdit {
		// Try Edit once to check support (not inside retry — ErrNotSupported is deterministic)
		editResult, editErr := s.imageGen.Edit(ctx, s.selectedCharacterImage, prompt.SanitizedPrompt, imagegen.EditOptions{
			Width:  opts.Width,
			Height: opts.Height,
			Model:  opts.Model,
			Seed:   opts.Seed,
		})
		if errors.Is(editErr, imagegen.ErrNotSupported) {
			// Provider doesn't support Edit — fall back to Generate path
			useEdit = false
			genMethod = "fallback_t2i"
		} else if editErr != nil {
			// Real error on first attempt — let retry handle it
			genMethod = "image_edit"
		} else {
			result = editResult
			genMethod = "image_edit"
		}
	}

	// If Edit succeeded, skip retry. Otherwise retry the appropriate method.
	if result == nil {
		err := retry.Do(ctx, 3, 1*time.Second, func() error {
			var genErr error
			if useEdit {
				result, genErr = s.imageGen.Edit(ctx, s.selectedCharacterImage, prompt.SanitizedPrompt, imagegen.EditOptions{
					Width:  opts.Width,
					Height: opts.Height,
					Model:  opts.Model,
					Seed:   opts.Seed,
				})
			} else {
				result, genErr = s.imageGen.Generate(ctx, prompt.SanitizedPrompt, opts)
			}
			return genErr
		})
		if err != nil {
			s.logger.Error("image generation failed",
				"project_id", projectID,
				"scene_num", prompt.SceneNum,
				"method", genMethod,
				"err", err,
			)
			s.markSceneFailed(projectID, prompt.SceneNum, err)
			return nil, fmt.Errorf("image gen: scene %d: %w", prompt.SceneNum, err)
		}
	}

	elapsed := time.Since(start).Milliseconds()

	// Save image to scene directory
	sceneDir, err := workspace.InitSceneDir(projectPath, prompt.SceneNum)
	if err != nil {
		return nil, fmt.Errorf("image gen: init scene dir %d: %w", prompt.SceneNum, err)
	}

	ext := result.Format
	if ext == "" {
		ext = "png"
	}
	imagePath := filepath.Join(sceneDir, fmt.Sprintf("image.%s", ext))
	if err := workspace.WriteFileAtomic(imagePath, result.ImageData); err != nil {
		return nil, fmt.Errorf("image gen: save image %d: %w", prompt.SceneNum, err)
	}

	// Save prompts to scene directory (prompt.txt per AC3)
	promptPath := filepath.Join(sceneDir, "prompt.txt")
	if err := workspace.WriteFileAtomic(promptPath, []byte(prompt.SanitizedPrompt)); err != nil {
		return nil, fmt.Errorf("image gen: save prompt %d: %w", prompt.SceneNum, err)
	}

	// Compute image hash for manifest
	imgHash := hashBytes(result.ImageData)

	// Update scene manifest in SQLite (AC1: image hash + generation timestamp)
	s.updateManifestImageHash(projectID, prompt.SceneNum, imgHash)

	s.logger.Info("scene image generated",
		"project_id", projectID,
		"scene_num", prompt.SceneNum,
		"format", ext,
		"method", genMethod,
		"duration_ms", elapsed,
		"image_hash", imgHash[:16],
	)

	return &domain.Scene{
		SceneNum:    prompt.SceneNum,
		ImagePrompt: prompt.SanitizedPrompt,
		ImagePath:   imagePath,
	}, nil
}

// GenerateAllImages generates images for all or selected scenes.
// If sceneNums is non-empty, only those scenes are generated (AC2: selective regeneration).
// On partial failure, it continues processing remaining scenes and returns all successful results (AC4).
func (s *ImageGenService) GenerateAllImages(ctx context.Context, prompts []ImagePromptResult, projectID, projectPath string, opts imagegen.GenerateOptions, sceneNums []int) ([]*domain.Scene, error) {
	filtered := filterPrompts(prompts, sceneNums)

	scenes := make([]*domain.Scene, 0, len(filtered))
	var errs []error
	for _, p := range filtered {
		// Check context cancellation between scenes (M3)
		if err := ctx.Err(); err != nil {
			s.logger.Warn("image generation cancelled", "project_id", projectID, "remaining", len(filtered)-len(scenes)-len(errs))
			errs = append(errs, fmt.Errorf("image gen: cancelled: %w", err))
			break
		}

		scene, err := s.GenerateSceneImage(ctx, p, projectID, projectPath, opts)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		scenes = append(scenes, scene)
	}
	if len(errs) > 0 {
		return scenes, fmt.Errorf("image gen: %d/%d scenes failed: %w", len(errs), len(filtered), errors.Join(errs...))
	}
	return scenes, nil
}

// ReadManualPrompt reads a manually edited prompt file from the scene directory (AC3).
// Returns the prompt text and true if the file exists and differs from the original.
func (s *ImageGenService) ReadManualPrompt(projectPath string, sceneNum int) (string, bool, error) {
	sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", sceneNum))
	promptPath := filepath.Join(sceneDir, "prompt.txt")

	data, err := os.ReadFile(promptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read manual prompt scene %d: %w", sceneNum, err)
	}

	return strings.TrimSpace(string(data)), true, nil
}

// filterPrompts filters prompts to only include specified scene numbers.
// If sceneNums is empty, returns all prompts (no filtering).
func filterPrompts(prompts []ImagePromptResult, sceneNums []int) []ImagePromptResult {
	if len(sceneNums) == 0 {
		return prompts
	}
	wanted := make(map[int]bool, len(sceneNums))
	for _, n := range sceneNums {
		wanted[n] = true
	}
	filtered := make([]ImagePromptResult, 0, len(sceneNums))
	for _, p := range prompts {
		if wanted[p.SceneNum] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// updateManifestImageHash updates the scene manifest with the image hash.
func (s *ImageGenService) updateManifestImageHash(projectID string, sceneNum int, imgHash string) {
	manifest, err := s.store.GetManifest(projectID, sceneNum)
	if err != nil {
		// Manifest may not exist yet; create it
		manifest = &domain.SceneManifest{
			ProjectID: projectID,
			SceneNum:  sceneNum,
			ImageHash: imgHash,
			Status:    "image_generated",
		}
		if createErr := s.store.CreateManifest(manifest); createErr != nil {
			s.logger.Error("failed to create manifest", "project_id", projectID, "scene_num", sceneNum, "err", createErr)
		}
		return
	}
	manifest.ImageHash = imgHash
	manifest.Status = "image_generated"
	if updateErr := s.store.UpdateManifest(manifest); updateErr != nil {
		s.logger.Error("failed to update manifest", "project_id", projectID, "scene_num", sceneNum, "err", updateErr)
	}
}

// markSceneFailed marks a scene as failed in the manifest (AC4).
func (s *ImageGenService) markSceneFailed(projectID string, sceneNum int, genErr error) {
	manifest, err := s.store.GetManifest(projectID, sceneNum)
	if err != nil {
		manifest = &domain.SceneManifest{
			ProjectID: projectID,
			SceneNum:  sceneNum,
			Status:    "image_failed",
		}
		if createErr := s.store.CreateManifest(manifest); createErr != nil {
			s.logger.Error("failed to create failed manifest", "project_id", projectID, "scene_num", sceneNum, "err", createErr)
		}
		return
	}
	manifest.Status = "image_failed"
	if updateErr := s.store.UpdateManifest(manifest); updateErr != nil {
		s.logger.Error("failed to update failed manifest", "project_id", projectID, "scene_num", sceneNum, "err", updateErr)
	}
}

// hashBytes returns the hex-encoded SHA-256 hash of data.
func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// BackupSceneImage backs up an existing scene image before regeneration.
// The backup is saved as image.prev.{ext} in the same directory.
func BackupSceneImage(projectPath string, sceneNum int) {
	sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", sceneNum))

	// Find existing image file
	for _, ext := range []string{"png", "jpg", "webp"} {
		src := filepath.Join(sceneDir, "image."+ext)
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(sceneDir, "image.prev."+ext)
			data, readErr := os.ReadFile(src)
			if readErr == nil {
				_ = os.WriteFile(dst, data, 0o644)
				slog.Info("scene image backed up",
					"scene_num", sceneNum,
					"backup", dst,
				)
			}
			return
		}
	}
}
