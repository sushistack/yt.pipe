package pipeline

import (
	"log/slog"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/jay/youtube-pipeline/internal/store"
)

// AssetType represents a type of scene asset in the dependency chain.
type AssetType string

const (
	AssetNarration AssetType = "narration"
	AssetPrompt    AssetType = "prompt"
	AssetImage     AssetType = "image"
	AssetAudio     AssetType = "audio"
	AssetTiming    AssetType = "timing"
	AssetSubtitle  AssetType = "subtitle"
)

// dependencyChain defines which assets are downstream of each asset type.
// narration → prompt → image
// narration → audio → timing → subtitle
var dependencyChain = map[AssetType][]AssetType{
	AssetNarration: {AssetPrompt, AssetImage, AssetAudio, AssetTiming, AssetSubtitle},
	AssetPrompt:    {AssetImage},
	AssetAudio:     {AssetTiming, AssetSubtitle},
	AssetTiming:    {AssetSubtitle},
}

// DependencyTracker manages the scene dependency chain for stale invalidation.
type DependencyTracker struct {
	store  *store.Store
	logger *slog.Logger
}

// NewDependencyTracker creates a new DependencyTracker.
func NewDependencyTracker(s *store.Store, logger *slog.Logger) *DependencyTracker {
	return &DependencyTracker{store: s, logger: logger}
}

// InvalidationResult records which assets were invalidated and why.
type InvalidationResult struct {
	SceneNum      int       `json:"scene_num"`
	ChangedAsset  AssetType `json:"changed_asset"`
	Invalidated   []AssetType `json:"invalidated"`
}

// InvalidateDownstream marks downstream assets as stale when an upstream asset changes.
// Returns the list of assets that were invalidated.
func (dt *DependencyTracker) InvalidateDownstream(projectID string, sceneNum int, changedAsset AssetType) (*InvalidationResult, error) {
	downstream, ok := dependencyChain[changedAsset]
	if !ok || len(downstream) == 0 {
		return &InvalidationResult{
			SceneNum:     sceneNum,
			ChangedAsset: changedAsset,
		}, nil
	}

	manifest, err := dt.store.GetManifest(projectID, sceneNum)
	if err != nil {
		// Create a new manifest entry if not found
		manifest = &domain.SceneManifest{
			ProjectID: projectID,
			SceneNum:  sceneNum,
			Status:    "stale",
		}
		if createErr := dt.store.CreateManifest(manifest); createErr != nil {
			return nil, createErr
		}
	}

	// Clear hashes for all downstream assets
	for _, asset := range downstream {
		switch asset {
		case AssetPrompt:
			manifest.ContentHash = ""
		case AssetImage:
			manifest.ImageHash = ""
		case AssetAudio:
			manifest.AudioHash = ""
		case AssetSubtitle:
			manifest.SubtitleHash = ""
		}
	}
	manifest.Status = "stale"

	if err := dt.store.UpdateManifest(manifest); err != nil {
		// If update fails (new manifest), try create
		if createErr := dt.store.CreateManifest(manifest); createErr != nil {
			return nil, createErr
		}
	}

	dt.logger.Info("dependency invalidation",
		"project_id", projectID,
		"scene_num", sceneNum,
		"changed", changedAsset,
		"invalidated", downstream)

	return &InvalidationResult{
		SceneNum:     sceneNum,
		ChangedAsset: changedAsset,
		Invalidated:  downstream,
	}, nil
}

// DetectChanges compares current content hashes against stored manifests
// and triggers invalidation for changed scenes. Returns scenes that need regeneration.
func (dt *DependencyTracker) DetectChanges(projectID string, sceneHashes map[int]map[AssetType]string) ([]InvalidationResult, error) {
	manifests, err := dt.store.ListManifestsByProject(projectID)
	if err != nil {
		return nil, nil // No manifests = nothing to invalidate
	}

	manifestMap := make(map[int]*domain.SceneManifest, len(manifests))
	for _, m := range manifests {
		manifestMap[m.SceneNum] = m
	}

	var results []InvalidationResult

	for sceneNum, hashes := range sceneHashes {
		existing, ok := manifestMap[sceneNum]
		if !ok {
			continue
		}

		// Check narration change
		if hash, ok := hashes[AssetNarration]; ok {
			currentNarrationHash := service.ContentHash([]byte(hash))
			if existing.ContentHash != "" && existing.ContentHash != currentNarrationHash {
				result, err := dt.InvalidateDownstream(projectID, sceneNum, AssetNarration)
				if err != nil {
					return results, err
				}
				results = append(results, *result)
				continue // Narration invalidates everything, no need to check further
			}
		}

		// Check prompt change
		if hash, ok := hashes[AssetPrompt]; ok {
			currentPromptHash := service.ContentHash([]byte(hash))
			if existing.ContentHash != "" && existing.ContentHash != currentPromptHash {
				result, err := dt.InvalidateDownstream(projectID, sceneNum, AssetPrompt)
				if err != nil {
					return results, err
				}
				results = append(results, *result)
			}
		}

		// Check audio change
		if hash, ok := hashes[AssetAudio]; ok {
			if existing.AudioHash != "" && existing.AudioHash != hash {
				result, err := dt.InvalidateDownstream(projectID, sceneNum, AssetAudio)
				if err != nil {
					return results, err
				}
				results = append(results, *result)
			}
		}
	}

	return results, nil
}

// GetStaleScenes returns scene numbers that have stale assets.
func (dt *DependencyTracker) GetStaleScenes(projectID string) ([]int, error) {
	manifests, err := dt.store.ListManifestsByProject(projectID)
	if err != nil {
		return nil, err
	}

	var stale []int
	for _, m := range manifests {
		if m.Status == "stale" {
			stale = append(stale, m.SceneNum)
		}
	}
	return stale, nil
}
