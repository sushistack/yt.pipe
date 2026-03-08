package pipeline

import (
	"fmt"
	"log/slog"

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

// FilterScenesForImageGen returns scene numbers that need image regeneration.
// Compares current prompt hashes against stored manifest hashes.
func (c *SceneSkipChecker) FilterScenesForImageGen(projectID string, prompts []service.ImagePromptResult) (toGenerate, toSkip []int) {
	manifests, err := c.store.ListManifestsByProject(projectID)
	if err != nil || len(manifests) == 0 {
		// No manifests = all scenes need generation
		for _, p := range prompts {
			toGenerate = append(toGenerate, p.SceneNum)
		}
		return
	}

	manifestMap := make(map[int]*manifestEntry, len(manifests))
	for _, m := range manifests {
		manifestMap[m.SceneNum] = &manifestEntry{
			contentHash: m.ContentHash,
			imageHash:   m.ImageHash,
		}
	}

	for _, p := range prompts {
		currentHash := service.ContentHash([]byte(p.SanitizedPrompt))
		if entry, ok := manifestMap[p.SceneNum]; ok && entry.contentHash == currentHash && entry.imageHash != "" {
			c.logger.Info("scene unchanged, skipping",
				"scene_num", p.SceneNum,
				"asset", "image")
			toSkip = append(toSkip, p.SceneNum)
		} else {
			toGenerate = append(toGenerate, p.SceneNum)
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
