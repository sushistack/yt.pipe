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

// GenerateShotImage generates an image for a single shot and saves it to the scene directory.
func (s *ImageGenService) GenerateShotImage(
	ctx context.Context,
	projectID, projectPath string,
	sceneNum, shotNum int,
	prompt, negativePrompt string,
	entityVisible bool,
	scpID string,
	opts imagegen.GenerateOptions,
) (*domain.Shot, error) {
	// Character auto-reference: inject character refs only when entity is visible
	if s.characterSvc != nil && scpID != "" && entityVisible {
		char, _ := s.characterSvc.CheckExistingCharacter(scpID)
		if char != nil && char.SelectedImagePath != "" {
			opts.CharacterRefs = []imagegen.CharacterRef{{
				Name:             char.CanonicalName,
				VisualDescriptor: char.VisualDescriptor,
				ImagePromptBase:  char.ImagePromptBase,
			}}
			s.logger.Info("character refs injected (entity_visible=true)",
				"project_id", projectID, "scene_num", sceneNum, "shot_num", shotNum, "scp_id", scpID)
		}
	}

	var result *imagegen.ImageResult
	genMethod := "text_to_image"
	start := time.Now()

	// Scene-type classification: character-present scenes with reference image → try Edit() first
	useEdit := len(opts.CharacterRefs) > 0 && len(s.selectedCharacterImage) > 0
	if useEdit {
		editResult, editErr := s.imageGen.Edit(ctx, s.selectedCharacterImage, prompt, imagegen.EditOptions{
			Width:  opts.Width,
			Height: opts.Height,
			Model:  opts.Model,
			Seed:   opts.Seed,
		})
		if editErr == nil {
			result = editResult
			genMethod = "image_edit"
		} else if errors.Is(editErr, imagegen.ErrNotSupported) {
			genMethod = "fallback_t2i"
		} else {
			// Edit failed — fall back to text-to-image instead of retrying Edit
			s.logger.Warn("image edit failed, falling back to text-to-image",
				"scene_num", sceneNum, "shot_num", shotNum, "err", editErr)
			genMethod = "fallback_t2i"
		}
	}

	if result == nil {
		err := retry.Do(ctx, 3, 1*time.Second, func() error {
			var genErr error
			result, genErr = s.imageGen.Generate(ctx, prompt, opts)
			return genErr
		})
		if err != nil {
			s.logger.Error("shot image generation failed",
				"project_id", projectID,
				"scene_num", sceneNum,
				"shot_num", shotNum,
				"method", genMethod,
				"err", err,
			)
			s.markShotFailed(projectID, sceneNum, shotNum)
			return nil, fmt.Errorf("image gen: scene %d shot %d: %w", sceneNum, shotNum, err)
		}
	}

	elapsed := time.Since(start).Milliseconds()

	// Save image to scene directory
	sceneDir, err := workspace.InitSceneDir(projectPath, sceneNum)
	if err != nil {
		return nil, fmt.Errorf("image gen: init scene dir %d: %w", sceneNum, err)
	}

	ext := result.Format
	if ext == "" {
		ext = "png"
	}
	imagePath := filepath.Join(sceneDir, fmt.Sprintf("shot_%d.%s", shotNum, ext))
	if err := workspace.WriteFileAtomic(imagePath, result.ImageData); err != nil {
		return nil, fmt.Errorf("image gen: save shot image %d/%d: %w", sceneNum, shotNum, err)
	}

	// Save prompt to scene directory
	promptPath := filepath.Join(sceneDir, fmt.Sprintf("shot_%d_prompt.txt", shotNum))
	if err := workspace.WriteFileAtomic(promptPath, []byte(prompt)); err != nil {
		return nil, fmt.Errorf("image gen: save shot prompt %d/%d: %w", sceneNum, shotNum, err)
	}

	// Compute image hash and update shot manifest
	imgHash := hashBytes(result.ImageData)
	s.updateShotManifestImageHash(projectID, sceneNum, shotNum, imgHash, genMethod)

	s.logger.Info("shot image generated",
		"project_id", projectID,
		"scene_num", sceneNum,
		"shot_num", shotNum,
		"format", ext,
		"method", genMethod,
		"duration_ms", elapsed,
		"image_hash", imgHash[:16],
	)

	return &domain.Shot{
		ShotNum:     shotNum,
		ImagePrompt: prompt,
		ImagePath:   imagePath,
	}, nil
}

// GenerateAllShotImages generates images for all shots across all scenes.
// Skips shots in skipShots map (from incremental checker).
// Fault-tolerant: on per-shot error, logs + marks shot_manifest as failed + continues.
func (s *ImageGenService) GenerateAllShotImages(
	ctx context.Context,
	scenePrompts []*ScenePromptOutput,
	projectID, projectPath, scpID string,
	opts imagegen.GenerateOptions,
	skipShots map[domain.ShotKey]bool,
) ([]*domain.Scene, error) {
	scenes := make([]*domain.Scene, 0, len(scenePrompts))
	var errs []error

	for _, sp := range scenePrompts {
		if sp == nil {
			continue
		}

		if err := ctx.Err(); err != nil {
			s.logger.Warn("shot image generation cancelled", "project_id", projectID)
			errs = append(errs, fmt.Errorf("image gen: cancelled: %w", err))
			break
		}

		scene := &domain.Scene{
			SceneNum: sp.SceneNum,
			Shots:    make([]domain.Shot, 0, len(sp.Shots)),
		}

		for _, shot := range sp.Shots {
			key := domain.ShotKey{SceneNum: sp.SceneNum, ShotNum: shot.ShotNum}
			if skipShots[key] {
				s.logger.Info("shot unchanged, skipping",
					"scene_num", sp.SceneNum, "shot_num", shot.ShotNum)
				// Preserve existing image path from workspace
				existingPath := findExistingShotImage(projectPath, sp.SceneNum, shot.ShotNum)
				scene.Shots = append(scene.Shots, domain.Shot{
					ShotNum:      shot.ShotNum,
					SentenceText: shot.SentenceText,
					ImagePath:    existingPath,
				})
				continue
			}

			if shot.FinalPrompt == "" {
				s.logger.Warn("shot has no prompt, skipping",
					"scene_num", sp.SceneNum, "shot_num", shot.ShotNum)
				scene.Shots = append(scene.Shots, domain.Shot{
					ShotNum:      shot.ShotNum,
					SentenceText: shot.SentenceText,
				})
				continue
			}

			entityVisible := false
			if shot.ShotDesc != nil {
				entityVisible = shot.ShotDesc.EntityVisible
			}

			genShot, err := s.GenerateShotImage(ctx, projectID, projectPath,
				sp.SceneNum, shot.ShotNum,
				shot.FinalPrompt, shot.NegativePrompt,
				entityVisible, scpID, opts)
			if err != nil {
				errs = append(errs, err)
				scene.Shots = append(scene.Shots, domain.Shot{
					ShotNum:      shot.ShotNum,
					SentenceText: shot.SentenceText,
				})
				continue
			}

			genShot.SentenceText = shot.SentenceText
			genShot.NegativePrompt = shot.NegativePrompt
			if shot.ShotDesc != nil {
				genShot.Role = shot.ShotDesc.Role
				genShot.CameraType = shot.ShotDesc.CameraType
				genShot.EntityVisible = shot.ShotDesc.EntityVisible
			}
			scene.Shots = append(scene.Shots, *genShot)
		}

		// Set scene ImagePath to first shot's image (backward compat)
		if len(scene.Shots) > 0 && scene.Shots[0].ImagePath != "" {
			scene.ImagePath = scene.Shots[0].ImagePath
		}

		scenes = append(scenes, scene)
	}

	if len(errs) > 0 {
		return scenes, fmt.Errorf("image gen: %d shot(s) failed: %w", len(errs), errors.Join(errs...))
	}
	return scenes, nil
}

// ReadManualPrompt reads a manually edited prompt file from the scene directory.
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

// updateShotManifestImageHash updates the shot manifest with the image hash and generation method.
func (s *ImageGenService) updateShotManifestImageHash(projectID string, sceneNum, shotNum int, imgHash, genMethod string) {
	manifest, err := s.store.GetShotManifest(projectID, sceneNum, shotNum)
	if err != nil {
		manifest = &domain.ShotManifest{
			ProjectID: projectID,
			SceneNum:  sceneNum,
			ShotNum:   shotNum,
			ImageHash: imgHash,
			GenMethod: genMethod,
			Status:    "generated",
		}
		if createErr := s.store.CreateShotManifest(manifest); createErr != nil {
			s.logger.Error("failed to create shot manifest", "project_id", projectID,
				"scene_num", sceneNum, "shot_num", shotNum, "err", createErr)
		}
		return
	}
	manifest.ImageHash = imgHash
	manifest.GenMethod = genMethod
	manifest.Status = "generated"
	if updateErr := s.store.UpdateShotManifest(manifest); updateErr != nil {
		s.logger.Error("failed to update shot manifest", "project_id", projectID,
			"scene_num", sceneNum, "shot_num", shotNum, "err", updateErr)
	}
}

// markShotFailed marks a shot as failed in the manifest.
func (s *ImageGenService) markShotFailed(projectID string, sceneNum, shotNum int) {
	manifest, err := s.store.GetShotManifest(projectID, sceneNum, shotNum)
	if err != nil {
		manifest = &domain.ShotManifest{
			ProjectID: projectID,
			SceneNum:  sceneNum,
			ShotNum:   shotNum,
			Status:    "failed",
		}
		if createErr := s.store.CreateShotManifest(manifest); createErr != nil {
			s.logger.Error("failed to create failed shot manifest", "project_id", projectID,
				"scene_num", sceneNum, "shot_num", shotNum, "err", createErr)
		}
		return
	}
	manifest.Status = "failed"
	if updateErr := s.store.UpdateShotManifest(manifest); updateErr != nil {
		s.logger.Error("failed to update failed shot manifest", "project_id", projectID,
			"scene_num", sceneNum, "shot_num", shotNum, "err", updateErr)
	}
}

// hashBytes returns the hex-encoded SHA-256 hash of data.
func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// findExistingShotImage finds the existing shot image file on disk.
func findExistingShotImage(projectPath string, sceneNum, shotNum int) string {
	sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", sceneNum))
	for _, ext := range []string{"png", "jpg", "webp"} {
		p := filepath.Join(sceneDir, fmt.Sprintf("shot_%d.%s", shotNum, ext))
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// BackupShotImage backs up an existing shot image before regeneration.
func BackupShotImage(projectPath string, sceneNum, shotNum int) {
	sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", sceneNum))

	for _, ext := range []string{"png", "jpg", "webp"} {
		src := filepath.Join(sceneDir, fmt.Sprintf("shot_%d.%s", shotNum, ext))
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(sceneDir, fmt.Sprintf("shot_%d.prev.%s", shotNum, ext))
			data, readErr := os.ReadFile(src)
			if readErr == nil {
				_ = os.WriteFile(dst, data, 0o644)
				slog.Info("shot image backed up",
					"scene_num", sceneNum,
					"shot_num", shotNum,
					"backup", dst,
				)
			}
			return
		}
	}
}

// BackupSceneImage backs up an existing scene image before regeneration (backward compat).
func BackupSceneImage(projectPath string, sceneNum int) {
	sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", sceneNum))

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
