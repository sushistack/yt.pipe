package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitProject_CreatesDirectoryStructure(t *testing.T) {
	base := t.TempDir()

	projectPath, err := InitProject(base, "SCP-173")
	require.NoError(t, err)

	assert.DirExists(t, projectPath)
	assert.DirExists(t, filepath.Join(projectPath, "scenes"))
	assert.Contains(t, projectPath, "SCP-173-")
}

func TestInitSceneDir_CreatesSceneDirectory(t *testing.T) {
	base := t.TempDir()

	projectPath, err := InitProject(base, "SCP-173")
	require.NoError(t, err)

	sceneDir, err := InitSceneDir(projectPath, 1)
	require.NoError(t, err)
	assert.DirExists(t, sceneDir)
	assert.Contains(t, sceneDir, filepath.Join("scenes", "1"))

	sceneDir2, err := InitSceneDir(projectPath, 5)
	require.NoError(t, err)
	assert.DirExists(t, sceneDir2)
}

func TestWriteFileAtomic_Success(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	err := WriteFileAtomic(path, []byte(`{"key":"value"}`))
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, `{"key":"value"}`, string(data))
}

func TestWriteFileAtomic_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "deep", "test.txt")

	err := WriteFileAtomic(path, []byte("hello"))
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestWriteFileAtomic_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	require.NoError(t, WriteFileAtomic(path, []byte("first")))
	require.NoError(t, WriteFileAtomic(path, []byte("second")))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "second", string(data))
}

func TestProjectExists(t *testing.T) {
	base := t.TempDir()

	projectPath, err := InitProject(base, "SCP-173")
	require.NoError(t, err)

	assert.True(t, ProjectExists(projectPath))
	assert.False(t, ProjectExists(filepath.Join(base, "nonexistent")))
}
