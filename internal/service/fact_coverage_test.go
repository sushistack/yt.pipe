package service

import (
	"testing"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestVerifyFactCoverage_FullCoverage(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{FactTags: []domain.FactTag{{Key: "containment"}, {Key: "origin"}}},
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
			{FactTags: []domain.FactTag{{Key: "a"}, {Key: "b"}, {Key: "c"}, {Key: "d"}}},
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
			{FactTags: []domain.FactTag{{Key: "a"}}},
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
			{FactTags: []domain.FactTag{{Key: "a"}}},
			{FactTags: []domain.FactTag{{Key: "b"}}},
			{FactTags: []domain.FactTag{{Key: "a"}}}, // duplicate
		},
	}
	facts := map[string]string{"a": "1", "b": "2"}

	result := VerifyFactCoverage(scenario, facts, 80)
	assert.True(t, result.Pass)
	assert.Equal(t, 2, result.CoveredFacts)
}

func TestFormatCoverageReport_Pass(t *testing.T) {
	result := &FactCoverageResult{
		TotalFacts:  5,
		CoveredFacts: 4,
		CoveragePct: 80.0,
		Threshold:   80.0,
		Pass:        true,
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
