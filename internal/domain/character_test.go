package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCharacterStruct(t *testing.T) {
	c := Character{
		ID:               "char-1",
		SCPID:            "SCP-173",
		CanonicalName:    "SCP-173",
		Aliases:          []string{"The Sculpture", "조각상"},
		VisualDescriptor: "concrete humanoid sculpture",
		StyleGuide:       "dark, institutional",
		ImagePromptBase:  "concrete sculpture in containment",
	}
	assert.Equal(t, "char-1", c.ID)
	assert.Equal(t, "SCP-173", c.SCPID)
	assert.Equal(t, "SCP-173", c.CanonicalName)
	assert.Len(t, c.Aliases, 2)
	assert.Equal(t, "concrete humanoid sculpture", c.VisualDescriptor)
}

func TestValidateAliases_ValidAliases(t *testing.T) {
	err := ValidateAliases([]string{"The Sculpture", "조각상"})
	require.NoError(t, err)
}

func TestValidateAliases_EmptySlice(t *testing.T) {
	err := ValidateAliases([]string{})
	require.NoError(t, err)
}

func TestValidateAliases_NilSlice(t *testing.T) {
	err := ValidateAliases(nil)
	require.NoError(t, err)
}

func TestValidateAliases_EmptyString(t *testing.T) {
	err := ValidateAliases([]string{"valid", ""})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index 1")
}

func TestValidateAliases_WhitespaceOnly(t *testing.T) {
	err := ValidateAliases([]string{"  "})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index 0")
}
