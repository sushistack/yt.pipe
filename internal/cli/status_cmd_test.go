package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckAsset_Found(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "image.png")
	err := os.WriteFile(imgPath, []byte("fake image data"), 0644)
	assert.NoError(t, err)

	path, status := checkAsset(dir, "image.png", "image.jpg")
	assert.Equal(t, imgPath, path)
	assert.Equal(t, "generated", status)
}

func TestCheckAsset_NotFound(t *testing.T) {
	dir := t.TempDir()
	path, status := checkAsset(dir, "image.png")
	assert.Empty(t, path)
	assert.Equal(t, "pending", status)
}

func TestCheckAsset_Empty(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "image.png")
	err := os.WriteFile(imgPath, []byte{}, 0644)
	assert.NoError(t, err)

	_, status := checkAsset(dir, "image.png")
	assert.Equal(t, "empty", status)
}

func TestCollectSceneStatuses_NoScenesDir(t *testing.T) {
	dir := t.TempDir()
	scenes := collectSceneStatuses(dir, 3)
	assert.Len(t, scenes, 3)
	assert.Equal(t, "pending", scenes[0].ImageStatus)
}

func TestCollectSceneStatuses_WithScenes(t *testing.T) {
	dir := t.TempDir()
	sceneDir := filepath.Join(dir, "scenes", "1")
	err := os.MkdirAll(sceneDir, 0755)
	assert.NoError(t, err)

	// Create an image file
	err = os.WriteFile(filepath.Join(sceneDir, "image.png"), []byte("data"), 0644)
	assert.NoError(t, err)

	scenes := collectSceneStatuses(dir, 1)
	assert.Len(t, scenes, 1)
	assert.Equal(t, 1, scenes[0].SceneNum)
	assert.Equal(t, "generated", scenes[0].ImageStatus)
	assert.Equal(t, "pending", scenes[0].AudioStatus)
}

func TestTruncatePrompt(t *testing.T) {
	assert.Equal(t, "short", truncatePrompt("short", 10))
	assert.Equal(t, "longst...", truncatePrompt("longstring here", 6))
}
