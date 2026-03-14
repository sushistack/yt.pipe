package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// ExpectedSchemaVersion is the expected schema version for SCP data files.
const ExpectedSchemaVersion = "1.0"

// SCPData holds all loaded data for a single SCP entry.
type SCPData struct {
	SCPID    string
	Facts    *FactsFile
	Meta     *MetaFile
	MainText string
}

// FactsFile represents the contents of facts.json.
// Supports both legacy format (schema_version + facts map) and
// crawled format (flat fields like scp_id, object_class, etc.).
type FactsFile struct {
	SchemaVersion string            `json:"schema_version,omitempty"`
	Facts         map[string]string `json:"facts,omitempty"`
	// Crawled format fields (used as raw data for LLM)
	RawFields map[string]interface{} `json:"-"`
}

// MetaFile represents the contents of meta.json.
// Supports both legacy format (schema_version + structured fields) and
// crawled format (flat fields without schema_version).
type MetaFile struct {
	SchemaVersion  string `json:"schema_version,omitempty"`
	Title          string `json:"title"`
	ObjectClass    string `json:"object_class"`
	Series         string `json:"series"`
	URL            string `json:"url"`
	Author         string `json:"author,omitempty"`
	CopyrightNotes string `json:"copyright_notes,omitempty"`
}

// SCPListEntry is a summary of an available SCP on the filesystem.
type SCPListEntry struct {
	SCPID      string `json:"scp_id"`
	Title      string `json:"title,omitempty"`
	Rating     int    `json:"rating"`
	HasProject bool   `json:"has_project"`
}

// ListAvailableSCPs scans the SCP data directory and returns all valid SCP entries,
// sorted by rating descending. existingSCPs is a set of SCP IDs that already have projects.
func ListAvailableSCPs(basePath string, existingSCPs map[string]bool) ([]SCPListEntry, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("list scps: read dir: %w", err)
	}

	var result []SCPListEntry
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dir := filepath.Join(basePath, name)

		// Check that facts.json exists (primary data file for crawled format)
		factsPath := filepath.Join(dir, "facts.json")
		if _, statErr := os.Stat(factsPath); statErr != nil {
			continue
		}

		entry := SCPListEntry{SCPID: name}
		if existingSCPs != nil {
			entry.HasProject = existingSCPs[name]
		}

		// Read rating and title from facts.json
		if data, readErr := os.ReadFile(factsPath); readErr == nil {
			var raw struct {
				Title  string `json:"title"`
				Rating int    `json:"rating"`
			}
			if json.Unmarshal(data, &raw) == nil {
				entry.Rating = raw.Rating
				if raw.Title != "" {
					entry.Title = raw.Title
				}
			}
		}

		result = append(result, entry)
	}

	// Sort by rating descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Rating > result[j].Rating
	})

	return result, nil
}

// LoadSCPData loads and validates SCP structured data from the data directory.
// It expects {basePath}/{scpID}/ to contain facts.json, meta.json, and main.txt.
func LoadSCPData(basePath, scpID string) (*SCPData, error) {
	dir := filepath.Join(basePath, scpID)

	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil, &domain.NotFoundError{Resource: "SCP data", ID: scpID}
	}
	if err != nil {
		return nil, fmt.Errorf("scp data: stat directory: %w", err)
	}
	if !info.IsDir() {
		return nil, &domain.NotFoundError{Resource: "SCP data", ID: scpID}
	}

	facts, err := loadFactsFile(filepath.Join(dir, "facts.json"))
	if err != nil {
		return nil, err
	}

	meta, err := loadMetaFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return nil, err
	}

	mainText, err := loadMainText(filepath.Join(dir, "main.txt"))
	if err != nil {
		return nil, err
	}

	return &SCPData{
		SCPID:    scpID,
		Facts:    facts,
		Meta:     meta,
		MainText: mainText,
	}, nil
}

func loadFactsFile(path string) (*FactsFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, &domain.ValidationError{Field: "facts.json", Message: "file not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("scp data: read facts.json: %w", err)
	}

	var f FactsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, &domain.ValidationError{Field: "facts.json", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	// If no schema_version, treat as crawled format — store raw fields
	if f.SchemaVersion == "" {
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err == nil {
			f.RawFields = raw
		}
	} else if f.SchemaVersion != ExpectedSchemaVersion {
		return nil, &domain.ValidationError{
			Field:   "facts.json",
			Message: fmt.Sprintf("schema version mismatch: expected %s, got %s", ExpectedSchemaVersion, f.SchemaVersion),
		}
	}

	return &f, nil
}

func loadMetaFile(path string) (*MetaFile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, &domain.ValidationError{Field: "meta.json", Message: "file not found"}
	}
	if err != nil {
		return nil, fmt.Errorf("scp data: read meta.json: %w", err)
	}

	var m MetaFile
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, &domain.ValidationError{Field: "meta.json", Message: fmt.Sprintf("invalid JSON: %v", err)}
	}

	// Skip version check if no schema_version (crawled format)
	if m.SchemaVersion != "" && m.SchemaVersion != ExpectedSchemaVersion {
		return nil, &domain.ValidationError{
			Field:   "meta.json",
			Message: fmt.Sprintf("schema version mismatch: expected %s, got %s", ExpectedSchemaVersion, m.SchemaVersion),
		}
	}

	return &m, nil
}

func loadMainText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", &domain.ValidationError{Field: "main.txt", Message: "file not found"}
	}
	if err != nil {
		return "", fmt.Errorf("scp data: read main.txt: %w", err)
	}
	return string(data), nil
}
