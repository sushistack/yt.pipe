package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptString_WithDefault(t *testing.T) {
	r := strings.NewReader("\n")
	w := &bytes.Buffer{}

	val, err := promptString(r, w, "Enter path", "/default/path")
	require.NoError(t, err)
	assert.Equal(t, "/default/path", val)
	assert.Contains(t, w.String(), "[/default/path]")
}

func TestPromptString_WithInput(t *testing.T) {
	r := strings.NewReader("/custom/path\n")
	w := &bytes.Buffer{}

	val, err := promptString(r, w, "Enter path", "/default/path")
	require.NoError(t, err)
	assert.Equal(t, "/custom/path", val)
}

func TestPromptString_TrimWhitespace(t *testing.T) {
	r := strings.NewReader("  /trimmed/path  \n")
	w := &bytes.Buffer{}

	val, err := promptString(r, w, "Enter path", "")
	require.NoError(t, err)
	assert.Equal(t, "/trimmed/path", val)
}

func TestPromptString_NoDefault(t *testing.T) {
	r := strings.NewReader("value\n")
	w := &bytes.Buffer{}

	val, err := promptString(r, w, "Enter value", "")
	require.NoError(t, err)
	assert.Equal(t, "value", val)
	// Should not show brackets when no default
	assert.Equal(t, "Enter value: ", w.String())
}

func TestPromptSecret_ReadsLine(t *testing.T) {
	r := strings.NewReader("sk-secret-key-123\n")
	w := &bytes.Buffer{}

	val, err := promptSecret(r, w, "API Key")
	require.NoError(t, err)
	assert.Equal(t, "sk-secret-key-123", val)
	assert.Contains(t, w.String(), "API Key: ")
}

func TestPromptSecret_EmptyInput(t *testing.T) {
	r := strings.NewReader("\n")
	w := &bytes.Buffer{}

	val, err := promptSecret(r, w, "API Key")
	require.NoError(t, err)
	assert.Equal(t, "", val)
}

func TestPromptSelect_ValidChoice(t *testing.T) {
	r := strings.NewReader("2\n")
	w := &bytes.Buffer{}

	val, err := promptSelect(r, w, "Choose provider", []string{"openai", "google", "edge"}, 1)
	require.NoError(t, err)
	assert.Equal(t, "google", val)
}

func TestPromptSelect_InvalidThenValid(t *testing.T) {
	// First input is out of range, second is valid
	r := strings.NewReader("5\n2\n")
	w := &bytes.Buffer{}

	val, err := promptSelect(r, w, "Choose provider", []string{"openai", "google", "edge"}, 1)
	require.NoError(t, err)
	assert.Equal(t, "google", val)
	assert.Contains(t, w.String(), "Invalid choice")
}

func TestPromptSelect_Default(t *testing.T) {
	r := strings.NewReader("\n")
	w := &bytes.Buffer{}

	val, err := promptSelect(r, w, "Choose provider", []string{"openai", "google", "edge"}, 2)
	require.NoError(t, err)
	assert.Equal(t, "google", val)
}

func TestPromptSelect_NonNumericThenValid(t *testing.T) {
	r := strings.NewReader("abc\n1\n")
	w := &bytes.Buffer{}

	val, err := promptSelect(r, w, "Choose provider", []string{"openai", "google"}, 1)
	require.NoError(t, err)
	assert.Equal(t, "openai", val)
	assert.Contains(t, w.String(), "Invalid choice")
}

func TestPromptSelect_NoOptions(t *testing.T) {
	r := strings.NewReader("\n")
	w := &bytes.Buffer{}

	_, err := promptSelect(r, w, "Choose", []string{}, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no options provided")
}

func TestPromptConfirm_Yes(t *testing.T) {
	r := strings.NewReader("y\n")
	w := &bytes.Buffer{}

	val, err := promptConfirm(r, w, "Continue?", false)
	require.NoError(t, err)
	assert.True(t, val)
}

func TestPromptConfirm_No(t *testing.T) {
	r := strings.NewReader("n\n")
	w := &bytes.Buffer{}

	val, err := promptConfirm(r, w, "Continue?", true)
	require.NoError(t, err)
	assert.False(t, val)
}

func TestPromptConfirm_Default_Yes(t *testing.T) {
	r := strings.NewReader("\n")
	w := &bytes.Buffer{}

	val, err := promptConfirm(r, w, "Continue?", true)
	require.NoError(t, err)
	assert.True(t, val)
	assert.Contains(t, w.String(), "[Y/n]")
}

func TestPromptConfirm_Default_No(t *testing.T) {
	r := strings.NewReader("\n")
	w := &bytes.Buffer{}

	val, err := promptConfirm(r, w, "Continue?", false)
	require.NoError(t, err)
	assert.False(t, val)
	assert.Contains(t, w.String(), "[y/N]")
}

func TestPromptConfirm_YesVariant(t *testing.T) {
	r := strings.NewReader("yes\n")
	w := &bytes.Buffer{}

	val, err := promptConfirm(r, w, "Continue?", false)
	require.NoError(t, err)
	assert.True(t, val)
}
