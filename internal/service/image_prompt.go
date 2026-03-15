package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// DefaultSafetyModifiers are appended to image prompts to avoid NSFW filter triggers.
var DefaultSafetyModifiers = []string{
	"anime illustration",
	"cel shading",
	"safe for work",
	"clean composition",
}

// DefaultDangerousTerms are removed from prompts during sanitization.
var DefaultDangerousTerms = []string{
	"gore", "blood", "violent", "gruesome",
	"mutilation", "decapitation", "dismemberment",
}

// DefaultPromptTemplate is the built-in prompt template used when no external template is configured.
const DefaultPromptTemplate = `{{.VisualDescription}}{{if .Mood}}, mood: {{.Mood}}{{end}}`

// ImagePromptConfig holds configuration for image prompt generation.
type ImagePromptConfig struct {
	TemplatePath    string   // Path to external template file; empty uses default
	DangerousTerms  []string // Terms to remove during sanitization; nil uses defaults
	SafetyModifiers []string // Modifiers appended after sanitization; nil uses defaults
}

// ImagePromptResult holds the original and sanitized image prompt for a scene.
type ImagePromptResult struct {
	SceneNum        int    `json:"scene_num"`
	OriginalPrompt  string `json:"original_prompt"`
	SanitizedPrompt string `json:"sanitized_prompt"`
	TemplateVersion string `json:"template_version"`
	SCPID           string `json:"scp_id,omitempty"`     // for character auto-reference
	SceneText       string `json:"scene_text,omitempty"` // narration text for character matching
	EntityVisible   bool   `json:"entity_visible"`       // SCP entity appears in this scene's image
}

// GenerateImagePrompts creates image prompts from scenario visual descriptions and applies safety sanitization.
func GenerateImagePrompts(scenario *domain.ScenarioOutput, cfg *ImagePromptConfig) ([]ImagePromptResult, error) {
	if scenario == nil {
		return nil, fmt.Errorf("image prompt: scenario is nil")
	}

	if cfg == nil {
		cfg = &ImagePromptConfig{}
	}
	if cfg.SafetyModifiers == nil {
		cfg.SafetyModifiers = DefaultSafetyModifiers
	}
	if cfg.DangerousTerms == nil {
		cfg.DangerousTerms = DefaultDangerousTerms
	}

	tmplStr, err := loadTemplate(cfg.TemplatePath)
	if err != nil {
		return nil, fmt.Errorf("image prompt: load template: %w", err)
	}
	tmplVersion := hashTemplate(tmplStr)

	tmpl, err := template.New("image_prompt").Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("image prompt: parse template: %w", err)
	}

	results := make([]ImagePromptResult, 0, len(scenario.Scenes))
	for _, scene := range scenario.Scenes {
		var buf strings.Builder
		if err := tmpl.Execute(&buf, scene); err != nil {
			return nil, fmt.Errorf("image prompt: execute template for scene %d: %w", scene.SceneNum, err)
		}
		original := buf.String()
		sanitized := sanitizePrompt(original, cfg.DangerousTerms, cfg.SafetyModifiers)
		results = append(results, ImagePromptResult{
			SceneNum:        scene.SceneNum,
			OriginalPrompt:  original,
			SanitizedPrompt: sanitized,
			TemplateVersion: tmplVersion,
		})
	}
	return results, nil
}

func loadTemplate(path string) (string, error) {
	if path == "" {
		return DefaultPromptTemplate, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read template file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func hashTemplate(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:8])
}

var multiSpaceRe = regexp.MustCompile(`\s{2,}`)

func sanitizePrompt(prompt string, dangerousTerms, modifiers []string) string {
	sanitized := prompt
	for _, term := range dangerousTerms {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
		sanitized = re.ReplaceAllString(sanitized, "")
	}
	sanitized = multiSpaceRe.ReplaceAllString(sanitized, " ")
	sanitized = strings.TrimSpace(sanitized)

	if len(modifiers) > 0 {
		sanitized = sanitized + ", " + strings.Join(modifiers, ", ")
	}
	return sanitized
}
