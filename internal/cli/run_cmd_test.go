package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sushistack/yt.pipe/internal/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeTestConfig creates a temp config YAML and sets up env vars for API keys.
// Returns the config file path.
func writeTestConfig(t *testing.T) string {
	t.Helper()
	scpDir := t.TempDir()
	wsDir := t.TempDir()
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.yaml")

	content := fmt.Sprintf(`scp_data_path: %q
workspace_path: %q
llm:
  provider: "openai"
  model: "gpt-4"
imagegen:
  provider: "siliconflow"
  model: "flux"
tts:
  provider: "openai"
  voice: "alloy"
  speed: 1.0
output:
  provider: "capcut"
`, scpDir, wsDir)

	err := os.WriteFile(cfgPath, []byte(content), 0600)
	require.NoError(t, err)

	t.Setenv("YTP_LLM_API_KEY", "test-llm-key")
	t.Setenv("YTP_IMAGEGEN_API_KEY", "test-imagegen-key")
	t.Setenv("YTP_TTS_API_KEY", "test-tts-key")

	t.Cleanup(func() { appConfig = nil })
	return cfgPath
}

func TestRunCmd_MissingSCPID(t *testing.T) {
	rootCmd.SetArgs([]string{"run"})
	err := rootCmd.Execute()
	assert.Error(t, err)
}

func TestRunCmd_NoDryRun_NeedsPlugins(t *testing.T) {
	cfgPath := writeTestConfig(t)

	// Add db_path so the database can be opened in a writable temp dir
	cfgData, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	dbDir := t.TempDir()
	cfgData = append(cfgData, []byte(fmt.Sprintf("\ndb_path: %q\n", filepath.Join(dbDir, "test.db")))...)
	require.NoError(t, os.WriteFile(cfgPath, cfgData, 0600))

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"run", "SCP-173", "--config", cfgPath})
	t.Cleanup(func() { rootCmd.SetOut(nil) })

	// Without registered plugins, the run command fails with a plugin error
	err = rootCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "plugin")
}

func TestRunCmd_DryRunFlag(t *testing.T) {
	cfgPath := writeTestConfig(t)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"run", "SCP-173", "--dry-run", "--config", cfgPath})
	t.Cleanup(func() { rootCmd.SetOut(nil) })

	err := rootCmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Dry-Run Results")
	assert.Contains(t, buf.String(), "SCP-173")
	assert.Contains(t, buf.String(), "All stages passed")
}

func TestRunCmd_DryRunJSONOutput(t *testing.T) {
	cfgPath := writeTestConfig(t)
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"run", "SCP-173", "--dry-run", "--json-output", "--config", cfgPath})
	t.Cleanup(func() { rootCmd.SetOut(nil) })

	err := rootCmd.Execute()
	assert.NoError(t, err)

	var result pipeline.DryRunResult
	err = json.Unmarshal(buf.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "SCP-173", result.SCPID)
	assert.True(t, result.Success)
	assert.Len(t, result.Stages, 7)
}

func TestDisplayKey_NotSet(t *testing.T) {
	assert.Equal(t, "not set", displayKey(""))
}

func TestDisplayKey_Masked(t *testing.T) {
	assert.Equal(t, "***", displayKey("***"))
}
