package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateImagePrompts_Basic(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, VisualDescription: "A dark containment cell", Mood: "tense"},
			{SceneNum: 2, VisualDescription: "Security cameras", Mood: "suspenseful"},
		},
	}

	results, err := GenerateImagePrompts(scenario, nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, 1, results[0].SceneNum)
	assert.Contains(t, results[0].OriginalPrompt, "A dark containment cell")
	assert.Contains(t, results[0].OriginalPrompt, "mood: tense")
	assert.Contains(t, results[0].SanitizedPrompt, "digital illustration")
	assert.NotEmpty(t, results[0].TemplateVersion)
}

func TestGenerateImagePrompts_SafetySanitization(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, VisualDescription: "A violent gore scene with blood"},
		},
	}

	results, err := GenerateImagePrompts(scenario, &ImagePromptConfig{
		SafetyModifiers: DefaultSafetyModifiers,
	})
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.NotContains(t, results[0].SanitizedPrompt, "violent")
	assert.NotContains(t, results[0].SanitizedPrompt, "gore")
	assert.NotContains(t, results[0].SanitizedPrompt, "blood")
	assert.Contains(t, results[0].SanitizedPrompt, "safe for work")
}

func TestGenerateImagePrompts_PreservesCase(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, VisualDescription: "SCP-173 in a Dark Chamber", Mood: "Eerie"},
		},
	}

	results, err := GenerateImagePrompts(scenario, &ImagePromptConfig{
		SafetyModifiers: []string{"illustration"},
	})
	require.NoError(t, err)
	assert.Contains(t, results[0].SanitizedPrompt, "SCP-173")
	assert.Contains(t, results[0].SanitizedPrompt, "Dark Chamber")
}

func TestGenerateImagePrompts_WordBoundary(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, VisualDescription: "A bloodhound in a gory alley"},
		},
	}

	results, err := GenerateImagePrompts(scenario, &ImagePromptConfig{
		DangerousTerms:  DefaultDangerousTerms,
		SafetyModifiers: []string{},
	})
	require.NoError(t, err)
	// "bloodhound" should NOT be mangled — "blood" is a substring, not whole word
	assert.Contains(t, results[0].SanitizedPrompt, "bloodhound")
	// "gory" should NOT be mangled — "gore" is not a whole-word match for "gory"
	assert.Contains(t, results[0].SanitizedPrompt, "gory")
}

func TestGenerateImagePrompts_CustomModifiers(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, VisualDescription: "A dark room"},
		},
	}

	results, err := GenerateImagePrompts(scenario, &ImagePromptConfig{
		SafetyModifiers: []string{"watercolor", "soft lighting"},
	})
	require.NoError(t, err)
	assert.Contains(t, results[0].SanitizedPrompt, "watercolor")
	assert.Contains(t, results[0].SanitizedPrompt, "soft lighting")
}

func TestGenerateImagePrompts_CustomDangerousTerms(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, VisualDescription: "A scary monster attacks"},
		},
	}

	results, err := GenerateImagePrompts(scenario, &ImagePromptConfig{
		DangerousTerms:  []string{"scary", "attacks"},
		SafetyModifiers: []string{},
	})
	require.NoError(t, err)
	assert.NotContains(t, results[0].SanitizedPrompt, "scary")
	assert.NotContains(t, results[0].SanitizedPrompt, "attacks")
	assert.Contains(t, results[0].SanitizedPrompt, "monster")
}

func TestGenerateImagePrompts_EmptyScenario(t *testing.T) {
	scenario := &domain.ScenarioOutput{Scenes: nil}
	results, err := GenerateImagePrompts(scenario, nil)
	require.NoError(t, err)
	assert.Len(t, results, 0)
}

func TestGenerateImagePrompts_NilScenario(t *testing.T) {
	results, err := GenerateImagePrompts(nil, nil)
	require.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "scenario is nil")
}

func TestGenerateImagePrompts_NoMood(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, VisualDescription: "A simple room"},
		},
	}

	results, err := GenerateImagePrompts(scenario, nil)
	require.NoError(t, err)
	assert.Equal(t, "A simple room", results[0].OriginalPrompt)
	assert.NotContains(t, results[0].OriginalPrompt, "mood")
}

func TestGenerateImagePrompts_ExternalTemplate(t *testing.T) {
	// Write a custom template file
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "prompt.tmpl")
	err := os.WriteFile(tmplPath, []byte(`Scene {{.SceneNum}}: {{.VisualDescription}} [{{.Mood}}]`), 0o644)
	require.NoError(t, err)

	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 5, VisualDescription: "A hallway", Mood: "dark"},
		},
	}

	results, err := GenerateImagePrompts(scenario, &ImagePromptConfig{
		TemplatePath:    tmplPath,
		SafetyModifiers: []string{},
	})
	require.NoError(t, err)
	assert.Equal(t, "Scene 5: A hallway [dark]", results[0].OriginalPrompt)
	assert.NotEmpty(t, results[0].TemplateVersion)
}

func TestGenerateImagePrompts_ExternalTemplateNotFound(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, VisualDescription: "test"},
		},
	}

	_, err := GenerateImagePrompts(scenario, &ImagePromptConfig{
		TemplatePath: "/nonexistent/template.tmpl",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load template")
}

func TestGenerateImagePrompts_TemplateVersionChanges(t *testing.T) {
	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "prompt.tmpl")

	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, VisualDescription: "test"},
		},
	}

	// Version 1
	require.NoError(t, os.WriteFile(tmplPath, []byte(`v1: {{.VisualDescription}}`), 0o644))
	r1, err := GenerateImagePrompts(scenario, &ImagePromptConfig{
		TemplatePath:    tmplPath,
		SafetyModifiers: []string{},
	})
	require.NoError(t, err)

	// Version 2
	require.NoError(t, os.WriteFile(tmplPath, []byte(`v2: {{.VisualDescription}}`), 0o644))
	r2, err := GenerateImagePrompts(scenario, &ImagePromptConfig{
		TemplatePath:    tmplPath,
		SafetyModifiers: []string{},
	})
	require.NoError(t, err)

	assert.NotEqual(t, r1[0].TemplateVersion, r2[0].TemplateVersion)
}
