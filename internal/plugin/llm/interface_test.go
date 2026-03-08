package llm_test

import (
	"testing"

	llmplugin "github.com/jay/youtube-pipeline/internal/plugin/llm"
)

// Compile-time interface check: ensure the interface is well-defined and importable.
var _ llmplugin.LLM = (llmplugin.LLM)(nil)

func TestLLMInterfaceCompiles(t *testing.T) {
	// This test exists to verify the LLM interface compiles correctly.
	// Actual implementation tests will use mockery-generated mocks.
	t.Log("LLM interface compiles successfully")
}
