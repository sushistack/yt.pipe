package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateMoodPreset_Valid(t *testing.T) {
	err := ValidateMoodPreset("tense", "fearful", 1.0, 1.0)
	assert.NoError(t, err)
}

func TestValidateMoodPreset_EmptyName(t *testing.T) {
	err := ValidateMoodPreset("", "fearful", 1.0, 1.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestValidateMoodPreset_EmptyEmotion(t *testing.T) {
	err := ValidateMoodPreset("tense", "", 1.0, 1.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "emotion")
}

func TestValidateMoodPreset_InvalidSpeed(t *testing.T) {
	err := ValidateMoodPreset("tense", "fearful", 0, 1.0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "speed")
}

func TestValidateMoodPreset_InvalidPitch(t *testing.T) {
	err := ValidateMoodPreset("tense", "fearful", 1.0, -0.5)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pitch")
}
