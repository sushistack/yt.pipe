package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// InitProject creates the project workspace directory structure.
// Layout: {basePath}/{scpID}-{timestamp}/scenes/
// Returns the full project workspace path.
func InitProject(basePath, scpID string) (string, error) {
	ts := time.Now().UTC().Format("20060102-150405")
	projectDir := filepath.Join(basePath, fmt.Sprintf("%s-%s", scpID, ts))

	scenesDir := filepath.Join(projectDir, "scenes")
	if err := os.MkdirAll(scenesDir, 0o755); err != nil {
		return "", fmt.Errorf("workspace: create project directory: %w", err)
	}

	return projectDir, nil
}

// InitSceneDir creates a scene directory within a project workspace.
// Layout: {projectPath}/scenes/{sceneNum}/
func InitSceneDir(projectPath string, sceneNum int) (string, error) {
	sceneDir := filepath.Join(projectPath, "scenes", fmt.Sprintf("%d", sceneNum))
	if err := os.MkdirAll(sceneDir, 0o755); err != nil {
		return "", fmt.Errorf("workspace: create scene directory: %w", err)
	}
	return sceneDir, nil
}

// WriteFileAtomic writes data to a file atomically using temp file + rename.
func WriteFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("workspace: create directory for atomic write: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("workspace: create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("workspace: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("workspace: close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("workspace: rename temp to target: %w", err)
	}

	return nil
}

// ProjectExists checks if a project workspace directory exists.
func ProjectExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ReadFile reads the contents of a file. Returns an error if the file doesn't exist.
func ReadFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("workspace: read file %s: %w", path, err)
	}
	return data, nil
}
