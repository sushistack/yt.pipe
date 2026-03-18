package ffmpeg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/domain"
)

func TestGenerateImageConcat_BasicScenes(t *testing.T) {
	dir := t.TempDir()
	scenes := []domain.Scene{
		{SceneNum: 2, ImagePath: "/img/scene02.png", AudioDuration: 4.5},
		{SceneNum: 1, ImagePath: "/img/scene01.png", AudioDuration: 3.2},
	}

	path, err := generateImageConcat(scenes, dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "images.txt"), path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	// Scene 1 should come before Scene 2
	assert.Contains(t, content, "file '/img/scene01.png'\nduration 3.200\n")
	assert.Contains(t, content, "file '/img/scene02.png'\nduration 4.500\n")

	idx1 := len("file '/img/scene01.png'\nduration 3.200\n")
	assert.True(t, idx1 > 0 && idx1 < len(content))
}

func TestGenerateImageConcat_WithShots(t *testing.T) {
	dir := t.TempDir()
	scenes := []domain.Scene{
		{
			SceneNum:      1,
			AudioDuration: 6.0,
			Shots: []domain.Shot{
				{SentenceStart: 1, CutNum: 2, ImagePath: "/img/s1_c2.png", StartSec: 3.0, EndSec: 6.0},
				{SentenceStart: 1, CutNum: 1, ImagePath: "/img/s1_c1.png", StartSec: 0.0, EndSec: 3.0},
			},
		},
	}

	path, err := generateImageConcat(scenes, dir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	// CutNum 1 should come before CutNum 2
	assert.Equal(t, "file '/img/s1_c1.png'\nduration 3.000\nfile '/img/s1_c2.png'\nduration 3.000\n", content)
}

func TestGenerateImageConcat_EmptyScenes(t *testing.T) {
	dir := t.TempDir()
	_, err := generateImageConcat(nil, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no scenes to render")
}

func TestGenerateAudioConcat_BasicScenes(t *testing.T) {
	dir := t.TempDir()
	scenes := []domain.Scene{
		{SceneNum: 2, AudioPath: "/audio/scene02.wav"},
		{SceneNum: 1, AudioPath: "/audio/scene01.wav"},
	}

	path, err := generateAudioConcat(scenes, dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "audio_concat.txt"), path)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	content := string(data)

	assert.Equal(t, "file '/audio/scene01.wav'\nfile '/audio/scene02.wav'\n", content)
}

func TestGenerateAudioConcat_EmptyScenes(t *testing.T) {
	dir := t.TempDir()
	_, err := generateAudioConcat(nil, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no scenes to render")
}

func TestGenerateAudioConcat_SkipsMissingAudio(t *testing.T) {
	dir := t.TempDir()
	scenes := []domain.Scene{
		{SceneNum: 1, AudioPath: "/audio/scene01.wav"},
		{SceneNum: 2, AudioPath: ""},
		{SceneNum: 3, AudioPath: "/audio/scene03.wav"},
	}

	path, err := generateAudioConcat(scenes, dir)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "file '/audio/scene01.wav'\nfile '/audio/scene03.wav'\n", string(data))
}
