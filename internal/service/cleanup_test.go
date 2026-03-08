package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestWorkspace(t *testing.T) string {
	dir := t.TempDir()

	// Create scene directories with files
	scene1 := filepath.Join(dir, "scenes", "1")
	require.NoError(t, os.MkdirAll(scene1, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(scene1, "image.png"), make([]byte, 1024), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(scene1, "audio.mp3"), make([]byte, 2048), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(scene1, "subtitle.json"), make([]byte, 512), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(scene1, "prompt.txt"), make([]byte, 256), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(scene1, "manifest.json"), make([]byte, 128), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(scene1, "timing.json"), make([]byte, 64), 0o644))

	// Create scenario and output
	require.NoError(t, os.WriteFile(filepath.Join(dir, "scenario.json"), make([]byte, 4096), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "checkpoint.json"), make([]byte, 128), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), make([]byte, 256), 0o644))

	outDir := filepath.Join(dir, "output")
	require.NoError(t, os.MkdirAll(outDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outDir, "draft.json"), make([]byte, 8192), 0o644))

	return dir
}

func TestCleanProject_IntermediatesOnly(t *testing.T) {
	dir := setupTestWorkspace(t)

	result, err := CleanProject(dir, "", false, false)
	require.NoError(t, err)

	assert.Greater(t, len(result.FilesRemoved), 0)
	assert.Greater(t, result.BytesFreed, int64(0))
	assert.False(t, result.DryRun)

	// Final outputs should be preserved
	assert.FileExists(t, filepath.Join(dir, "scenario.json"))
	assert.FileExists(t, filepath.Join(dir, "output", "draft.json"))
	assert.FileExists(t, filepath.Join(dir, "scenes", "1", "image.png"))
	assert.FileExists(t, filepath.Join(dir, "scenes", "1", "audio.mp3"))

	// Intermediates should be gone
	assert.NoFileExists(t, filepath.Join(dir, "checkpoint.json"))
	assert.NoFileExists(t, filepath.Join(dir, "scenes", "1", "prompt.txt"))
	assert.NoFileExists(t, filepath.Join(dir, "scenes", "1", "timing.json"))
}

func TestCleanProject_DryRun(t *testing.T) {
	dir := setupTestWorkspace(t)

	result, err := CleanProject(dir, "", false, true)
	require.NoError(t, err)

	assert.True(t, result.DryRun)
	assert.Greater(t, len(result.FilesRemoved), 0)

	// Nothing should actually be deleted
	assert.FileExists(t, filepath.Join(dir, "checkpoint.json"))
	assert.FileExists(t, filepath.Join(dir, "scenes", "1", "prompt.txt"))
}

func TestCleanProject_All(t *testing.T) {
	dir := setupTestWorkspace(t)

	result, err := CleanProject(dir, "", true, false)
	require.NoError(t, err)

	assert.Greater(t, len(result.FilesRemoved), 0)
	assert.Greater(t, result.BytesFreed, int64(0))

	// Workspace should be completely removed
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err))
}

func TestCleanProject_AllDryRun(t *testing.T) {
	dir := setupTestWorkspace(t)

	result, err := CleanProject(dir, "", true, true)
	require.NoError(t, err)

	assert.True(t, result.DryRun)
	assert.Greater(t, len(result.FilesRemoved), 0)

	// Workspace should still exist
	assert.DirExists(t, dir)
}

func TestCleanProject_NonexistentPath(t *testing.T) {
	_, err := CleanProject("/nonexistent/path", "", false, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestGetDiskUsage(t *testing.T) {
	dir := setupTestWorkspace(t)

	usage, err := GetDiskUsage(dir)
	require.NoError(t, err)

	assert.Greater(t, usage.TotalBytes, int64(0))
	assert.Greater(t, usage.TotalFiles, 0)
	assert.NotEmpty(t, usage.Categories)

	// Check categories exist
	catMap := make(map[string]CategoryUsage)
	for _, c := range usage.Categories {
		catMap[c.Category] = c
	}
	assert.Contains(t, catMap, "images")
	assert.Contains(t, catMap, "audio")
	assert.Contains(t, catMap, "output")
}

func TestGetDiskUsage_NonexistentPath(t *testing.T) {
	usage, err := GetDiskUsage("/nonexistent/path")
	require.NoError(t, err) // returns empty usage, not error
	assert.Zero(t, usage.TotalBytes)
}

func TestCategorizeFile(t *testing.T) {
	assert.Equal(t, "images", categorizeFile("scenes/1/image.png", "image.png"))
	assert.Equal(t, "audio", categorizeFile("scenes/1/audio.mp3", "audio.mp3"))
	assert.Equal(t, "subtitles", categorizeFile("scenes/1/subtitle.json", "subtitle.json"))
	assert.Equal(t, "subtitles", categorizeFile("scenes/1/subtitle.srt", "subtitle.srt"))
	assert.Equal(t, "scenario", categorizeFile("scenario.json", "scenario.json"))
	assert.Equal(t, "output", categorizeFile("output/draft.json", "draft.json"))
	assert.Equal(t, "other", categorizeFile("checkpoint.json", "checkpoint.json"))
	// matchPrefix should not match "outputfoo" as "output"
	assert.Equal(t, "other", categorizeFile("outputfoo/bar.txt", "bar.txt"))
}

func TestCleanProject_ContainmentValidation(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "project1")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	// Valid: path inside root
	_, err := CleanProject(dir, root, false, true)
	assert.NoError(t, err)

	// Invalid: path outside root
	outside := t.TempDir()
	_, err = CleanProject(outside, root, false, true)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outside workspace root")
}

func TestMatchPrefix(t *testing.T) {
	assert.True(t, matchPrefix("output/file.json", "output"))
	assert.True(t, matchPrefix("output", "output"))
	assert.False(t, matchPrefix("outputfoo/file.json", "output"))
	assert.False(t, matchPrefix("other/file.json", "output"))
}
