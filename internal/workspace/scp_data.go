package workspace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
type FactsFile struct {
	SchemaVersion string            `json:"schema_version"`
	Facts         map[string]string `json:"facts"`
}

// MetaFile represents the contents of meta.json.
type MetaFile struct {
	SchemaVersion  string `json:"schema_version"`
	Title          string `json:"title"`
	ObjectClass    string `json:"object_class"`
	Series         string `json:"series"`
	URL            string `json:"url"`
	Author         string `json:"author,omitempty"`
	CopyrightNotes string `json:"copyright_notes,omitempty"`
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

	if f.SchemaVersion != ExpectedSchemaVersion {
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

	if m.SchemaVersion != ExpectedSchemaVersion {
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
