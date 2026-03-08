package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetup_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := SetupWithWriter("info", "json", &buf)

	logger.Info("test message", "key", "value")

	var entry map[string]any
	err := json.Unmarshal(buf.Bytes(), &entry)
	require.NoError(t, err)
	assert.Equal(t, "test message", entry["msg"])
	assert.Equal(t, "value", entry["key"])
	assert.Equal(t, "INFO", entry["level"])
}

func TestSetup_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := SetupWithWriter("debug", "text", &buf)

	logger.Debug("debug msg")

	output := buf.String()
	assert.True(t, strings.Contains(output, "debug msg"))
	assert.True(t, strings.Contains(output, "DEBUG"))
}

func TestSetup_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := SetupWithWriter("warn", "json", &buf)

	logger.Info("should be filtered")
	assert.Empty(t, buf.String())

	logger.Warn("should appear")
	assert.NotEmpty(t, buf.String())
}

func TestSetup_DefaultLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := SetupWithWriter("", "json", &buf)

	logger.Info("info msg")
	assert.NotEmpty(t, buf.String())

	buf.Reset()
	logger.Debug("debug msg")
	assert.Empty(t, buf.String())
}
