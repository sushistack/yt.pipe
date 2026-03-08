package service

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestVerifyFactCoverage_FullCoverage(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, FactTags: []domain.FactTag{{Key: "containment"}, {Key: "origin"}}},
		},
	}
	facts := map[string]string{"containment": "Euclid", "origin": "Site-19"}

	result := VerifyFactCoverage(scenario, facts, 80)
	assert.True(t, result.Pass)
	assert.Equal(t, 100.0, result.CoveragePct)
	assert.Len(t, result.UncoveredKeys, 0)
}

func TestVerifyFactCoverage_PartialCoverage_Pass(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, FactTags: []domain.FactTag{{Key: "a"}, {Key: "b"}, {Key: "c"}, {Key: "d"}}},
		},
	}
	facts := map[string]string{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5"}

	result := VerifyFactCoverage(scenario, facts, 80)
	assert.True(t, result.Pass)
	assert.Equal(t, 80.0, result.CoveragePct)
}

func TestVerifyFactCoverage_BelowThreshold(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, FactTags: []domain.FactTag{{Key: "a"}}},
		},
	}
	facts := map[string]string{"a": "1", "b": "2", "c": "3"}

	result := VerifyFactCoverage(scenario, facts, 80)
	assert.False(t, result.Pass)
	assert.InDelta(t, 33.33, result.CoveragePct, 0.1)
	assert.Len(t, result.UncoveredKeys, 2)
}

func TestVerifyFactCoverage_EmptyFacts(t *testing.T) {
	scenario := &domain.ScenarioOutput{Scenes: nil}
	facts := map[string]string{}

	result := VerifyFactCoverage(scenario, facts, 80)
	assert.False(t, result.Pass) // 0% < 80%
	assert.Equal(t, 0, result.TotalFacts)
}

func TestVerifyFactCoverage_DefaultThreshold(t *testing.T) {
	scenario := &domain.ScenarioOutput{Scenes: nil}
	facts := map[string]string{"a": "1"}

	result := VerifyFactCoverage(scenario, facts, 0)
	assert.Equal(t, DefaultFactCoverageThreshold, result.Threshold)
}

func TestVerifyFactCoverage_CrossSceneFacts(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, FactTags: []domain.FactTag{{Key: "a"}}},
			{SceneNum: 2, FactTags: []domain.FactTag{{Key: "b"}}},
			{SceneNum: 3, FactTags: []domain.FactTag{{Key: "a"}}}, // duplicate
		},
	}
	facts := map[string]string{"a": "1", "b": "2"}

	result := VerifyFactCoverage(scenario, facts, 80)
	assert.True(t, result.Pass)
	assert.Equal(t, 2, result.CoveredFacts)
}

func TestVerifyFactCoverage_Details(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, FactTags: []domain.FactTag{{Key: "containment"}}},
			{SceneNum: 3, FactTags: []domain.FactTag{{Key: "containment"}}},
		},
	}
	facts := map[string]string{"containment": "Euclid", "origin": "Site-19"}

	result := VerifyFactCoverage(scenario, facts, 50.0)
	assert.Len(t, result.Details, 2)

	for _, d := range result.Details {
		if d.Key == "containment" {
			assert.True(t, d.Covered)
			assert.Contains(t, d.SceneNums, 1)
			assert.Contains(t, d.SceneNums, 3)
		}
		if d.Key == "origin" {
			assert.False(t, d.Covered)
			assert.Empty(t, d.SceneNums)
		}
	}
}

func TestSuggestFactPlacements(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1}, {SceneNum: 2}, {SceneNum: 3}, {SceneNum: 4},
		},
	}
	facts := map[string]string{"appearance": "Concrete", "origin": "Site-19"}
	result := VerifyFactCoverage(scenario, facts, 80.0)

	suggestions := SuggestFactPlacements(scenario, result)
	assert.NotEmpty(t, suggestions)
	for _, s := range suggestions {
		assert.NotZero(t, s.SuggestedScene)
		assert.NotEmpty(t, s.FactKey)
	}
}

func TestFormatCoverageReport_Pass(t *testing.T) {
	result := &FactCoverageResult{
		TotalFacts:   5,
		CoveredFacts: 4,
		CoveragePct:  80.0,
		Threshold:    80.0,
		Pass:         true,
	}
	report := FormatCoverageReport(result)
	assert.Contains(t, report, "PASS")
	assert.Contains(t, report, "80.0%")
}

func TestFormatCoverageReport_Warn(t *testing.T) {
	result := &FactCoverageResult{
		TotalFacts:    5,
		CoveredFacts:  2,
		CoveragePct:   40.0,
		Threshold:     80.0,
		Pass:          false,
		UncoveredKeys: []string{"fact1", "fact2", "fact3"},
	}
	report := FormatCoverageReport(result)
	assert.Contains(t, report, "WARN")
	assert.Contains(t, report, "fact1")
}

func TestFormatDetailedReport(t *testing.T) {
	result := &FactCoverageResult{
		TotalFacts:    3,
		CoveredFacts:  2,
		CoveragePct:   66.7,
		Threshold:     80.0,
		Pass:          false,
		CoveredKeys:   []string{"a", "b"},
		UncoveredKeys: []string{"c"},
		Details: []FactCoverageItem{
			{Key: "a", Content: "fact a", Covered: true, SceneNums: []int{1}},
			{Key: "b", Content: "fact b", Covered: true, SceneNums: []int{2, 3}},
			{Key: "c", Content: "fact c", Covered: false},
		},
	}

	report := FormatDetailedReport(result)
	assert.Contains(t, report, "FAIL")
	assert.Contains(t, report, "66.7%")
	assert.Contains(t, report, "[OK] a")
	assert.Contains(t, report, "[--] c")
}

func TestCategorizeFactKey(t *testing.T) {
	assert.Equal(t, "physical_description", categorizeFactKey("appearance"))
	assert.Equal(t, "anomalous_properties", categorizeFactKey("anomalous_behavior"))
	assert.Equal(t, "containment_procedures", categorizeFactKey("containment_protocol"))
	assert.Equal(t, "discovery", categorizeFactKey("discovery_log"))
	assert.Equal(t, "incidents", categorizeFactKey("incident_report"))
	assert.Equal(t, "general", categorizeFactKey("some_random_key"))
}
