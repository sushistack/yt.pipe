package ffmpeg

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/config"
)

func TestCheckFFmpegAvailable_ErrorMessage(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	_, err := checkFFmpegAvailable()
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "ffmpeg binary not found in PATH"))
	assert.True(t, strings.Contains(err.Error(), "install ffmpeg or use Docker image with ffmpeg included"))
}

func TestApplyDefaults(t *testing.T) {
	cfg := applyDefaults(config.FFmpegConfig{})
	assert.Equal(t, "medium", cfg.Preset)
	assert.Equal(t, 23, cfg.CRF)
	assert.Equal(t, "192k", cfg.AudioBitrate)
	assert.Equal(t, "1920x1080", cfg.Resolution)
	assert.Equal(t, 30, cfg.FPS)
	assert.Equal(t, 24, cfg.SubtitleFontSize)
}

func TestApplyDefaults_PreservesCustomValues(t *testing.T) {
	cfg := applyDefaults(config.FFmpegConfig{
		Preset:         "fast",
		CRF:            18,
		AudioBitrate:   "256k",
		Resolution:     "1280x720",
		FPS:            60,
		SubtitleFontSize: 32,
	})
	assert.Equal(t, "fast", cfg.Preset)
	assert.Equal(t, 18, cfg.CRF)
	assert.Equal(t, "256k", cfg.AudioBitrate)
	assert.Equal(t, "1280x720", cfg.Resolution)
	assert.Equal(t, 60, cfg.FPS)
	assert.Equal(t, 32, cfg.SubtitleFontSize)
}

func TestBuildFFmpegArgs_Basic(t *testing.T) {
	a := &Assembler{
		cfg: applyDefaults(config.FFmpegConfig{}),
	}

	args := a.buildFFmpegArgs(
		"/tmp/images.txt",
		"/tmp/audio_concat.txt",
		"",
		false,
		bgmFilterResult{},
		"/tmp/output.mp4",
	)

	assert.Contains(t, args, "-y")
	assert.Contains(t, args, "libx264")
	assert.Contains(t, args, "medium")
	assert.Contains(t, args, "23")
	assert.Contains(t, args, "aac")
	assert.Contains(t, args, "192k")
	assert.Contains(t, args, "1920x1080")
	assert.Contains(t, args, "/tmp/output.mp4")
	// No subtitles → no -vf with subtitles
	for i, arg := range args {
		if arg == "-vf" {
			t.Errorf("should not have -vf when no subtitles, got: %s", args[i+1])
		}
	}
}

func TestBuildFFmpegArgs_WithSubtitles(t *testing.T) {
	a := &Assembler{
		cfg: applyDefaults(config.FFmpegConfig{}),
	}

	args := a.buildFFmpegArgs(
		"/tmp/images.txt",
		"/tmp/audio_concat.txt",
		"/tmp/subtitles.srt",
		true,
		bgmFilterResult{},
		"/tmp/output.mp4",
	)

	hasVF := false
	for i, arg := range args {
		if arg == "-vf" {
			hasVF = true
			assert.Contains(t, args[i+1], "subtitles=")
			assert.Contains(t, args[i+1], "FontSize=24")
		}
	}
	assert.True(t, hasVF, "should have -vf flag for subtitles")
}

func TestBuildFFmpegArgs_WithBGM(t *testing.T) {
	a := &Assembler{
		cfg: applyDefaults(config.FFmpegConfig{}),
	}

	bgm := bgmFilterResult{
		filterComplex: "[bgm_in_0]volume=-6.0dB[bgm_0]",
		inputFiles:    []string{"/bgm/track.mp3"},
	}

	args := a.buildFFmpegArgs(
		"/tmp/images.txt",
		"/tmp/audio_concat.txt",
		"",
		false,
		bgm,
		"/tmp/output.mp4",
	)

	assert.Contains(t, args, "-i")
	assert.Contains(t, args, "/bgm/track.mp3")
	assert.Contains(t, args, "-filter_complex")
	assert.Contains(t, args, "-map")
}
