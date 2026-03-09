package domain

import "time"

// TemplateCategory represents the category of a prompt template.
type TemplateCategory string

const (
	CategoryScenario TemplateCategory = "scenario"
	CategoryImage    TemplateCategory = "image"
	CategoryTTS      TemplateCategory = "tts"
	CategoryCaption  TemplateCategory = "caption"
)

// ValidTemplateCategories defines allowed template category values.
var ValidTemplateCategories = map[TemplateCategory]bool{
	CategoryScenario: true,
	CategoryImage:    true,
	CategoryTTS:      true,
	CategoryCaption:  true,
}

// PromptTemplate represents a versioned prompt template.
type PromptTemplate struct {
	ID        string
	Category  TemplateCategory
	Name      string
	Content   string
	Version   int
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TemplateVersion represents a historical version of a prompt template.
type TemplateVersion struct {
	ID         string
	TemplateID string
	Version    int
	Content    string
	CreatedAt  time.Time
}

// ProjectTemplateOverride represents a per-project template override.
type ProjectTemplateOverride struct {
	ProjectID  string
	TemplateID string
	Content    string
	CreatedAt  time.Time
}
