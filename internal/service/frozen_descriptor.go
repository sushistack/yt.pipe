package service

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/sushistack/yt.pipe/internal/workspace"
)

const (
	frozenDescriptorFile = "frozen_descriptor.txt"
	// fuzzyMatchThreshold is the minimum similarity ratio (0-1) for fuzzy matching.
	fuzzyMatchThreshold = 0.95
)

// FrozenDescriptorService manages the Frozen Descriptor Protocol for entity visual consistency.
type FrozenDescriptorService struct{}

// NewFrozenDescriptorService creates a new FrozenDescriptorService.
func NewFrozenDescriptorService() *FrozenDescriptorService {
	return &FrozenDescriptorService{}
}

// ExtractFromResearch extracts the Frozen Descriptor from the research stage Visual Identity Profile.
// It looks for a dense text block containing physical attributes.
// Priority: "Frozen Descriptor" section > "Visual Identity" section > "Physical Description" section.
func (s *FrozenDescriptorService) ExtractFromResearch(researchContent string) string {
	// First pass: look for explicit "Frozen Descriptor" section
	if desc := extractSection(researchContent, "frozen descriptor"); desc != "" {
		return desc
	}
	// Fallback: look for "Visual Identity" section
	if desc := extractSection(researchContent, "visual identity"); desc != "" {
		return desc
	}
	// Fallback: look for "Physical Description" section
	return extractSection(researchContent, "physical description")
}

// extractSection extracts the first paragraph of content after a section header containing the keyword.
func extractSection(content, keyword string) string {
	lines := strings.Split(content, "\n")
	var descriptorLines []string
	inSection := false

	for _, line := range lines {
		lower := strings.ToLower(line)
		trimmed := strings.TrimSpace(line)

		if !inSection {
			// Look for section header containing keyword
			if strings.Contains(lower, keyword) && (strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "**")) {
				inSection = true
				continue
			}
			continue
		}

		// In section: collect content
		// Stop at next section header
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			if len(descriptorLines) > 0 {
				break
			}
			// Skip nested headers within section
			continue
		}
		// Skip empty lines at start
		if trimmed == "" && len(descriptorLines) == 0 {
			continue
		}
		// Stop at empty line after content (paragraph break)
		if trimmed == "" && len(descriptorLines) > 0 {
			break
		}
		// Strip markdown formatting
		cleaned := stripMarkdown(trimmed)
		if cleaned != "" {
			descriptorLines = append(descriptorLines, cleaned)
		}
	}

	if len(descriptorLines) == 0 {
		return ""
	}
	return strings.Join(descriptorLines, " ")
}

// SaveToWorkspace saves the frozen descriptor to the project workspace.
func (s *FrozenDescriptorService) SaveToWorkspace(projectPath, descriptor string) error {
	path := filepath.Join(projectPath, frozenDescriptorFile)
	if err := workspace.WriteFileAtomic(path, []byte(descriptor)); err != nil {
		return fmt.Errorf("save frozen descriptor: %w", err)
	}
	slog.Info("frozen descriptor saved", "path", path, "length", len(descriptor))
	return nil
}

// LoadFromWorkspace loads the frozen descriptor from the project workspace.
func (s *FrozenDescriptorService) LoadFromWorkspace(projectPath string) (string, error) {
	path := filepath.Join(projectPath, frozenDescriptorFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("load frozen descriptor: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// ValidateInPrompt validates that the frozen descriptor appears in the final prompt.
// Returns (isValid, correctedPrompt, validationType).
// validationType: "verbatim" (exact match), "fuzzy" (similar enough), "corrected" (auto-fixed).
func (s *FrozenDescriptorService) ValidateInPrompt(prompt, frozenDescriptor string, entityVisible bool) (bool, string, string) {
	if !entityVisible || frozenDescriptor == "" {
		return true, prompt, "not_applicable"
	}

	// Tier 1: Strict verbatim match
	if strings.Contains(prompt, frozenDescriptor) {
		return true, prompt, "verbatim"
	}

	// Tier 2: Fuzzy similarity check (>=95% threshold)
	similarity := computeSimilarity(prompt, frozenDescriptor)
	if similarity >= fuzzyMatchThreshold {
		slog.Warn("frozen descriptor fuzzy match",
			"similarity", fmt.Sprintf("%.2f%%", similarity*100),
			"threshold", fmt.Sprintf("%.0f%%", fuzzyMatchThreshold*100),
		)
		return true, prompt, "fuzzy"
	}

	// Tier 3: Auto-correct by re-inserting descriptor at beginning of prompt
	slog.Warn("frozen descriptor mismatch, auto-correcting",
		"similarity", fmt.Sprintf("%.2f%%", similarity*100),
	)
	corrected := frozenDescriptor + ", " + prompt
	return false, corrected, "corrected"
}

// computeSimilarity calculates a similarity ratio between the prompt and the frozen descriptor.
// Uses a simple approach: checks how many words from the descriptor appear in the prompt.
func computeSimilarity(prompt, descriptor string) float64 {
	descriptorWords := tokenize(descriptor)
	if len(descriptorWords) == 0 {
		return 0
	}

	promptLower := strings.ToLower(prompt)
	matchCount := 0
	for _, word := range descriptorWords {
		if strings.Contains(promptLower, strings.ToLower(word)) {
			matchCount++
		}
	}

	return float64(matchCount) / float64(len(descriptorWords))
}

// tokenize splits text into meaningful words (ignoring short words and punctuation).
func tokenize(text string) []string {
	words := strings.Fields(text)
	var result []string
	for _, w := range words {
		// Strip punctuation
		cleaned := strings.TrimFunc(w, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsNumber(r)
		})
		if len(cleaned) >= 3 { // skip short words like "a", "in", "of"
			result = append(result, cleaned)
		}
	}
	return result
}

// stripMarkdown removes common markdown formatting from a line.
func stripMarkdown(line string) string {
	// Remove bullet points
	line = strings.TrimPrefix(line, "- ")
	line = strings.TrimPrefix(line, "* ")
	// Remove bold/italic markers
	line = strings.ReplaceAll(line, "**", "")
	line = strings.ReplaceAll(line, "__", "")
	line = strings.ReplaceAll(line, "*", "")
	line = strings.ReplaceAll(line, "_", " ")
	// Clean up
	line = multiSpaceRe.ReplaceAllString(line, " ")
	return strings.TrimSpace(line)
}
