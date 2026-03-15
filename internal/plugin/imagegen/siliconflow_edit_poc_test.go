//go:build integration

package imagegen

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImageEditPoC verifies SiliconFlow image-edit API availability for character consistency.
//
// Run: YTP_IMAGEGEN_API_KEY=<key> go test -tags=integration -run TestImageEditPoC -v ./internal/plugin/imagegen/...
func TestImageEditPoC(t *testing.T) {
	apiKey := os.Getenv("YTP_IMAGEGEN_API_KEY")
	if apiKey == "" {
		t.Skip("YTP_IMAGEGEN_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	provider, err := NewSiliconFlowProvider(SiliconFlowConfig{
		APIKey: apiKey,
	})
	require.NoError(t, err)

	// Step 1: Generate a character reference image
	t.Log("Step 1: Generating character reference image...")
	refResult, err := provider.Generate(ctx, "A humanoid concrete statue with crude painted features, rebar protruding from body, standing in a containment cell, SCP Foundation style, detailed character portrait", GenerateOptions{
		Width:  1024,
		Height: 1024,
	})
	require.NoError(t, err)
	require.NotEmpty(t, refResult.ImageData)
	t.Logf("  Reference image: %d bytes", len(refResult.ImageData))

	// Step 2: Test Edit() with the reference image
	t.Log("Step 2: Testing Edit() API...")
	editResult, editErr := provider.Edit(ctx, refResult.ImageData, "The concrete statue in a dark hallway with red emergency lighting", EditOptions{
		Width:  1024,
		Height: 576,
	})

	require.NoError(t, editErr, "Edit() should succeed with implemented Qwen-Image-Edit")

	require.NotNil(t, editResult)
	t.Logf("  Edit result: %d bytes", len(editResult.ImageData))

	// Step 3: Run 10 diverse scene prompts with same reference
	t.Log("Step 3: Testing character consistency across 10 scenes...")
	scenePrompts := []string{
		"The statue standing motionless in a bright white containment cell",
		"The statue in a dark corner, security camera perspective",
		"The statue surrounded by researchers in hazmat suits",
		"Close-up of the statue's crude painted face",
		"The statue at the end of a long concrete corridor",
		"The statue in an outdoor courtyard, overcast sky",
		"The statue partially obscured by shadows",
		"Multiple angles of the statue in a well-lit lab",
		"The statue behind reinforced glass viewing panel",
		"The statue in a destroyed room with debris",
	}

	successCount := 0
	for i, prompt := range scenePrompts {
		sceneResult, sceneErr := provider.Edit(ctx, refResult.ImageData, prompt, EditOptions{
			Width:  1024,
			Height: 576,
		})
		if sceneErr != nil {
			t.Logf("  Scene %d: FAILED - %v", i+1, sceneErr)
			continue
		}
		if len(sceneResult.ImageData) > 0 {
			successCount++
			t.Logf("  Scene %d: OK (%d bytes)", i+1, len(sceneResult.ImageData))
		}
	}

	t.Logf("\nPoC Result: %d/10 scenes generated successfully", successCount)
	if successCount >= 7 {
		t.Log("DECISION: Full image-edit implementation (≥7/10)")
	} else if successCount >= 5 {
		t.Log("DECISION: Hybrid approach (5-6/10)")
	} else {
		t.Log("DECISION: Fallback to CharacterRef prompt injection (≤4/10)")
	}

	assert.Greater(t, successCount, 0, "at least one scene should succeed")
	fmt.Fprintf(os.Stderr, "\n=== PoC Score: %d/10 ===\n", successCount)
}
