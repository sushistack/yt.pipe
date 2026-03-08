// Package template provides prompt template management and versioning.
package template

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
)

// maxTemplateSize is the maximum allowed template file size (1 MB).
const maxTemplateSize = 1 << 20

// Info describes a loaded template with its version hash.
type Info struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"` // SHA-256 truncated hash
	Size    int    `json:"size_bytes"`
	ModTime string `json:"mod_time,omitempty"`
}

// Manager manages prompt templates from a templates directory.
// Manager is designed for a load-then-read lifecycle: call LoadAll() or Load()
// during initialization, then use Get/GetInfo/GetVersion/List during execution.
// It is NOT safe for concurrent loading and reading.
type Manager struct {
	templatesDir string
	templates    map[string]*loadedTemplate
}

type loadedTemplate struct {
	info    Info
	content string
	tmpl    *template.Template
}

// NewManager creates a Manager for the given templates directory.
// If templatesDir is empty, only built-in defaults are available.
func NewManager(templatesDir string) *Manager {
	return &Manager{
		templatesDir: templatesDir,
		templates:    make(map[string]*loadedTemplate),
	}
}

// LoadAll loads all .tmpl files from the templates directory.
// Returns an error if any template has syntax errors (fail-fast).
func (m *Manager) LoadAll() error {
	if m.templatesDir == "" {
		return nil
	}

	entries, err := os.ReadDir(m.templatesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // directory doesn't exist yet, that's fine
		}
		return fmt.Errorf("template: read dir %s: %w", m.templatesDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tmpl") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".tmpl")
		if err := m.loadFile(name, filepath.Join(m.templatesDir, entry.Name())); err != nil {
			return err // fail-fast on syntax errors
		}
	}

	return nil
}

// Load loads a single template by name from the templates directory.
func (m *Manager) Load(name string) error {
	if m.templatesDir == "" {
		return fmt.Errorf("template: no templates directory configured")
	}
	path := filepath.Join(m.templatesDir, name+".tmpl")
	return m.loadFile(name, path)
}

func (m *Manager) loadFile(name, path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("template: stat %s: %w", path, err)
	}
	if fi.Size() > maxTemplateSize {
		return fmt.Errorf("template: %s exceeds maximum size (%d bytes)", path, maxTemplateSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("template: read %s: %w", path, err)
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return fmt.Errorf("template: %s is empty", path)
	}

	tmpl, err := template.New(name).Parse(content)
	if err != nil {
		return fmt.Errorf("template: syntax error in %s: %w", path, err)
	}

	version := HashContent(content)

	m.templates[name] = &loadedTemplate{
		info: Info{
			Name:    name,
			Path:    path,
			Version: version,
			Size:    len(content),
			ModTime: fi.ModTime().Format(time.RFC3339),
		},
		content: content,
		tmpl:    tmpl,
	}

	return nil
}

// Get returns a parsed template by name. Returns nil if not found.
func (m *Manager) Get(name string) *template.Template {
	lt, ok := m.templates[name]
	if !ok {
		return nil
	}
	return lt.tmpl
}

// GetInfo returns template info by name.
func (m *Manager) GetInfo(name string) (Info, bool) {
	lt, ok := m.templates[name]
	if !ok {
		return Info{}, false
	}
	return lt.info, true
}

// GetVersion returns the version hash for a named template.
func (m *Manager) GetVersion(name string) string {
	lt, ok := m.templates[name]
	if !ok {
		return ""
	}
	return lt.info.Version
}

// List returns info for all loaded templates, sorted by name.
func (m *Manager) List() []Info {
	result := make([]Info, 0, len(m.templates))
	for _, lt := range m.templates {
		result = append(result, lt.info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// HashContent computes a truncated SHA-256 hash of template content.
func HashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:8])
}

// ValidateFile parses a template file and returns any syntax error.
func ValidateFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("template: read %s: %w", path, err)
	}

	content := strings.TrimSpace(string(data))
	if content == "" {
		return fmt.Errorf("template: %s is empty", path)
	}

	if _, err := template.New(filepath.Base(path)).Parse(content); err != nil {
		return fmt.Errorf("template: syntax error in %s: %w", path, err)
	}
	return nil
}
