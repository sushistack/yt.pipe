package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CleanupResult holds the result of a project cleanup operation.
type CleanupResult struct {
	ProjectID    string   `json:"project_id"`
	FilesRemoved []string `json:"files_removed"`
	BytesFreed   int64    `json:"bytes_freed"`
	Errors       []string `json:"errors,omitempty"`
	DryRun       bool     `json:"dry_run"`
}

// DiskUsage holds disk usage information for a project.
type DiskUsage struct {
	ProjectID     string          `json:"project_id"`
	TotalBytes    int64           `json:"total_bytes"`
	TotalFiles    int             `json:"total_files"`
	Categories    []CategoryUsage `json:"categories"`
	WorkspacePath string          `json:"workspace_path"`
}

// CategoryUsage holds disk usage for a file category.
type CategoryUsage struct {
	Category string `json:"category"`
	Bytes    int64  `json:"bytes"`
	Files    int    `json:"files"`
}

// intermediatePatterns are glob patterns for intermediate artifacts that can be cleaned.
var intermediatePatterns = []string{
	"scenes/*/prompt.txt",
	"scenes/*/manifest.json",
	"checkpoint.json",
	"manifest.json",
}

// CleanProject removes intermediate artifacts from a project workspace.
// If allFiles is true, removes everything including final outputs.
// workspaceRoot is used to validate that workspacePath is contained within it.
func CleanProject(workspacePath, workspaceRoot string, allFiles bool, dryRun bool) (*CleanupResult, error) {
	result := &CleanupResult{DryRun: dryRun}

	// Validate workspace path containment
	if err := validateContainment(workspacePath, workspaceRoot); err != nil {
		return nil, err
	}

	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("cleanup: workspace path does not exist: %s", workspacePath)
	}

	if allFiles {
		return cleanAll(workspacePath, dryRun)
	}

	// Clean intermediate files only
	for _, pattern := range intermediatePatterns {
		matches, err := filepath.Glob(filepath.Join(workspacePath, pattern))
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("glob %s: %v", pattern, err))
			continue
		}
		for _, path := range matches {
			info, err := os.Lstat(path) // Lstat to avoid following symlinks
			if err != nil {
				continue
			}
			if info.Mode()&os.ModeSymlink != 0 {
				continue // skip symlinks
			}
			result.BytesFreed += info.Size()
			result.FilesRemoved = append(result.FilesRemoved, path)
			if !dryRun {
				if err := os.Remove(path); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("remove %s: %v", path, err))
				}
			}
		}
	}

	// Clean scene temporary files
	scenesDir := filepath.Join(workspacePath, "scenes")
	entries, err := os.ReadDir(scenesDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			sceneDir := filepath.Join(scenesDir, entry.Name())
			tempFiles := []string{"timing.json", "words.json"}
			for _, name := range tempFiles {
				path := filepath.Join(sceneDir, name)
				info, err := os.Lstat(path)
				if err != nil {
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					continue
				}
				result.BytesFreed += info.Size()
				result.FilesRemoved = append(result.FilesRemoved, path)
				if !dryRun {
					if err := os.Remove(path); err != nil {
						result.Errors = append(result.Errors, fmt.Sprintf("remove %s: %v", path, err))
					}
				}
			}
		}
	}

	return result, nil
}

func cleanAll(workspacePath string, dryRun bool) (*CleanupResult, error) {
	result := &CleanupResult{DryRun: dryRun}

	// Use Lstat-aware walk to avoid following symlinks
	err := filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		// Use Lstat to get the link info, not the target
		linfo, lerr := os.Lstat(path)
		if lerr != nil {
			return nil
		}
		result.BytesFreed += linfo.Size()
		result.FilesRemoved = append(result.FilesRemoved, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cleanup: walk workspace: %w", err)
	}

	if !dryRun {
		if err := os.RemoveAll(workspacePath); err != nil {
			return nil, fmt.Errorf("cleanup: remove workspace: %w", err)
		}
	}

	return result, nil
}

// validateContainment checks that workspacePath is within workspaceRoot.
func validateContainment(workspacePath, workspaceRoot string) error {
	if workspaceRoot == "" {
		return nil // no root configured, skip validation
	}

	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return fmt.Errorf("cleanup: resolve workspace path: %w", err)
	}
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return fmt.Errorf("cleanup: resolve workspace root: %w", err)
	}

	// Ensure the workspace path is under the workspace root
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		return fmt.Errorf("cleanup: workspace path %s is outside workspace root %s", absPath, absRoot)
	}
	return nil
}

// GetDiskUsage calculates disk usage for a project workspace.
func GetDiskUsage(workspacePath string) (*DiskUsage, error) {
	usage := &DiskUsage{WorkspacePath: workspacePath}

	if _, err := os.Stat(workspacePath); os.IsNotExist(err) {
		return usage, nil
	}

	categories := map[string]*CategoryUsage{
		"images":    {Category: "images"},
		"audio":     {Category: "audio"},
		"subtitles": {Category: "subtitles"},
		"scenario":  {Category: "scenario"},
		"output":    {Category: "output"},
		"other":     {Category: "other"},
	}

	err := filepath.Walk(workspacePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		usage.TotalBytes += info.Size()
		usage.TotalFiles++

		rel, _ := filepath.Rel(workspacePath, path)
		cat := categorizeFile(rel, info.Name())
		if c, ok := categories[cat]; ok {
			c.Bytes += info.Size()
			c.Files++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("disk usage: walk: %w", err)
	}

	for _, cat := range []string{"images", "audio", "subtitles", "scenario", "output", "other"} {
		c := categories[cat]
		if c.Files > 0 {
			usage.Categories = append(usage.Categories, *c)
		}
	}

	return usage, nil
}

func categorizeFile(relPath, name string) string {
	ext := filepath.Ext(name)
	switch {
	case matchPrefix(relPath, "output"):
		return "output"
	case ext == ".png" || ext == ".jpg" || ext == ".webp":
		return "images"
	case ext == ".mp3" || ext == ".wav":
		return "audio"
	case ext == ".srt" || name == "subtitle.json":
		return "subtitles"
	case name == "scenario.json":
		return "scenario"
	default:
		return "other"
	}
}

func matchPrefix(relPath, prefix string) bool {
	return relPath == prefix ||
		(len(relPath) > len(prefix) && relPath[:len(prefix)] == prefix && relPath[len(prefix)] == filepath.Separator)
}
