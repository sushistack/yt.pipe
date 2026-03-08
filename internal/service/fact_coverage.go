package service

import (
	"fmt"

	"github.com/jay/youtube-pipeline/internal/domain"
)

// DefaultFactCoverageThreshold is the default minimum coverage percentage.
const DefaultFactCoverageThreshold = 80.0

// FactCoverageResult holds the results of fact coverage verification.
type FactCoverageResult struct {
	TotalFacts    int      `json:"total_facts"`
	CoveredFacts  int      `json:"covered_facts"`
	CoveragePct   float64  `json:"coverage_pct"`
	Threshold     float64  `json:"threshold"`
	Pass          bool     `json:"pass"`
	CoveredKeys   []string `json:"covered_keys"`
	UncoveredKeys []string `json:"uncovered_keys"`
}

// VerifyFactCoverage checks how many facts from the source data are referenced in the scenario.
func VerifyFactCoverage(scenario *domain.ScenarioOutput, allFacts map[string]string, threshold float64) *FactCoverageResult {
	if threshold <= 0 {
		threshold = DefaultFactCoverageThreshold
	}

	// Collect all fact keys referenced in scenario
	referenced := make(map[string]bool)
	for _, scene := range scenario.Scenes {
		for _, ft := range scene.FactTags {
			referenced[ft.Key] = true
		}
	}

	totalFacts := len(allFacts)
	var coveredKeys, uncoveredKeys []string

	for key := range allFacts {
		if referenced[key] {
			coveredKeys = append(coveredKeys, key)
		} else {
			uncoveredKeys = append(uncoveredKeys, key)
		}
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
	}
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
