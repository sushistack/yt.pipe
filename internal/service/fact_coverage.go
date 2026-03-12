package service

import (
	"fmt"
	"sort"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// DefaultFactCoverageThreshold is the default minimum coverage percentage.
const DefaultFactCoverageThreshold = 80.0

// FactCoverageResult holds the results of fact coverage verification.
type FactCoverageResult struct {
	TotalFacts    int                `json:"total_facts"`
	CoveredFacts  int                `json:"covered_facts"`
	CoveragePct   float64            `json:"coverage_pct"`
	Threshold     float64            `json:"threshold"`
	Pass          bool               `json:"pass"`
	CoveredKeys   []string           `json:"covered_keys"`
	UncoveredKeys []string           `json:"uncovered_keys"`
	Details       []FactCoverageItem `json:"details,omitempty"`
}

// FactCoverageItem provides per-fact detail on coverage.
type FactCoverageItem struct {
	Key       string `json:"key"`
	Content   string `json:"content"`
	Covered   bool   `json:"covered"`
	SceneNums []int  `json:"scene_nums,omitempty"` // scenes where fact appears
	Category  string `json:"category,omitempty"`
}

// FactSceneSuggestion suggests which scene could incorporate a missing fact.
type FactSceneSuggestion struct {
	FactKey      string `json:"fact_key"`
	FactContent  string `json:"fact_content"`
	SuggestedScene int  `json:"suggested_scene"`
	Reason       string `json:"reason"`
}

// VerifyFactCoverage checks how many facts from the source data are referenced in the scenario.
func VerifyFactCoverage(scenario *domain.ScenarioOutput, allFacts map[string]string, threshold float64) *FactCoverageResult {
	if threshold <= 0 {
		threshold = DefaultFactCoverageThreshold
	}

	// Collect all fact keys referenced in scenario, tracking which scenes
	factScenes := make(map[string][]int)
	for _, scene := range scenario.Scenes {
		for _, ft := range scene.FactTags {
			factScenes[ft.Key] = append(factScenes[ft.Key], scene.SceneNum)
		}
	}

	totalFacts := len(allFacts)
	var coveredKeys, uncoveredKeys []string
	var details []FactCoverageItem

	// Sort keys for deterministic output
	sortedKeys := make([]string, 0, len(allFacts))
	for k := range allFacts {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		content := allFacts[key]
		scenes := factScenes[key]
		covered := len(scenes) > 0

		if covered {
			coveredKeys = append(coveredKeys, key)
		} else {
			uncoveredKeys = append(uncoveredKeys, key)
		}

		details = append(details, FactCoverageItem{
			Key:       key,
			Content:   content,
			Covered:   covered,
			SceneNums: scenes,
			Category:  categorizeFactKey(key),
		})
	}

	coveredCount := len(coveredKeys)
	var coveragePct float64
	if totalFacts > 0 {
		coveragePct = float64(coveredCount) / float64(totalFacts) * 100
	}

	return &FactCoverageResult{
		TotalFacts:    totalFacts,
		CoveredFacts:  coveredCount,
		CoveragePct:   coveragePct,
		Threshold:     threshold,
		Pass:          coveragePct >= threshold,
		CoveredKeys:   coveredKeys,
		UncoveredKeys: uncoveredKeys,
		Details:       details,
	}
}

// SuggestFactPlacements suggests which scenes could incorporate missing facts.
func SuggestFactPlacements(scenario *domain.ScenarioOutput, result *FactCoverageResult) []FactSceneSuggestion {
	if len(result.UncoveredKeys) == 0 || len(scenario.Scenes) == 0 {
		return nil
	}

	var suggestions []FactSceneSuggestion
	for _, item := range result.Details {
		if item.Covered {
			continue
		}

		// Suggest a scene based on fact category
		suggestedScene := suggestSceneForFact(scenario, item)
		suggestions = append(suggestions, FactSceneSuggestion{
			FactKey:        item.Key,
			FactContent:    item.Content,
			SuggestedScene: suggestedScene,
			Reason:         fmt.Sprintf("Scene %d covers related %s content", suggestedScene, item.Category),
		})
	}

	return suggestions
}

// FormatCoverageReport returns a human-readable report of fact coverage.
func FormatCoverageReport(r *FactCoverageResult) string {
	status := "PASS"
	if !r.Pass {
		status = "WARN"
	}

	report := fmt.Sprintf("Fact Coverage: %s (%.1f%% of %.0f%% threshold)\n", status, r.CoveragePct, r.Threshold)
	report += fmt.Sprintf("  Covered: %d/%d facts\n", r.CoveredFacts, r.TotalFacts)

	if len(r.UncoveredKeys) > 0 {
		report += "\n  Uncovered facts:\n"
		for _, key := range r.UncoveredKeys {
			report += fmt.Sprintf("    - %s\n", key)
		}
	}

	return report
}

// FormatDetailedReport returns a detailed per-fact report.
func FormatDetailedReport(r *FactCoverageResult) string {
	status := "PASS"
	if !r.Pass {
		status = "FAIL"
	}

	report := fmt.Sprintf("Fact Coverage Report: %s (%.1f%% / %.0f%% threshold)\n", status, r.CoveragePct, r.Threshold)
	report += fmt.Sprintf("Total: %d | Covered: %d | Missing: %d\n\n", r.TotalFacts, r.CoveredFacts, len(r.UncoveredKeys))

	for _, item := range r.Details {
		if item.Covered {
			report += fmt.Sprintf("  [OK] %s (scenes: %v)\n", item.Key, item.SceneNums)
		} else {
			report += fmt.Sprintf("  [--] %s (MISSING)\n", item.Key)
		}
		report += fmt.Sprintf("       %s\n", truncate(item.Content, 80))
	}

	return report
}

// categorizeFactKey attempts to categorize a fact key.
func categorizeFactKey(key string) string {
	// Ordered slice to ensure deterministic matching (first match wins).
	type catEntry struct {
		name     string
		keywords []string
	}
	categories := []catEntry{
		{"physical_description", []string{"appearance", "physical", "size", "weight", "color", "shape"}},
		{"anomalous_properties", []string{"anomal", "property", "effect", "ability", "behavior"}},
		{"containment_procedures", []string{"containment", "procedure", "protocol", "security", "cell"}},
		{"discovery", []string{"discovery", "found", "origin", "history"}},
		{"incidents", []string{"incident", "breach", "test", "log", "experiment"}},
	}

	for _, cat := range categories {
		for _, kw := range cat.keywords {
			if containsIgnoreCase(key, kw) {
				return cat.name
			}
		}
	}
	return "general"
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findLower(s, substr))
}

func findLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 'a' - 'A'
			}
			tc := substr[j]
			if tc >= 'A' && tc <= 'Z' {
				tc += 'a' - 'A'
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// suggestSceneForFact finds the best scene to place a missing fact based on content similarity.
func suggestSceneForFact(scenario *domain.ScenarioOutput, item FactCoverageItem) int {
	if len(scenario.Scenes) == 0 {
		return 1
	}

	// Default: middle scenes for general facts, early scenes for description, late for incidents
	switch item.Category {
	case "physical_description":
		return scenario.Scenes[0].SceneNum // early
	case "discovery":
		if len(scenario.Scenes) > 1 {
			return scenario.Scenes[1].SceneNum
		}
	case "incidents":
		idx := len(scenario.Scenes) * 3 / 4
		if idx >= len(scenario.Scenes) {
			idx = len(scenario.Scenes) - 1
		}
		return scenario.Scenes[idx].SceneNum
	}

	// Default to middle scene
	mid := len(scenario.Scenes) / 2
	return scenario.Scenes[mid].SceneNum
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
