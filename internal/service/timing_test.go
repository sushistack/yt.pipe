package service

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestTimingResolver(t *testing.T) *TimingResolver {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewTimingResolver(logger)
}

func TestResolveTimings_Basic(t *testing.T) {
	resolver := newTestTimingResolver(t)
	scenes := []*domain.Scene{
		{SceneNum: 1, AudioDuration: 5.0},
		{SceneNum: 2, AudioDuration: 3.0},
		{SceneNum: 3, AudioDuration: 7.0},
	}

	timings := resolver.ResolveTimings(scenes)
	assert.Len(t, timings, 3)

	assert.Equal(t, 0.0, timings[0].StartSec)
	assert.Equal(t, 5.0, timings[0].EndSec)
	assert.Equal(t, 5.0, timings[0].DurationSec)
	assert.Equal(t, 5.0, timings[0].TransitionPoint)

	assert.Equal(t, 5.0, timings[1].StartSec)
	assert.Equal(t, 8.0, timings[1].EndSec)
	assert.Equal(t, 8.0, timings[1].TransitionPoint)

	assert.Equal(t, 8.0, timings[2].StartSec)
	assert.Equal(t, 15.0, timings[2].EndSec)
}

func TestResolveTimings_WithWordTimings(t *testing.T) {
	resolver := newTestTimingResolver(t)
	scenes := []*domain.Scene{
		{
			SceneNum:      1,
			AudioDuration: 2.0,
			WordTimings: []domain.WordTiming{
				{Word: "Hello", StartSec: 0.0, EndSec: 0.5},
				{Word: "world", StartSec: 0.5, EndSec: 1.0},
			},
		},
		{
			SceneNum:      2,
			AudioDuration: 3.0,
			WordTimings: []domain.WordTiming{
				{Word: "Goodbye", StartSec: 0.0, EndSec: 0.8},
			},
		},
	}

	timings := resolver.ResolveTimings(scenes)

	// Scene 1 word timings: absolute times (offset 0)
	assert.Len(t, timings[0].WordTimings, 2)
	assert.Equal(t, 0.0, timings[0].WordTimings[0].StartSec)

	// Scene 2 word timings: offset by scene 1 duration (2.0)
	assert.Len(t, timings[1].WordTimings, 1)
	assert.Equal(t, 2.0, timings[1].WordTimings[0].StartSec)
	assert.Equal(t, 2.8, timings[1].WordTimings[0].EndSec)
}

func TestResolveTimings_SubtitleSegments(t *testing.T) {
	resolver := newTestTimingResolver(t)
	words := make([]domain.WordTiming, 12)
	for i := range words {
		words[i] = domain.WordTiming{
			Word:     "word",
			StartSec: float64(i) * 0.5,
			EndSec:   float64(i)*0.5 + 0.4,
		}
	}

	scenes := []*domain.Scene{
		{SceneNum: 1, AudioDuration: 6.0, WordTimings: words},
	}

	timings := resolver.ResolveTimings(scenes)
	// 12 words / 8 per segment = 2 segments (8 + 4)
	assert.Len(t, timings[0].SubtitleSegments, 2)
	assert.Equal(t, 8, countWords(timings[0].SubtitleSegments[0].Text))
}

func TestResolveTimings_Empty(t *testing.T) {
	resolver := newTestTimingResolver(t)
	timings := resolver.ResolveTimings(nil)
	assert.Len(t, timings, 0)
}

func TestResolveTimings_SingleScene(t *testing.T) {
	resolver := newTestTimingResolver(t)
	scenes := []*domain.Scene{{SceneNum: 1, AudioDuration: 10.5}}
	timings := resolver.ResolveTimings(scenes)
	assert.Len(t, timings, 1)
	assert.Equal(t, 0.0, timings[0].StartSec)
	assert.Equal(t, 10.5, timings[0].EndSec)
}

func TestTotalDuration(t *testing.T) {
	timings := []SceneTiming{
		{SceneNum: 1, StartSec: 0, EndSec: 5, DurationSec: 5},
		{SceneNum: 2, StartSec: 5, EndSec: 12, DurationSec: 7},
	}
	assert.Equal(t, 12.0, TotalDuration(timings))
}

func TestTotalDuration_Empty(t *testing.T) {
	assert.Equal(t, 0.0, TotalDuration(nil))
}

func TestSaveTimingFiles(t *testing.T) {
	resolver := newTestTimingResolver(t)
	projectPath := t.TempDir()

	timings := []SceneTiming{
		{SceneNum: 1, StartSec: 0, EndSec: 5, DurationSec: 5, TransitionPoint: 5},
		{SceneNum: 2, StartSec: 5, EndSec: 10, DurationSec: 5, TransitionPoint: 10},
	}

	err := resolver.SaveTimingFiles(timings, projectPath)
	require.NoError(t, err)

	// Verify per-scene timing.json (AC3)
	assert.FileExists(t, filepath.Join(projectPath, "scenes", "1", "timing.json"))
	assert.FileExists(t, filepath.Join(projectPath, "scenes", "2", "timing.json"))

	// Verify project-level timeline.json (AC3)
	timelinePath := filepath.Join(projectPath, "timeline.json")
	assert.FileExists(t, timelinePath)

	data, err := os.ReadFile(timelinePath)
	require.NoError(t, err)
	var timeline Timeline
	require.NoError(t, json.Unmarshal(data, &timeline))
	assert.Equal(t, 10.0, timeline.TotalDurationSec)
	assert.Equal(t, 2, timeline.SceneCount)
}

func TestBuildTimeline(t *testing.T) {
	resolver := newTestTimingResolver(t)
	timings := []SceneTiming{
		{SceneNum: 1, StartSec: 0, EndSec: 5, DurationSec: 5},
		{SceneNum: 2, StartSec: 5, EndSec: 12, DurationSec: 7},
	}
	timeline := resolver.BuildTimeline(timings)
	assert.Equal(t, 12.0, timeline.TotalDurationSec)
	assert.Equal(t, 2, timeline.SceneCount)
	assert.Len(t, timeline.Scenes, 2)
}

func TestResolveTimings_ZeroDurationUsesDefault(t *testing.T) {
	resolver := newTestTimingResolver(t)
	scenes := []*domain.Scene{
		{SceneNum: 1, AudioDuration: 5.0},
		{SceneNum: 2, AudioDuration: 0}, // no narration
		{SceneNum: 3, AudioDuration: 4.0},
	}

	timings := resolver.ResolveTimings(scenes)
	assert.Len(t, timings, 3)

	// Scene 2 should use default duration (3.0s)
	assert.Equal(t, 5.0, timings[1].StartSec)
	assert.Equal(t, 3.0, timings[1].DurationSec)
	assert.Equal(t, 8.0, timings[1].EndSec)

	// Scene 3 offset should account for default duration
	assert.Equal(t, 8.0, timings[2].StartSec)
	assert.Equal(t, 12.0, timings[2].EndSec)
}

func TestWithDefaultSceneDuration(t *testing.T) {
	resolver := newTestTimingResolver(t)

	// Custom default duration
	resolver = resolver.WithDefaultSceneDuration(5.0)

	scenes := []*domain.Scene{
		{SceneNum: 1, AudioDuration: 0}, // no narration, should use 5.0
	}

	timings := resolver.ResolveTimings(scenes)
	assert.Len(t, timings, 1)
	assert.Equal(t, 5.0, timings[0].DurationSec)
}

func TestWithDefaultSceneDuration_IgnoresInvalid(t *testing.T) {
	resolver := newTestTimingResolver(t)

	// Zero or negative should not change default
	resolver = resolver.WithDefaultSceneDuration(0)
	resolver = resolver.WithDefaultSceneDuration(-1)

	scenes := []*domain.Scene{
		{SceneNum: 1, AudioDuration: 0},
	}

	timings := resolver.ResolveTimings(scenes)
	assert.Equal(t, DefaultSceneDurationSec, timings[0].DurationSec)
}

// --- YouTube Chapters Tests ---

func TestGenerateChapters_MultiScene(t *testing.T) {
	resolver := newTestTimingResolver(t)
	timeline := Timeline{
		TotalDurationSec: 180,
		SceneCount:       3,
		Scenes: []SceneTiming{
			{SceneNum: 1, StartSec: 0, EndSec: 60},
			{SceneNum: 2, StartSec: 60, EndSec: 120},
			{SceneNum: 3, StartSec: 120, EndSec: 180},
		},
	}
	scenes := []domain.SceneScript{
		{SceneNum: 1, Mood: "Eerie", VisualDescription: "A dark corridor stretching endlessly into the void"},
		{SceneNum: 2, Mood: "Tense", VisualDescription: "Guards patrolling the facility"},
		{SceneNum: 3, Mood: "Calm", VisualDescription: "Morning light"},
	}

	chapters := resolver.GenerateChapters(timeline, scenes)
	assert.Len(t, chapters, 3)
	assert.Equal(t, "Intro", chapters[0].Title)
	assert.Equal(t, 0.0, chapters[0].TimestampSec)
	assert.Equal(t, "Tense - Guards patrolling the facility", chapters[1].Title)
	assert.Equal(t, 60.0, chapters[1].TimestampSec)
	assert.Equal(t, "Calm - Morning light", chapters[2].Title)

	// Verify format output
	content := FormatChapters(chapters)
	assert.Contains(t, content, "0:00 Intro\n")
	assert.Contains(t, content, "1:00 Tense - Guards patrolling the facility\n")
	assert.Contains(t, content, "2:00 Calm - Morning light\n")
}

func TestGenerateChapters_SingleScene(t *testing.T) {
	resolver := newTestTimingResolver(t)
	timeline := Timeline{
		TotalDurationSec: 30,
		SceneCount:       1,
		Scenes: []SceneTiming{
			{SceneNum: 1, StartSec: 0, EndSec: 30},
		},
	}
	scenes := []domain.SceneScript{
		{SceneNum: 1, Mood: "Eerie", VisualDescription: "A dark room"},
	}

	chapters := resolver.GenerateChapters(timeline, scenes)
	assert.Len(t, chapters, 1)
	assert.Equal(t, "Intro", chapters[0].Title)

	content := FormatChapters(chapters)
	assert.Equal(t, "0:00 Intro\n", content)
}

func TestGenerateChapters_HourPlusTimestamp(t *testing.T) {
	resolver := newTestTimingResolver(t)
	timeline := Timeline{
		TotalDurationSec: 4000,
		SceneCount:       2,
		Scenes: []SceneTiming{
			{SceneNum: 1, StartSec: 0, EndSec: 3661},
			{SceneNum: 2, StartSec: 3661, EndSec: 4000},
		},
	}
	scenes := []domain.SceneScript{
		{SceneNum: 1, Mood: "Intro Mood"},
		{SceneNum: 2, Mood: "Finale", VisualDescription: "End scene"},
	}

	chapters := resolver.GenerateChapters(timeline, scenes)
	content := FormatChapters(chapters)
	// 3661 sec = 1h 1m 1s
	assert.Contains(t, content, "1:01:01 Finale - End scene\n")
}

func TestGenerateChapters_TitleTruncation(t *testing.T) {
	resolver := newTestTimingResolver(t)
	timeline := Timeline{
		TotalDurationSec: 20,
		SceneCount:       2,
		Scenes: []SceneTiming{
			{SceneNum: 1, StartSec: 0, EndSec: 10},
			{SceneNum: 2, StartSec: 10, EndSec: 20},
		},
	}
	longDesc := "This is a very long visual description that exceeds thirty characters limit"
	scenes := []domain.SceneScript{
		{SceneNum: 1},
		{SceneNum: 2, Mood: "Dark", VisualDescription: longDesc},
	}

	chapters := resolver.GenerateChapters(timeline, scenes)
	// Mood + " - " + first 30 chars of VisualDesc + "…"
	assert.Equal(t, "Dark - This is a very long visual des…", chapters[1].Title)
}

func TestGenerateChapters_EmptyMood(t *testing.T) {
	resolver := newTestTimingResolver(t)
	timeline := Timeline{
		TotalDurationSec: 20,
		SceneCount:       2,
		Scenes: []SceneTiming{
			{SceneNum: 1, StartSec: 0, EndSec: 10},
			{SceneNum: 2, StartSec: 10, EndSec: 20},
		},
	}
	scenes := []domain.SceneScript{
		{SceneNum: 1},
		{SceneNum: 2, VisualDescription: "A quiet room"},
	}

	chapters := resolver.GenerateChapters(timeline, scenes)
	assert.Equal(t, "A quiet room", chapters[1].Title)
}

func TestGenerateChapters_EmptyMoodAndDesc(t *testing.T) {
	resolver := newTestTimingResolver(t)
	timeline := Timeline{
		TotalDurationSec: 20,
		SceneCount:       2,
		Scenes: []SceneTiming{
			{SceneNum: 1, StartSec: 0, EndSec: 10},
			{SceneNum: 2, StartSec: 10, EndSec: 20},
		},
	}
	scenes := []domain.SceneScript{
		{SceneNum: 1},
		{SceneNum: 2},
	}

	chapters := resolver.GenerateChapters(timeline, scenes)
	assert.Equal(t, "Scene 2", chapters[1].Title)
}

func TestSaveChaptersFile(t *testing.T) {
	resolver := newTestTimingResolver(t)
	projectPath := t.TempDir()

	content := "0:00 Intro\n1:23 Second scene\n"
	err := resolver.SaveChaptersFile(content, projectPath)
	require.NoError(t, err)

	outputPath := filepath.Join(projectPath, "output", "chapters.txt")
	assert.FileExists(t, outputPath)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		sec      float64
		expected string
	}{
		{0, "0:00"},
		{5, "0:05"},
		{65, "1:05"},
		{600, "10:00"},
		{3599, "59:59"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
		{7325, "2:02:05"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatTimestamp(tt.sec))
		})
	}
}

// countWords counts space-separated words in text.
func countWords(text string) int {
	count := 0
	inWord := false
	for _, c := range text {
		if c == ' ' {
			inWord = false
		} else if !inWord {
			count++
			inWord = true
		}
	}
	return count
}
