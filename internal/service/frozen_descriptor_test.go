package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFromResearch_FrozenDescriptorSection(t *testing.T) {
	svc := NewFrozenDescriptorService()

	research := `### Visual Identity Profile
Some intro text about the entity.

### Frozen Descriptor
A tall, pale humanoid figure standing approximately 2.5 meters tall with elongated limbs and a featureless face covered by a dark porcelain mask with hollow eye sockets.

### Behavioral Patterns
The entity moves slowly...`

	descriptor := svc.ExtractFromResearch(research)
	assert.Contains(t, descriptor, "tall, pale humanoid figure")
	assert.Contains(t, descriptor, "porcelain mask")
	assert.NotContains(t, descriptor, "Behavioral Patterns")
}

func TestExtractFromResearch_VisualIdentityFallback(t *testing.T) {
	svc := NewFrozenDescriptorService()

	research := `### Visual Identity
A massive reptilian creature with iridescent green scales, four muscular legs, and a row of bone-white spines running along its back.

### Containment
Special containment...`

	descriptor := svc.ExtractFromResearch(research)
	assert.Contains(t, descriptor, "reptilian creature")
	assert.Contains(t, descriptor, "iridescent green scales")
	assert.NotContains(t, descriptor, "Containment")
}

func TestExtractFromResearch_NoSection(t *testing.T) {
	svc := NewFrozenDescriptorService()
	descriptor := svc.ExtractFromResearch("No visual identity information here")
	assert.Empty(t, descriptor)
}

func TestExtractFromResearch_MarkdownStripped(t *testing.T) {
	svc := NewFrozenDescriptorService()

	research := `### Frozen Descriptor
- **Silhouette**: A tall humanoid with elongated arms
- **Face**: Featureless porcelain mask

### Next Section`

	descriptor := svc.ExtractFromResearch(research)
	assert.NotContains(t, descriptor, "**")
	assert.NotContains(t, descriptor, "- ")
	assert.Contains(t, descriptor, "tall humanoid")
}

func TestSaveAndLoadFromWorkspace(t *testing.T) {
	svc := NewFrozenDescriptorService()
	tmpDir := t.TempDir()

	descriptor := "A tall humanoid figure with pale skin and hollow eyes"
	require.NoError(t, svc.SaveToWorkspace(tmpDir, descriptor))

	loaded, err := svc.LoadFromWorkspace(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, descriptor, loaded)
}

func TestLoadFromWorkspace_NotExists(t *testing.T) {
	svc := NewFrozenDescriptorService()
	loaded, err := svc.LoadFromWorkspace(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, loaded)
}

func TestLoadFromWorkspace_FileExists(t *testing.T) {
	svc := NewFrozenDescriptorService()
	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, frozenDescriptorFile)
	require.NoError(t, os.WriteFile(path, []byte("existing descriptor text"), 0o644))

	loaded, err := svc.LoadFromWorkspace(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "existing descriptor text", loaded)
}

func TestValidateInPrompt_Verbatim(t *testing.T) {
	svc := NewFrozenDescriptorService()
	descriptor := "A tall humanoid figure with pale skin"
	prompt := "A tall humanoid figure with pale skin, standing in a dark hallway, wide shot"

	valid, corrected, vType := svc.ValidateInPrompt(prompt, descriptor, true)
	assert.True(t, valid)
	assert.Equal(t, prompt, corrected)
	assert.Equal(t, "verbatim", vType)
}

func TestValidateInPrompt_FuzzyMatch(t *testing.T) {
	svc := NewFrozenDescriptorService()
	descriptor := "A tall humanoid figure with pale skin and hollow dark eyes wearing a tattered robe"
	// Most words match but not verbatim
	prompt := "A tall humanoid figure with pale skin and hollow dark eyes wearing tattered robe, in corridor"

	valid, _, vType := svc.ValidateInPrompt(prompt, descriptor, true)
	assert.True(t, valid)
	assert.Equal(t, "fuzzy", vType)
}

func TestValidateInPrompt_AutoCorrect(t *testing.T) {
	svc := NewFrozenDescriptorService()
	descriptor := "A tall humanoid figure with pale skin and hollow eyes wearing a tattered robe"
	prompt := "A generic monster in a dark room"

	valid, corrected, vType := svc.ValidateInPrompt(prompt, descriptor, true)
	assert.False(t, valid)
	assert.Equal(t, "corrected", vType)
	assert.Contains(t, corrected, descriptor)
	assert.Contains(t, corrected, "generic monster")
}

func TestValidateInPrompt_NotApplicable(t *testing.T) {
	svc := NewFrozenDescriptorService()

	// Entity not visible
	valid, _, vType := svc.ValidateInPrompt("prompt", "descriptor", false)
	assert.True(t, valid)
	assert.Equal(t, "not_applicable", vType)

	// Empty descriptor
	valid, _, vType = svc.ValidateInPrompt("prompt", "", true)
	assert.True(t, valid)
	assert.Equal(t, "not_applicable", vType)
}

func TestComputeSimilarity(t *testing.T) {
	descriptor := "tall humanoid figure with pale skin and hollow eyes"

	// Exact match (all words present)
	sim := computeSimilarity("a tall humanoid figure with pale skin and hollow eyes in a room", descriptor)
	assert.Greater(t, sim, 0.9)

	// Partial match
	sim = computeSimilarity("a tall humanoid figure in a room", descriptor)
	assert.Greater(t, sim, 0.3)
	assert.Less(t, sim, 0.7)

	// No match
	sim = computeSimilarity("a generic monster", descriptor)
	assert.Less(t, sim, 0.3)
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("A tall, pale humanoid figure with hollow eyes")
	assert.Contains(t, tokens, "tall")
	assert.Contains(t, tokens, "pale")
	assert.Contains(t, tokens, "humanoid")
	assert.Contains(t, tokens, "figure")
	// Short words should be excluded
	for _, tok := range tokens {
		assert.GreaterOrEqual(t, len(tok), 3)
	}
}

func TestStripMarkdown(t *testing.T) {
	assert.Equal(t, "bold text here", stripMarkdown("**bold text** here"))
	assert.Equal(t, "bullet item", stripMarkdown("- bullet item"))
	assert.Equal(t, "star item", stripMarkdown("* star item"))
}
