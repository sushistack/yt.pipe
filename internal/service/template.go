package service

import (
	"embed"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
)

//go:embed default_templates/*.md
var defaultTemplatesFS embed.FS

// TemplateService manages prompt template lifecycle and business rules.
type TemplateService struct {
	store *store.Store
}

// NewTemplateService creates a new TemplateService.
func NewTemplateService(s *store.Store) *TemplateService {
	return &TemplateService{store: s}
}

// CreateTemplate validates category, generates UUID, and creates the template.
func (ts *TemplateService) CreateTemplate(category domain.TemplateCategory, name, content string, isDefault bool) (*domain.PromptTemplate, error) {
	if !domain.ValidTemplateCategories[category] {
		return nil, &domain.ValidationError{Field: "category", Message: fmt.Sprintf("invalid category: %s", category)}
	}
	if name == "" {
		return nil, &domain.ValidationError{Field: "name", Message: "must not be empty"}
	}
	if content == "" {
		return nil, &domain.ValidationError{Field: "content", Message: "must not be empty"}
	}

	t := &domain.PromptTemplate{
		ID:        uuid.New().String(),
		Category:  category,
		Name:      name,
		Content:   content,
		IsDefault: isDefault,
	}
	if err := ts.store.CreateTemplate(t); err != nil {
		return nil, fmt.Errorf("service: create template: %w", err)
	}
	return t, nil
}

// GetTemplate retrieves a template by ID.
func (ts *TemplateService) GetTemplate(id string) (*domain.PromptTemplate, error) {
	return ts.store.GetTemplate(id)
}

// ListTemplates returns templates optionally filtered by category.
func (ts *TemplateService) ListTemplates(category string) ([]*domain.PromptTemplate, error) {
	if category != "" && !domain.ValidTemplateCategories[domain.TemplateCategory(category)] {
		return nil, &domain.ValidationError{Field: "category", Message: fmt.Sprintf("invalid category: %s", category)}
	}
	return ts.store.ListTemplates(category)
}

// UpdateTemplate updates template content and creates a new version.
func (ts *TemplateService) UpdateTemplate(id, content string) (*domain.PromptTemplate, error) {
	if content == "" {
		return nil, &domain.ValidationError{Field: "content", Message: "must not be empty"}
	}

	// Verify template exists
	if _, err := ts.store.GetTemplate(id); err != nil {
		return nil, err
	}

	versionID := uuid.New().String()
	if err := ts.store.UpdateTemplate(id, content, versionID); err != nil {
		return nil, fmt.Errorf("service: update template: %w", err)
	}

	return ts.store.GetTemplate(id)
}

// RollbackTemplate restores template content to a specified version.
func (ts *TemplateService) RollbackTemplate(id string, version int) (*domain.PromptTemplate, error) {
	// Verify version exists
	if _, err := ts.store.GetTemplateVersion(id, version); err != nil {
		return nil, err
	}

	versionID := uuid.New().String()
	if err := ts.store.RollbackTemplate(id, version, versionID); err != nil {
		return nil, fmt.Errorf("service: rollback template: %w", err)
	}

	return ts.store.GetTemplate(id)
}

// DeleteTemplate removes a template. Default templates cannot be deleted.
func (ts *TemplateService) DeleteTemplate(id string) error {
	t, err := ts.store.GetTemplate(id)
	if err != nil {
		return err
	}
	if t.IsDefault {
		return &domain.ValidationError{Field: "template", Message: "cannot delete default template"}
	}
	return ts.store.DeleteTemplate(id)
}

// ResolveTemplate returns project override content if exists, else global template content.
func (ts *TemplateService) ResolveTemplate(projectID, templateID string) (string, error) {
	override, err := ts.store.GetOverride(projectID, templateID)
	if err == nil {
		return override.Content, nil
	}
	// If not a NotFoundError, propagate
	if _, ok := err.(*domain.NotFoundError); !ok {
		return "", err
	}

	t, err := ts.store.GetTemplate(templateID)
	if err != nil {
		return "", err
	}
	return t.Content, nil
}

// SetOverride stores a project-specific template override.
func (ts *TemplateService) SetOverride(projectID, templateID, content string) error {
	if content == "" {
		return &domain.ValidationError{Field: "content", Message: "must not be empty"}
	}
	// Verify template exists
	if _, err := ts.store.GetTemplate(templateID); err != nil {
		return err
	}
	return ts.store.SetOverride(projectID, templateID, content)
}

// DeleteOverride removes a project-specific template override.
func (ts *TemplateService) DeleteOverride(projectID, templateID string) error {
	return ts.store.DeleteOverride(projectID, templateID)
}

// GetTemplateVersion retrieves a specific version of a template.
func (ts *TemplateService) GetTemplateVersion(templateID string, version int) (*domain.TemplateVersion, error) {
	return ts.store.GetTemplateVersion(templateID, version)
}

// ListTemplateVersions returns all version records for a template.
func (ts *TemplateService) ListTemplateVersions(templateID string) ([]*domain.TemplateVersion, error) {
	return ts.store.ListTemplateVersions(templateID)
}

// defaultTemplateEntry defines a default template to install.
type defaultTemplateEntry struct {
	Category domain.TemplateCategory
	Name     string
	File     string // filename in default_templates/
}

var defaultTemplateEntries = []defaultTemplateEntry{
	{Category: domain.CategoryScenario, Name: "SCP Research & Analysis", File: "scenario.md"},
	{Category: domain.CategoryImage, Name: "Cinematic Shot Breakdown", File: "image.md"},
	{Category: domain.CategoryTTS, Name: "Korean TTS Preprocessing", File: "tts.md"},
	{Category: domain.CategoryCaption, Name: "Korean Subtitle Generation", File: "caption.md"},
}

// InstallDefaults installs default prompt templates for all 4 categories.
// Idempotent: skips if default templates already exist.
// Returns the number of templates installed (0 if already present).
func (ts *TemplateService) InstallDefaults() (int, error) {
	// Check if defaults already installed
	existing, err := ts.store.ListTemplates("")
	if err != nil {
		return 0, fmt.Errorf("service: install defaults: %w", err)
	}
	for _, t := range existing {
		if t.IsDefault {
			return 0, nil // defaults already installed
		}
	}

	installed := 0
	for _, entry := range defaultTemplateEntries {
		content, err := defaultTemplatesFS.ReadFile("default_templates/" + entry.File)
		if err != nil {
			return installed, fmt.Errorf("service: read default template %s: %w", entry.File, err)
		}

		trimmed := strings.TrimSpace(string(content))
		if trimmed == "" {
			continue
		}

		t := &domain.PromptTemplate{
			ID:        uuid.New().String(),
			Category:  entry.Category,
			Name:      entry.Name,
			Content:   trimmed,
			IsDefault: true,
		}
		if err := ts.store.CreateTemplate(t); err != nil {
			return installed, fmt.Errorf("service: install default template %s: %w", entry.Name, err)
		}
		installed++
	}

	return installed, nil
}
