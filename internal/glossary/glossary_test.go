package glossary

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGlossaryFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.json")
	entries := []Entry{
		{Term: "SCP-173", Pronunciation: "ess see pee one seven three", Definition: "The Sculpture", Category: "entity"},
		{Term: "Euclid", Pronunciation: "yoo-klid", Definition: "Object class requiring special containment", Category: "containment_class"},
		{Term: "Foundation", Pronunciation: "", Definition: "SCP Foundation", Category: "organization"},
	}
	require.NoError(t, WriteToFile(path, entries))
	return path
}

func TestLoadFromFile_Success(t *testing.T) {
	path := setupGlossaryFile(t)
	g := LoadFromFile(path)
	assert.Equal(t, 3, g.Len())
}

func TestLoadFromFile_MissingFile(t *testing.T) {
	g := LoadFromFile("/nonexistent/path.json")
	assert.Equal(t, 0, g.Len())
}

func TestLoadFromFile_EmptyPath(t *testing.T) {
	g := LoadFromFile("")
	assert.Equal(t, 0, g.Len())
}

func TestLoadFromFile_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{not valid json"), 0o644))
	g := LoadFromFile(path)
	assert.Equal(t, 0, g.Len())
}

func TestLookup_Found(t *testing.T) {
	path := setupGlossaryFile(t)
	g := LoadFromFile(path)

	e, ok := g.Lookup("SCP-173")
	assert.True(t, ok)
	assert.Equal(t, "The Sculpture", e.Definition)
}

func TestLookup_CaseInsensitive(t *testing.T) {
	path := setupGlossaryFile(t)
	g := LoadFromFile(path)

	e, ok := g.Lookup("euclid")
	assert.True(t, ok)
	assert.Equal(t, "yoo-klid", e.Pronunciation)

	e2, ok2 := g.Lookup("EUCLID")
	assert.True(t, ok2)
	assert.Equal(t, e.Pronunciation, e2.Pronunciation)
}

func TestLookup_NotFound(t *testing.T) {
	path := setupGlossaryFile(t)
	g := LoadFromFile(path)

	_, ok := g.Lookup("nonexistent")
	assert.False(t, ok)
}

func TestPronunciation_WithOverride(t *testing.T) {
	path := setupGlossaryFile(t)
	g := LoadFromFile(path)
	assert.Equal(t, "ess see pee one seven three", g.Pronunciation("SCP-173"))
}

func TestPronunciation_NoOverride(t *testing.T) {
	path := setupGlossaryFile(t)
	g := LoadFromFile(path)
	assert.Equal(t, "Foundation", g.Pronunciation("Foundation"))
}

func TestPronunciation_NotInGlossary(t *testing.T) {
	g := New()
	assert.Equal(t, "unknown-term", g.Pronunciation("unknown-term"))
}

func TestEntries(t *testing.T) {
	path := setupGlossaryFile(t)
	g := LoadFromFile(path)
	entries := g.Entries()
	assert.Len(t, entries, 3)
}
