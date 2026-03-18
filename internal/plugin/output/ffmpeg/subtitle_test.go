package ffmpeg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/domain"
)

func TestFormatSRTTime(t *testing.T) {
	tests := []struct {
		sec      float64
		expected string
	}{
		{0, "00:00:00,000"},
		{1.5, "00:00:01,500"},
		{61.234, "00:01:01,234"},
		{3661.001, "01:01:01,001"},
		{-1.0, "00:00:00,000"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatSRTTime(tt.sec))
	}
}

func TestGroupWordTimings(t *testing.T) {
	timings := []domain.WordTiming{
		{Word: "SCP", StartSec: 0.0, EndSec: 0.5},
		{Word: "173은", StartSec: 0.5, EndSec: 1.0},
		{Word: "조각상이다", StartSec: 1.0, EndSec: 1.8},
	}

	entries := groupWordTimings(timings, 0, 2)
	require.Len(t, entries, 2)

	assert.Equal(t, "SCP 173은", entries[0].text)
	assert.InDelta(t, 0.0, entries[0].startSec, 0.001)
	assert.InDelta(t, 1.0, entries[0].endSec, 0.001)

	assert.Equal(t, "조각상이다", entries[1].text)
	assert.InDelta(t, 1.0, entries[1].startSec, 0.001)
	assert.InDelta(t, 1.8, entries[1].endSec, 0.001)
}

func TestGroupWordTimings_WithOffset(t *testing.T) {
	timings := []domain.WordTiming{
		{Word: "hello", StartSec: 0.0, EndSec: 0.5},
	}

	entries := groupWordTimings(timings, 10.0, 8)
	require.Len(t, entries, 1)
	assert.InDelta(t, 10.0, entries[0].startSec, 0.001)
	assert.InDelta(t, 10.5, entries[0].endSec, 0.001)
}

func TestGroupWordTimings_Empty(t *testing.T) {
	entries := groupWordTimings(nil, 0, 8)
	assert.Nil(t, entries)
}

func TestGenerateSRT_BasicScenes(t *testing.T) {
	dir := t.TempDir()
	scenes := []domain.Scene{
		{
			SceneNum:      1,
			AudioDuration: 2.0,
			WordTimings: []domain.WordTiming{
				{Word: "SCP", StartSec: 0.0, EndSec: 0.5},
				{Word: "173", StartSec: 0.5, EndSec: 1.0},
			},
		},
		{
			SceneNum:      2,
			AudioDuration: 3.0,
			WordTimings: []domain.WordTiming{
				{Word: "눈을", StartSec: 0.0, EndSec: 0.8},
				{Word: "감지마세요", StartSec: 0.8, EndSec: 1.5},
			},
		},
	}

	path, err := generateSRT(scenes, dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "subtitles.srt"), path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	// Scene 1 subtitle
	assert.Contains(t, content, "1\n00:00:00,000 --> 00:00:01,000\nSCP 173\n")
	// Scene 2 subtitle: offset by scene 1 AudioDuration (2.0s)
	assert.Contains(t, content, "2\n00:00:02,000 --> 00:00:03,500\n눈을 감지마세요\n")
}

func TestGenerateSRT_EmptyScenes(t *testing.T) {
	dir := t.TempDir()
	_, err := generateSRT(nil, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no scenes to render")
}

func TestGenerateSRT_ScenesWithNoWordTimings(t *testing.T) {
	dir := t.TempDir()
	scenes := []domain.Scene{
		{SceneNum: 1, AudioDuration: 3.0, WordTimings: nil},
	}

	path, err := generateSRT(scenes, dir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	// Empty SRT is valid (no entries)
	assert.Empty(t, string(data))
}
