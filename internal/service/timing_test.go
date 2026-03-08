package service

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jay/youtube-pipeline/internal/domain"
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
