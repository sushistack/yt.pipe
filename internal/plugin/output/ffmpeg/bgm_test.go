package ffmpeg

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/plugin/output"
)

func TestGenerateBGMFilter_Empty(t *testing.T) {
	result := generateBGMFilter(nil, nil, 60.0)
	assert.Empty(t, result.filterComplex)
	assert.Nil(t, result.inputFiles)
}

func TestGenerateBGMFilter_SingleTrack(t *testing.T) {
	assignments := []output.BGMAssignment{
		{
			SceneNum:  1,
			FilePath:  "/bgm/track1.mp3",
			VolumeDB:  -6.0,
			FadeInMs:  3000,
			FadeOutMs: 2000,
			DuckingDB: -12.0,
		},
	}

	result := generateBGMFilter(assignments, nil, 30.0)
	require.NotEmpty(t, result.filterComplex)
	assert.Equal(t, []string{"/bgm/track1.mp3"}, result.inputFiles)

	// Check volume adjustment
	assert.Contains(t, result.filterComplex, "volume=-6.0dB")
	// Check fade-in (3s)
	assert.Contains(t, result.filterComplex, "afade=t=in:st=0:d=3.00")
	// Check fade-out (2s, starting at 28s)
	assert.Contains(t, result.filterComplex, "afade=t=out:st=28.00:d=2.00")
	// Check output label
	assert.Contains(t, result.filterComplex, "[bgm_0]")
}

func TestGenerateBGMFilter_DefaultFade(t *testing.T) {
	assignments := []output.BGMAssignment{
		{
			FilePath: "/bgm/track1.mp3",
			VolumeDB: -3.0,
			// FadeInMs: 0 → default 2000
			// FadeOutMs: 0 → default 2000
		},
	}

	result := generateBGMFilter(assignments, nil, 20.0)
	assert.Contains(t, result.filterComplex, "afade=t=in:st=0:d=2.00")
	assert.Contains(t, result.filterComplex, "afade=t=out:st=18.00:d=2.00")
}

func TestGenerateBGMFilter_WithDucking(t *testing.T) {
	assignments := []output.BGMAssignment{
		{
			FilePath:  "/bgm/track1.mp3",
			VolumeDB:  -6.0,
			DuckingDB: -12.0,
		},
	}
	narrationDurations := map[int]float64{
		1: 3.0,
		2: 4.0,
	}

	result := generateBGMFilter(assignments, narrationDurations, 10.0)
	assert.Contains(t, result.filterComplex, "between(t")
	assert.Contains(t, result.filterComplex, "eval=frame")
}

func TestGenerateBGMFilter_MultipleTracks(t *testing.T) {
	assignments := []output.BGMAssignment{
		{FilePath: "/bgm/track1.mp3", VolumeDB: -6.0},
		{FilePath: "/bgm/track2.mp3", VolumeDB: -8.0},
	}

	result := generateBGMFilter(assignments, nil, 30.0)
	assert.Contains(t, result.filterComplex, "[bgm_0]")
	assert.Contains(t, result.filterComplex, "[bgm_1]")
	assert.Contains(t, result.filterComplex, "amix=inputs=2:duration=longest[bgm_mixed]")
	assert.Len(t, result.inputFiles, 2)
}

func TestDbToRatio(t *testing.T) {
	// -6dB ≈ 0.5
	assert.InDelta(t, 0.5012, dbToRatio(-6.0), 0.001)
	// -12dB ≈ 0.251
	assert.InDelta(t, 0.2512, dbToRatio(-12.0), 0.001)
	// 0dB = 1.0
	assert.InDelta(t, 1.0, dbToRatio(0.0), 0.001)
	// -20dB = 0.1
	assert.InDelta(t, 0.1, dbToRatio(-20.0), 0.001)
}

func TestFormatFloat(t *testing.T) {
	assert.Equal(t, "0.251189", formatFloat(math.Pow(10, -12.0/20.0)))
	assert.Equal(t, "1", formatFloat(1.0))
	assert.Equal(t, "0.5", formatFloat(0.5))
}
