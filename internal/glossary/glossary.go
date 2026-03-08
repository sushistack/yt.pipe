package glossary

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// Entry represents a single glossary entry for SCP terminology.
type Entry struct {
	Term          string `json:"term"`
	Pronunciation string `json:"pronunciation"`
	Definition    string `json:"definition"`
	Category      string `json:"category"` // containment_class, organization, entity, etc.
}

// Glossary provides thread-safe lookup of SCP terminology.
type Glossary struct {
	mu      sync.RWMutex
	entries map[string]Entry // keyed by lowercase term
}

// New creates an empty Glossary.
func New() *Glossary {
	return &Glossary{
		entries: make(map[string]Entry),
	}
}

// LoadFromFile loads glossary entries from a JSON file.
// Returns an empty glossary with a warning if the file is missing or malformed.
func LoadFromFile(path string) *Glossary {
	g := New()

	if path == "" {
		slog.Warn("glossary path not configured, using empty glossary")
		return g
	}

	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("glossary file not found, using empty glossary", "path", path, "error", err)
		return g
	}

	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		slog.Warn("glossary file malformed, using empty glossary", "path", path, "error", err)
		return g
	}

	for _, e := range entries {
		g.entries[normalizeKey(e.Term)] = e
	}

	slog.Info("glossary loaded", "entries", len(g.entries), "path", path)
	return g
}

// Lookup returns the glossary entry for a term, or false if not found.
func (g *Glossary) Lookup(term string) (Entry, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	e, ok := g.entries[normalizeKey(term)]
	return e, ok
}

// Pronunciation returns the pronunciation override for a term, or the term itself if not found.
func (g *Glossary) Pronunciation(term string) string {
	if e, ok := g.Lookup(term); ok && e.Pronunciation != "" {
		return e.Pronunciation
	}
	return term
}

// Entries returns all glossary entries.
func (g *Glossary) Entries() []Entry {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]Entry, 0, len(g.entries))
	for _, e := range g.entries {
		result = append(result, e)
	}
	return result
}

// Len returns the number of entries.
func (g *Glossary) Len() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.entries)
}

func normalizeKey(term string) string {
	// Simple lowercase normalization
	result := make([]byte, len(term))
	for i := 0; i < len(term); i++ {
		c := term[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// MarshalJSON serializes the glossary entries for output.
func (g *Glossary) MarshalJSON() ([]byte, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	entries := make([]Entry, 0, len(g.entries))
	for _, e := range g.entries {
		entries = append(entries, e)
	}
	return json.Marshal(entries)
}

// WriteToFile writes the glossary to a JSON file.
func WriteToFile(path string, entries []Entry) error {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("glossary: marshal entries: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}
