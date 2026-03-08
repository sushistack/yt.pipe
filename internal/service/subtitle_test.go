package service

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSubtitleService(t *testing.T, g *glossary.Glossary) (*SubtitleService, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewSubtitleService(g, s, logger), s
}

func TestGenerateSubtitles_Basic(t *testing.T) {
	svc, _ := newTestSubtitleService(t, glossary.New())

	scene := &domain.Scene{
		SceneNum: 1,
		WordTimings: []domain.WordTiming{
			{Word: "Hello", StartSec: 0.0, EndSec: 0.5},
			{Word: "world", StartSec: 0.5, EndSec: 1.0},
			{Word: "this", StartSec: 1.0, EndSec: 1.3},
			{Word: "is", StartSec: 1.3, EndSec: 1.5},
			{Word: "a", StartSec: 1.5, EndSec: 1.6},
			{Word: "test", StartSec: 1.6, EndSec: 2.0},
		},
	}

	entries := svc.GenerateSubtitles(scene, 3)
	assert.Len(t, entries, 2)
	assert.Equal(t, "Hello world this", entries[0].Text)
	assert.Equal(t, 0.0, entries[0].StartSec)
	assert.Equal(t, 1.3, entries[0].EndSec)
	assert.Equal(t, "is a test", entries[1].Text)
}

func TestGenerateSubtitles_Empty(t *testing.T) {
	svc, _ := newTestSubtitleService(t, glossary.New())
	scene := &domain.Scene{SceneNum: 1}
	entries := svc.GenerateSubtitles(scene, 8)
	assert.Nil(t, entries)
}

func TestGenerateSubtitles_DefaultMaxWords(t *testing.T) {
	svc, _ := newTestSubtitleService(t, glossary.New())
	var timings []domain.WordTiming
	for i := 0; i < 20; i++ {
		timings = append(timings, domain.WordTiming{
			Word:     "word",
			StartSec: float64(i) * 0.5,
			EndSec:   float64(i)*0.5 + 0.5,
		})
	}
	scene := &domain.Scene{SceneNum: 1, WordTimings: timings}

	entries := svc.GenerateSubtitles(scene, 0) // 0 = default 8
	assert.Len(t, entries, 3)                  // 8+8+4
}

func TestGenerateSubtitles_WithGlossary(t *testing.T) {
	dir := t.TempDir()
	gPath := filepath.Join(dir, "glossary.json")
	require.NoError(t, glossary.WriteToFile(gPath, []glossary.Entry{
		{Term: "SCP-173", Pronunciation: "ess see pee"},
	}))
	g := glossary.LoadFromFile(gPath)
	svc, _ := newTestSubtitleService(t, g)

	scene := &domain.Scene{
		SceneNum: 1,
		WordTimings: []domain.WordTiming{
			{Word: "scp-173", StartSec: 0.0, EndSec: 0.5},
			{Word: "is", StartSec: 0.5, EndSec: 0.8},
		},
	}

	entries := svc.GenerateSubtitles(scene, 8)
	require.Len(t, entries, 1)
	// AC2: Should use canonical spelling from glossary
	assert.Equal(t, "SCP-173 is", entries[0].Text)
}

func TestFormatSRT(t *testing.T) {
	entries := []SubtitleEntry{
		{Index: 1, StartSec: 0.0, EndSec: 2.5, Text: "Hello world"},
		{Index: 2, StartSec: 2.5, EndSec: 5.123, Text: "Second line"},
	}

	srt := FormatSRT(entries)
	assert.Contains(t, srt, "1\n00:00:00,000 --> 00:00:02,500\nHello world")
	assert.Contains(t, srt, "2\n00:00:02,500 --> 00:00:05,123\nSecond line")
}

func TestFormatSRTTime(t *testing.T) {
	assert.Equal(t, "00:00:00,000", formatSRTTime(0))
	assert.Equal(t, "00:01:30,500", formatSRTTime(90.5))
	assert.Equal(t, "01:00:00,000", formatSRTTime(3600))
}

func TestSaveSceneSubtitles_JSON(t *testing.T) {
	svc, st := newTestSubtitleService(t, glossary.New())
	projectPath := t.TempDir()
	projectID := "sub-proj"
	createTestProject(t, st, projectID)

	scene := &domain.Scene{
		SceneNum: 1,
		WordTimings: []domain.WordTiming{
			{Word: "Hello", StartSec: 0.0, EndSec: 0.5},
			{Word: "world", StartSec: 0.5, EndSec: 1.0},
		},
	}

	path, err := svc.SaveSceneSubtitles(scene, projectID, projectPath, 8)
	require.NoError(t, err)

	// AC1: Verify subtitle.json saved
	assert.Contains(t, path, "subtitle.json")
	assert.FileExists(t, path)

	// Verify JSON is valid
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var entries []SubtitleEntry
	require.NoError(t, json.Unmarshal(data, &entries))
	assert.Len(t, entries, 1)

	// Also verify SRT file
	srtPath := filepath.Join(projectPath, "scenes", "1", "subtitle.srt")
	assert.FileExists(t, srtPath)

	// Verify manifest updated
	manifest, err := st.GetManifest(projectID, 1)
	require.NoError(t, err)
	assert.NotEmpty(t, manifest.SubtitleHash)
}

func TestSaveAllSubtitles_CombinedFile(t *testing.T) {
	svc, st := newTestSubtitleService(t, glossary.New())
	projectPath := t.TempDir()
	projectID := "sub-combined"
	createTestProject(t, st, projectID)

	scenes := []*domain.Scene{
		{
			SceneNum: 1,
			WordTimings: []domain.WordTiming{
				{Word: "Scene", StartSec: 0.0, EndSec: 0.5},
				{Word: "one", StartSec: 0.5, EndSec: 1.0},
			},
		},
		{
			SceneNum: 2,
			WordTimings: []domain.WordTiming{
				{Word: "Scene", StartSec: 1.0, EndSec: 1.5},
				{Word: "two", StartSec: 1.5, EndSec: 2.0},
			},
		},
	}

	err := svc.SaveAllSubtitles(scenes, projectID, projectPath, 8, nil)
	require.NoError(t, err)

	// AC3: Verify combined project-level files
	assert.FileExists(t, filepath.Join(projectPath, "subtitles.json"))
	assert.FileExists(t, filepath.Join(projectPath, "subtitles.srt"))

	// Verify combined JSON
	data, err := os.ReadFile(filepath.Join(projectPath, "subtitles.json"))
	require.NoError(t, err)
	var entries []SubtitleEntry
	require.NoError(t, json.Unmarshal(data, &entries))
	assert.Len(t, entries, 2)
	// Global index re-numbering
	assert.Equal(t, 1, entries[0].Index)
	assert.Equal(t, 2, entries[1].Index)
}

func TestSaveAllSubtitles_SceneFilter(t *testing.T) {
	svc, st := newTestSubtitleService(t, glossary.New())
	projectPath := t.TempDir()
	projectID := "sub-filter"
	createTestProject(t, st, projectID)

	scenes := []*domain.Scene{
		{SceneNum: 1, WordTimings: []domain.WordTiming{{Word: "one", StartSec: 0, EndSec: 0.5}}},
		{SceneNum: 2, WordTimings: []domain.WordTiming{{Word: "two", StartSec: 0.5, EndSec: 1.0}}},
	}

	// AC4: Only regenerate scene 2
	err := svc.SaveAllSubtitles(scenes, projectID, projectPath, 8, []int{2})
	require.NoError(t, err)

	// Only scene 2 should have subtitle file
	assert.NoFileExists(t, filepath.Join(projectPath, "scenes", "1", "subtitle.json"))
	assert.FileExists(t, filepath.Join(projectPath, "scenes", "2", "subtitle.json"))
}
