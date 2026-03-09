package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidTemplateCategories_AcceptsValidCategories(t *testing.T) {
	valid := []TemplateCategory{CategoryScenario, CategoryImage, CategoryTTS, CategoryCaption}
	for _, cat := range valid {
		t.Run(string(cat), func(t *testing.T) {
			assert.True(t, ValidTemplateCategories[cat], "expected %s to be valid", cat)
		})
	}
}

func TestValidTemplateCategories_RejectsInvalidCategories(t *testing.T) {
	invalid := []TemplateCategory{"video", "audio", "", "SCENARIO", "Image"}
	for _, cat := range invalid {
		t.Run(string(cat), func(t *testing.T) {
			assert.False(t, ValidTemplateCategories[cat], "expected %s to be invalid", cat)
		})
	}
}

func TestTemplateCategoryConstants(t *testing.T) {
	assert.Equal(t, TemplateCategory("scenario"), CategoryScenario)
	assert.Equal(t, TemplateCategory("image"), CategoryImage)
	assert.Equal(t, TemplateCategory("tts"), CategoryTTS)
	assert.Equal(t, TemplateCategory("caption"), CategoryCaption)
}
