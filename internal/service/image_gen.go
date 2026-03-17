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

// GenerateShotImage generates an image for a single cut and saves it to the scene directory.
func (s *ImageGenService) GenerateShotImage(
	ctx context.Context,
	projectID, projectPath string,
	sceneNum, sentenceStart, sentenceEnd, cutNum, shotNum int,
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
				StyleGuide:       char.StyleGuide,
			}}
			s.logger.Info("character refs injected (entity_visible=true)",
				"project_id", projectID, "scene_num", sceneNum,
				"sentence_start", sentenceStart, "cut_num", cutNum, "scp_id", scpID)
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
			s.logger.Warn("image edit failed, falling back to text-to-image",
				"scene_num", sceneNum, "sentence_start", sentenceStart, "cut_num", cutNum, "err", editErr)
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
			s.logger.Error("cut image generation failed",
				"project_id", projectID,
				"scene_num", sceneNum,
				"sentence_start", sentenceStart,
				"cut_num", cutNum,
				"method", genMethod,
				"err", err,
			)
			s.markCutFailed(projectID, sceneNum, sentenceStart, sentenceEnd, cutNum, shotNum)
			return nil, fmt.Errorf("image gen: scene %d cut %d_%d: %w", sceneNum, sentenceStart, cutNum, err)
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
	imagePath := filepath.Join(sceneDir, fmt.Sprintf("cut_%d_%d.%s", sentenceStart, cutNum, ext))
	if err := workspace.WriteFileAtomic(imagePath, result.ImageData); err != nil {
		return nil, fmt.Errorf("image gen: save cut image %d/%d_%d: %w", sceneNum, sentenceStart, cutNum, err)
	}

	// Save prompt to scene directory
	promptPath := filepath.Join(sceneDir, fmt.Sprintf("cut_%d_%d_prompt.txt", sentenceStart, cutNum))
	if err := workspace.WriteFileAtomic(promptPath, []byte(prompt)); err != nil {
		return nil, fmt.Errorf("image gen: save cut prompt %d/%d_%d: %w", sceneNum, sentenceStart, cutNum, err)
	}

	// Compute hashes and update shot manifest
	imgHash := hashBytes(result.ImageData)
	contentHash := hashBytes([]byte(prompt))
	s.updateCutManifestImageHash(projectID, sceneNum, sentenceStart, sentenceEnd, cutNum, shotNum, contentHash, imgHash, genMethod)

	s.logger.Info("cut image generated",
		"project_id", projectID,
		"scene_num", sceneNum,
		"sentence_start", sentenceStart,
		"cut_num", cutNum,
		"format", ext,
		"method", genMethod,
		"duration_ms", elapsed,
		"image_hash", imgHash[:16],
	)

	return &domain.Shot{
		SentenceStart: sentenceStart,
		CutNum:        cutNum,
		ImagePrompt:   prompt,
		ImagePath:     imagePath,
	}, nil
}

// GenerateAllCutImages generates images for all cuts across all scenes.
// Skips cuts in skipCuts map (from incremental checker).
// Fault-tolerant: on per-cut error, logs + marks manifest as failed + continues.
func (s *ImageGenService) GenerateAllCutImages(
	ctx context.Context,
	sceneCuts []*SceneCutOutput,
	projectID, projectPath, scpID string,
	opts imagegen.GenerateOptions,
	skipCuts map[domain.ShotKey]bool,
) ([]*domain.Scene, error) {
	scenes := make([]*domain.Scene, 0, len(sceneCuts))
	var errs []error

	for _, sc := range sceneCuts {
		if sc == nil {
			continue
		}

		if err := ctx.Err(); err != nil {
			s.logger.Warn("cut image generation cancelled", "project_id", projectID)
			errs = append(errs, fmt.Errorf("image gen: cancelled: %w", err))
			break
		}

		// Orphan cleanup before generating cuts for this scene
		s.cleanupOrphanCuts(projectID, projectPath, sc.SceneNum, sc.Cuts)

		scene := &domain.Scene{
			SceneNum: sc.SceneNum,
			Shots:    make([]domain.Shot, 0, len(sc.Cuts)),
		}

		for i, cut := range sc.Cuts {
			key := domain.ShotKey{SceneNum: sc.SceneNum, SentenceStart: cut.SentenceStart, CutNum: cut.CutNum}
			if skipCuts[key] {
				s.logger.Info("cut unchanged, skipping",
					"scene_num", sc.SceneNum, "sentence_start", cut.SentenceStart, "cut_num", cut.CutNum)
				existingPath := findExistingCutImage(projectPath, sc.SceneNum, cut.SentenceStart, cut.CutNum)
				scene.Shots = append(scene.Shots, domain.Shot{
					ShotNum:       i + 1,
					SentenceStart: cut.SentenceStart,
					SentenceEnd:   cut.SentenceEnd,
					CutNum:        cut.CutNum,
					ImagePath:     existingPath,
					SentenceText:  cut.SentenceText,
				})
				continue
			}

			if cut.FinalPrompt == "" {
				s.logger.Warn("cut has no prompt, skipping",
					"scene_num", sc.SceneNum, "sentence_start", cut.SentenceStart, "cut_num", cut.CutNum)
				scene.Shots = append(scene.Shots, domain.Shot{
					ShotNum:       i + 1,
					SentenceStart: cut.SentenceStart,
					SentenceEnd:   cut.SentenceEnd,
					CutNum:        cut.CutNum,
					SentenceText:  cut.SentenceText,
				})
				continue
			}

			entityVisible := false
			if cut.CutDesc != nil {
				entityVisible = cut.CutDesc.EntityVisible
			}

			genShot, err := s.GenerateShotImage(ctx, projectID, projectPath,
				sc.SceneNum, cut.SentenceStart, cut.SentenceEnd, cut.CutNum, i+1,
				cut.FinalPrompt, cut.NegativePrompt,
				entityVisible, scpID, opts)
			if err != nil {
				errs = append(errs, err)
				scene.Shots = append(scene.Shots, domain.Shot{
					ShotNum:       i + 1,
					SentenceStart: cut.SentenceStart,
					SentenceEnd:   cut.SentenceEnd,
					CutNum:        cut.CutNum,
					SentenceText:  cut.SentenceText,
				})
				continue
			}

			genShot.ShotNum = i + 1 // sequential index for backward compat
			genShot.SentenceEnd = cut.SentenceEnd
			genShot.SentenceText = cut.SentenceText
			genShot.NegativePrompt = cut.NegativePrompt
			if cut.CutDesc != nil {
				genShot.Role = cut.CutDesc.Role
				genShot.CameraType = cut.CutDesc.CameraType
				genShot.EntityVisible = cut.CutDesc.EntityVisible
			}
			scene.Shots = append(scene.Shots, *genShot)
		}

		// Set scene ImagePath to first cut's image (backward compat)
		if len(scene.Shots) > 0 && scene.Shots[0].ImagePath != "" {
			scene.ImagePath = scene.Shots[0].ImagePath
		}

		scenes = append(scenes, scene)
	}

	if len(errs) > 0 {
		return scenes, fmt.Errorf("image gen: %d cut(s) failed: %w", len(errs), errors.Join(errs...))
	}
	return scenes, nil
}

// GenerateAllShotImages is a backward-compatible wrapper that delegates to GenerateAllCutImages
// by converting ScenePromptOutput to SceneCutOutput.
func (s *ImageGenService) GenerateAllShotImages(
	ctx context.Context,
	scenePrompts []*ScenePromptOutput,
	projectID, projectPath, scpID string,
	opts imagegen.GenerateOptions,
	skipShots map[domain.ShotKey]bool,
) ([]*domain.Scene, error) {
	sceneCuts := make([]*SceneCutOutput, 0, len(scenePrompts))
	for _, sp := range scenePrompts {
		if sp == nil {
			sceneCuts = append(sceneCuts, nil)
			continue
		}
		sc := &SceneCutOutput{
			SceneNum: sp.SceneNum,
			Cuts:     make([]CutOutput, 0, len(sp.Shots)),
		}
		for _, shot := range sp.Shots {
			sc.Cuts = append(sc.Cuts, CutOutput{
				SentenceStart:  shot.ShotNum,
				SentenceEnd:    shot.ShotNum,
				CutNum:         1,
				CutDesc:        shotDescToCutDesc(shot.ShotDesc),
				FinalPrompt:    shot.FinalPrompt,
				NegativePrompt: shot.NegativePrompt,
			})
		}
		sceneCuts = append(sceneCuts, sc)
	}
	// Convert legacy skip map keys: {SceneNum, ShotNum} → {SceneNum, SentenceStart=ShotNum, CutNum=1}
	convertedSkip := make(map[domain.ShotKey]bool, len(skipShots))
	for k, v := range skipShots {
		convertedSkip[domain.ShotKey{SceneNum: k.SceneNum, SentenceStart: k.ShotNum, CutNum: 1}] = v
	}
	return s.GenerateAllCutImages(ctx, sceneCuts, projectID, projectPath, scpID, opts, convertedSkip)
}

// cleanupOrphanCuts removes manifest entries and images for cuts no longer present.
func (s *ImageGenService) cleanupOrphanCuts(projectID, projectPath string, sceneNum int, newCuts []CutOutput) {
	existing, err := s.store.ListShotManifestsByScene(projectID, sceneNum)
	if err != nil || len(existing) == 0 {
		return
	}

	newKeySet := make(map[string]bool, len(newCuts))
	for _, c := range newCuts {
		key := fmt.Sprintf("%d_%d", c.SentenceStart, c.CutNum)
		newKeySet[key] = true
	}

	for _, m := range existing {
		key := fmt.Sprintf("%d_%d", m.SentenceStart, m.CutNum)
		if !newKeySet[key] {
			_ = s.store.DeleteShotManifest(projectID, sceneNum, m.SentenceStart, m.CutNum)
			removeOrphanCutImage(projectPath, sceneNum, m.SentenceStart, m.CutNum)
			s.logger.Info("orphan cut cleaned up",
				"scene_num", sceneNum, "sentence_start", m.SentenceStart, "cut_num", m.CutNum)
		}
	}
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

// updateCutManifestImageHash updates the cut manifest with the image hash and generation method.
func (s *ImageGenService) updateCutManifestImageHash(projectID string, sceneNum, sentenceStart, sentenceEnd, cutNum, shotNum int, contentHash, imgHash, genMethod string) {
	manifest, err := s.store.GetShotManifest(projectID, sceneNum, sentenceStart, cutNum)
	if err != nil {
		manifest = &domain.ShotManifest{
			ProjectID:     projectID,
			SceneNum:      sceneNum,
			ShotNum:       shotNum,
			SentenceStart: sentenceStart,
			SentenceEnd:   sentenceEnd,
			CutNum:        cutNum,
			ContentHash:   contentHash,
			ImageHash:     imgHash,
			GenMethod:     genMethod,
			Status:        "generated",
		}
		if createErr := s.store.CreateShotManifest(manifest); createErr != nil {
			s.logger.Error("failed to create cut manifest", "project_id", projectID,
				"scene_num", sceneNum, "sentence_start", sentenceStart, "cut_num", cutNum, "err", createErr)
		}
		return
	}
	manifest.ContentHash = contentHash
	manifest.SentenceEnd = sentenceEnd
	manifest.ImageHash = imgHash
	manifest.GenMethod = genMethod
	manifest.Status = "generated"
	if updateErr := s.store.UpdateShotManifest(manifest); updateErr != nil {
		s.logger.Error("failed to update cut manifest", "project_id", projectID,
			"scene_num", sceneNum, "sentence_start", sentenceStart, "cut_num", cutNum, "err", updateErr)
	}
}

// markCutFailed marks a cut as failed in the manifest.
func (s *ImageGenService) markCutFailed(projectID string, sceneNum, sentenceStart, sentenceEnd, cutNum, shotNum int) {
	manifest, err := s.store.GetShotManifest(projectID, sceneNum, sentenceStart, cutNum)
	if err != nil {
		manifest = &domain.ShotManifest{
			ProjectID:     projectID,
			SceneNum:      sceneNum,
			ShotNum:       shotNum,
			SentenceStart: sentenceStart,
			SentenceEnd:   sentenceEnd,
			CutNum:        cutNum,
			Status:        "failed",
		}
		if createErr := s.store.CreateShotManifest(manifest); createErr != nil {
			s.logger.Error("failed to create failed cut manifest", "project_id", projectID,
				"scene_num", sceneNum, "sentence_start", sentenceStart, "cut_num", cutNum, "err", createErr)
		}
		return
	}
	manifest.Status = "failed"
	if updateErr := s.store.UpdateShotManifest(manifest); updateErr != nil {
		s.logger.Error("failed to update failed cut manifest", "project_id", projectID,
			"scene_num", sceneNum, "sentence_start", sentenceStart, "cut_num", cutNum, "err", updateErr)
	}
}

// hashBytes returns the hex-encoded SHA-256 hash of data.
func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// findExistingCutImage finds the existing cut image file on disk.
// Falls back to legacy shot_N naming for upgrade compatibility.
func findExistingCutImage(projectPath string, sceneNum, sentenceStart, cutNum int) string {
	sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", sceneNum))
	// Try new naming first
	for _, ext := range []string{"png", "jpg", "webp"} {
		p := filepath.Join(sceneDir, fmt.Sprintf("cut_%d_%d.%s", sentenceStart, cutNum, ext))
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// Fallback: legacy shot_N naming (for upgraded projects)
	if cutNum == 1 {
		for _, ext := range []string{"png", "jpg", "webp"} {
			p := filepath.Join(sceneDir, fmt.Sprintf("shot_%d.%s", sentenceStart, ext))
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// findExistingShotImage finds the existing shot image file on disk (legacy naming).
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

// removeOrphanCutImage removes an orphaned cut image file.
func removeOrphanCutImage(projectPath string, sceneNum, sentenceStart, cutNum int) {
	sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", sceneNum))
	for _, ext := range []string{"png", "jpg", "webp"} {
		p := filepath.Join(sceneDir, fmt.Sprintf("cut_%d_%d.%s", sentenceStart, cutNum, ext))
		if _, err := os.Stat(p); err == nil {
			_ = os.Remove(p)
			// Also remove prompt file
			_ = os.Remove(filepath.Join(sceneDir, fmt.Sprintf("cut_%d_%d_prompt.txt", sentenceStart, cutNum)))
			return
		}
	}
}

// BackupShotImage backs up an existing shot image before regeneration (legacy naming).
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

// shotDescToCutDesc converts a legacy ShotDescription to CutDescription for backward compat.
func shotDescToCutDesc(sd *ShotDescription) *CutDescription {
	if sd == nil {
		return nil
	}
	return &CutDescription{
		Role:          sd.Role,
		CameraType:    sd.CameraType,
		EntityVisible: sd.EntityVisible,
		Subject:       sd.Subject,
		Lighting:      sd.Lighting,
		Mood:          sd.Mood,
		Motion:        sd.Motion,
	}
}
