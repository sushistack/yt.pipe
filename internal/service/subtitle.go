package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// SubtitleEntry represents a single subtitle cue.
type SubtitleEntry struct {
	Index    int     `json:"index"`
	StartSec float64 `json:"start_sec"`
	EndSec   float64 `json:"end_sec"`
	Text     string  `json:"text"`
}

// SubtitleService handles subtitle generation for scenes.
type SubtitleService struct {
	glossary *glossary.Glossary
	store    *store.Store
	logger   *slog.Logger
}

// NewSubtitleService creates a new SubtitleService.
func NewSubtitleService(g *glossary.Glossary, s *store.Store, logger *slog.Logger) *SubtitleService {
	return &SubtitleService{glossary: g, store: s, logger: logger}
}

// GenerateSubtitles creates subtitle entries from word timings for a scene.
// Groups words into subtitle segments of maxWordsPerLine (default 8).
// Applies glossary canonical spelling (AC2).
func (svc *SubtitleService) GenerateSubtitles(scene *domain.Scene, maxWordsPerLine int) []SubtitleEntry {
	if maxWordsPerLine <= 0 {
		maxWordsPerLine = 8
	}

	timings := scene.WordTimings
	if len(timings) == 0 {
		return nil
	}

	var entries []SubtitleEntry
	index := 1

	for i := 0; i < len(timings); i += maxWordsPerLine {
		end := i + maxWordsPerLine
		if end > len(timings) {
			end = len(timings)
		}
		chunk := timings[i:end]

		var words []string
		for _, wt := range chunk {
			word := svc.canonicalSpelling(wt.Word)
			words = append(words, word)
		}

		entries = append(entries, SubtitleEntry{
			Index:    index,
			StartSec: chunk[0].StartSec,
			EndSec:   chunk[len(chunk)-1].EndSec,
			Text:     strings.Join(words, " "),
		})
		index++
	}

	return entries
}

// SaveSceneSubtitles generates and saves subtitles for a single scene as JSON (AC1: subtitle.json).
func (svc *SubtitleService) SaveSceneSubtitles(scene *domain.Scene, projectID, projectPath string, maxWordsPerLine int) (string, error) {
	entries := svc.GenerateSubtitles(scene, maxWordsPerLine)
	if len(entries) == 0 {
		return "", nil
	}

	sceneDir, err := workspace.InitSceneDir(projectPath, scene.SceneNum)
	if err != nil {
		return "", fmt.Errorf("subtitle: init scene dir %d: %w", scene.SceneNum, err)
	}

	// Save as JSON (AC1: subtitle.json)
	subtitlePath := filepath.Join(sceneDir, "subtitle.json")
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("subtitle: marshal scene %d: %w", scene.SceneNum, err)
	}
	if err := workspace.WriteFileAtomic(subtitlePath, data); err != nil {
		return "", fmt.Errorf("subtitle: save scene %d: %w", scene.SceneNum, err)
	}

	// Also save SRT for convenience
	srtPath := filepath.Join(sceneDir, "subtitle.srt")
	srt := FormatSRT(entries)
	if err := workspace.WriteFileAtomic(srtPath, []byte(srt)); err != nil {
		return "", fmt.Errorf("subtitle: save srt scene %d: %w", scene.SceneNum, err)
	}

	// Update manifest subtitle hash
	subtitleHash := hashBytes(data)
	svc.updateManifestSubtitleHash(projectID, scene.SceneNum, subtitleHash)

	svc.logger.Info("scene subtitles generated",
		"project_id", projectID,
		"scene_num", scene.SceneNum,
		"entries", len(entries),
	)

	return subtitlePath, nil
}

// SaveAllSubtitles generates subtitles for all or selected scenes and creates a combined project-level file (AC3).
func (svc *SubtitleService) SaveAllSubtitles(scenes []*domain.Scene, projectID, projectPath string, maxWordsPerLine int, sceneNums []int) error {
	filtered := filterScenes(scenes, sceneNums)

	var allEntries []SubtitleEntry
	globalIndex := 1

	for _, scene := range filtered {
		entries := svc.GenerateSubtitles(scene, maxWordsPerLine)
		// Re-index for combined file
		for i := range entries {
			entries[i].Index = globalIndex
			globalIndex++
		}
		allEntries = append(allEntries, entries...)

		if _, err := svc.SaveSceneSubtitles(scene, projectID, projectPath, maxWordsPerLine); err != nil {
			return err
		}
	}

	// Save combined project-level subtitle file (AC3)
	if len(allEntries) > 0 {
		combinedJSON, err := json.MarshalIndent(allEntries, "", "  ")
		if err != nil {
			return fmt.Errorf("subtitle: marshal combined: %w", err)
		}
		combinedJSONPath := filepath.Join(projectPath, "subtitles.json")
		if err := workspace.WriteFileAtomic(combinedJSONPath, combinedJSON); err != nil {
			return fmt.Errorf("subtitle: save combined json: %w", err)
		}

		combinedSRT := FormatSRT(allEntries)
		combinedSRTPath := filepath.Join(projectPath, "subtitles.srt")
		if err := workspace.WriteFileAtomic(combinedSRTPath, []byte(combinedSRT)); err != nil {
			return fmt.Errorf("subtitle: save combined srt: %w", err)
		}

		svc.logger.Info("combined subtitles saved",
			"project_id", projectID,
			"total_entries", len(allEntries),
		)
	}

	return nil
}

// canonicalSpelling applies glossary canonical spelling to a word (AC2).
func (svc *SubtitleService) canonicalSpelling(word string) string {
	if svc.glossary == nil {
		return word
	}
	entry, ok := svc.glossary.Lookup(word)
	if ok {
		return entry.Term // Use canonical term from glossary
	}
	return word
}

// FormatSRT formats subtitle entries as SRT content.
func FormatSRT(entries []SubtitleEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("%d\n", e.Index))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", formatSRTTime(e.StartSec), formatSRTTime(e.EndSec)))
		sb.WriteString(fmt.Sprintf("%s\n\n", e.Text))
	}
	return sb.String()
}

func formatSRTTime(sec float64) string {
	hours := int(sec) / 3600
	minutes := (int(sec) % 3600) / 60
	seconds := int(sec) % 60
	millis := int((sec - float64(int(sec))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

// filterScenes filters scenes to only include specified scene numbers.
func filterScenes(scenes []*domain.Scene, sceneNums []int) []*domain.Scene {
	if len(sceneNums) == 0 {
		return scenes
	}
	wanted := make(map[int]bool, len(sceneNums))
	for _, n := range sceneNums {
		wanted[n] = true
	}
	filtered := make([]*domain.Scene, 0, len(sceneNums))
	for _, s := range scenes {
		if wanted[s.SceneNum] {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// updateManifestSubtitleHash updates the scene manifest with the subtitle hash.
func (svc *SubtitleService) updateManifestSubtitleHash(projectID string, sceneNum int, subtitleHash string) {
	manifest, err := svc.store.GetManifest(projectID, sceneNum)
	if err != nil {
		manifest = &domain.SceneManifest{
			ProjectID:    projectID,
			SceneNum:     sceneNum,
			SubtitleHash: subtitleHash,
			Status:       "subtitle_generated",
		}
		if createErr := svc.store.CreateManifest(manifest); createErr != nil {
			svc.logger.Error("failed to create manifest", "project_id", projectID, "scene_num", sceneNum, "err", createErr)
		}
		return
	}
	manifest.SubtitleHash = subtitleHash
	manifest.Status = "subtitle_generated"
	if updateErr := svc.store.UpdateManifest(manifest); updateErr != nil {
		svc.logger.Error("failed to update manifest", "project_id", projectID, "scene_num", sceneNum, "err", updateErr)
	}
}
