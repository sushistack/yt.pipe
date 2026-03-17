package pipeline

import (
	"fmt"
	"log/slog"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
)

// IncrementalResult tracks what was processed vs skipped in an incremental build.
type IncrementalResult struct {
	TotalScenes      int `json:"total_scenes"`
	Regenerated      int `json:"regenerated"`
	Skipped          int `json:"skipped"`
}

// Summary returns a human-readable summary of the incremental build result.
func (ir IncrementalResult) Summary() string {
	return fmt.Sprintf("%d scenes regenerated, %d skipped", ir.Regenerated, ir.Skipped)
}

// SceneSkipChecker determines which scenes need regeneration based on manifest hashes.
type SceneSkipChecker struct {
	store    *store.Store
	logger   *slog.Logger
}

// NewSceneSkipChecker creates a new SceneSkipChecker.
func NewSceneSkipChecker(s *store.Store, logger *slog.Logger) *SceneSkipChecker {
	return &SceneSkipChecker{store: s, logger: logger}
}

// FilterShotsForImageGen returns shot keys that need image regeneration.
// Compares current sentence hashes against stored shot manifest hashes.
func (c *SceneSkipChecker) FilterShotsForImageGen(
	projectID string,
	scenePrompts []*service.ScenePromptOutput,
) (toGenerate, toSkip []domain.ShotKey) {
	for _, sp := range scenePrompts {
		if sp == nil {
			continue
		}

		// Check if scene shot count changed — invalidate all shots if so
		storedShots, _ := c.store.ListShotManifestsByScene(projectID, sp.SceneNum)
		if len(storedShots) != len(sp.Shots) && len(storedShots) > 0 {
			c.logger.Info("scene shot count changed, invalidating all shots",
				"scene_num", sp.SceneNum,
				"stored", len(storedShots),
				"new", len(sp.Shots))
			_ = c.store.DeleteShotManifestsByScene(projectID, sp.SceneNum)
			// All shots need regeneration
			for _, shot := range sp.Shots {
				toGenerate = append(toGenerate, domain.ShotKey{SceneNum: sp.SceneNum, ShotNum: shot.ShotNum})
			}
			continue
		}

		for _, shot := range sp.Shots {
			key := domain.ShotKey{SceneNum: sp.SceneNum, ShotNum: shot.ShotNum}
			currentHash := service.ContentHash([]byte(shot.SentenceText))

			// Legacy path: use shot_num as sentence_start, cut_num=1
			manifest, err := c.store.GetShotManifest(projectID, sp.SceneNum, shot.ShotNum, 1)
			if err == nil && manifest.ContentHash == currentHash && manifest.ImageHash != "" {
				c.logger.Info("shot unchanged, skipping",
					"scene_num", sp.SceneNum, "shot_num", shot.ShotNum, "asset", "image")
				toSkip = append(toSkip, key)
			} else {
				toGenerate = append(toGenerate, key)
			}
		}
	}
	return
}

// FilterCutsForImageGen returns cut keys that need image regeneration.
// Compares current content hashes (narration + scene metadata) against stored manifest hashes.
func (c *SceneSkipChecker) FilterCutsForImageGen(
	projectID string,
	sceneCuts []*service.SceneCutOutput,
) (toGenerate, toSkip []domain.ShotKey) {
	for _, sc := range sceneCuts {
		if sc == nil {
			continue
		}

		storedManifests, _ := c.store.ListShotManifestsByScene(projectID, sc.SceneNum)
		if len(storedManifests) != len(sc.Cuts) && len(storedManifests) > 0 {
			c.logger.Info("scene cut count changed, invalidating all cuts",
				"scene_num", sc.SceneNum,
				"stored", len(storedManifests),
				"new", len(sc.Cuts))
			_ = c.store.DeleteShotManifestsByScene(projectID, sc.SceneNum)
			for _, cut := range sc.Cuts {
				toGenerate = append(toGenerate, domain.ShotKey{
					SceneNum: sc.SceneNum, SentenceStart: cut.SentenceStart, CutNum: cut.CutNum})
			}
			continue
		}

		for _, cut := range sc.Cuts {
			key := domain.ShotKey{SceneNum: sc.SceneNum, SentenceStart: cut.SentenceStart, CutNum: cut.CutNum}
			currentHash := service.ContentHash([]byte(cut.FinalPrompt))

			manifest, err := c.store.GetShotManifest(projectID, sc.SceneNum, cut.SentenceStart, cut.CutNum)
			if err == nil && manifest.ContentHash == currentHash && manifest.ImageHash != "" {
				c.logger.Info("cut unchanged, skipping",
					"scene_num", sc.SceneNum, "sentence_start", cut.SentenceStart, "cut_num", cut.CutNum)
				toSkip = append(toSkip, key)
			} else {
				toGenerate = append(toGenerate, key)
			}
		}
	}
	return
}

// FilterScenesForTTS returns scene numbers that need TTS regeneration.
// Compares narration content hashes against stored manifest hashes.
func (c *SceneSkipChecker) FilterScenesForTTS(projectID string, narrations map[int]string) (toGenerate, toSkip []int) {
	manifests, err := c.store.ListManifestsByProject(projectID)
	if err != nil || len(manifests) == 0 {
		for sceneNum := range narrations {
			toGenerate = append(toGenerate, sceneNum)
		}
		return
	}

	manifestMap := make(map[int]*manifestEntry, len(manifests))
	for _, m := range manifests {
		manifestMap[m.SceneNum] = &manifestEntry{
			contentHash: m.ContentHash,
			audioHash:   m.AudioHash,
		}
	}

	for sceneNum, narration := range narrations {
		currentHash := service.ContentHash([]byte(narration))
		if entry, ok := manifestMap[sceneNum]; ok && entry.contentHash == currentHash && entry.audioHash != "" {
			c.logger.Info("scene unchanged, skipping",
				"scene_num", sceneNum,
				"asset", "audio")
			toSkip = append(toSkip, sceneNum)
		} else {
			toGenerate = append(toGenerate, sceneNum)
		}
	}
	return
}

type manifestEntry struct {
	contentHash string
	imageHash   string
	audioHash   string
}
