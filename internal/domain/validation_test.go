package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidationResult_CalculateScore_WithCharacter(t *testing.T) {
	v := &ValidationResult{
		PromptMatch:    80,
		CharacterMatch: 60,
		TechnicalScore: 90,
	}
	v.CalculateScore()
	// 80*0.5 + 60*0.3 + 90*0.2 = 40 + 18 + 18 = 76
	assert.Equal(t, 76, v.Score)
}

func TestValidationResult_CalculateScore_WithoutCharacter(t *testing.T) {
	v := &ValidationResult{
		PromptMatch:    80,
		CharacterMatch: -1,
		TechnicalScore: 90,
	}
	v.CalculateScore()
	// 80*0.7 + 90*0.3 = 56 + 27 = 83
	assert.Equal(t, 83, v.Score)
}

func TestValidationResult_CalculateScore_Perfect(t *testing.T) {
	v := &ValidationResult{
		PromptMatch:    100,
		CharacterMatch: 100,
		TechnicalScore: 100,
	}
	v.CalculateScore()
	assert.Equal(t, 100, v.Score)
}

func TestValidationResult_CalculateScore_Zero(t *testing.T) {
	v := &ValidationResult{
		PromptMatch:    0,
		CharacterMatch: 0,
		TechnicalScore: 0,
	}
	v.CalculateScore()
	assert.Equal(t, 0, v.Score)
}

func TestValidationResult_Evaluate_AboveThreshold(t *testing.T) {
	v := &ValidationResult{
		PromptMatch:    80,
		CharacterMatch: 80,
		TechnicalScore: 80,
	}
	v.Evaluate(70)
	assert.Equal(t, 80, v.Score)
	assert.False(t, v.ShouldRegenerate)
}

func TestValidationResult_Evaluate_BelowThreshold(t *testing.T) {
	v := &ValidationResult{
		PromptMatch:    40,
		CharacterMatch: 30,
		TechnicalScore: 50,
	}
	v.Evaluate(70)
	// 40*0.5 + 30*0.3 + 50*0.2 = 20 + 9 + 10 = 39
	assert.Equal(t, 39, v.Score)
	assert.True(t, v.ShouldRegenerate)
}

func TestValidationResult_Evaluate_ExactThreshold(t *testing.T) {
	v := &ValidationResult{
		PromptMatch:    70,
		CharacterMatch: 70,
		TechnicalScore: 70,
	}
	v.Evaluate(70)
	assert.Equal(t, 70, v.Score)
	assert.False(t, v.ShouldRegenerate)
}
