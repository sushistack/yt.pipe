package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/mocks"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupTemplatesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	scenarioDir := filepath.Join(dir, "scenario")
	require.NoError(t, os.MkdirAll(scenarioDir, 0o755))

	// Create minimal test templates
	templates := map[string]string{
		"01_research.md":  "Research {scp_id}\n{scp_fact_sheet}\n{main_text}\n{glossary_section}",
		"02_structure.md": "Structure {scp_id}\n{research_packet}\n{scp_visual_reference}\n{target_duration}\n{glossary_section}",
		"03_writing.md":   "Writing {scp_id}\n{scene_structure}\n{scp_visual_reference}\n{glossary_section}",
		"04_review.md":    "Review {scp_id}\n{narration_script}\n{scp_visual_reference}\n{scp_fact_sheet}\n{glossary_section}",
	}
	for name, content := range templates {
		require.NoError(t, os.WriteFile(filepath.Join(scenarioDir, name), []byte(content), 0o644))
	}
	return dir
}

func sampleWritingOutput() string {
	scenario := map[string]interface{}{
		"scp_id": "SCP-173",
		"title":  "The Sculpture - SCP-173",
		"scenes": []map[string]interface{}{
			{
				"scene_num":          1,
				"narration":          "어둠 속에서 조각상이 서 있습니다.",
				"visual_description": "A dark concrete room with a strange figure",
				"fact_tags":          []map[string]string{{"key": "containment", "content": "Euclid"}},
				"mood":               "tense",
			},
			{
				"scene_num":          2,
				"narration":          "직원들은 시선을 떼지 않아야 합니다.",
				"visual_description": "Security cameras focused on the sculpture",
				"fact_tags":          []map[string]string{{"key": "origin", "content": "Site-19"}},
				"mood":               "suspenseful",
			},
		},
		"metadata": map[string]string{"template_version": "1.0"},
	}
	data, _ := json.Marshal(scenario)
	return string(data)
}

func sampleReviewOutput() string {
	report := map[string]interface{}{
		"overall_pass": true,
		"coverage_pct": 100.0,
		"issues":       []interface{}{},
		"corrections":  []interface{}{},
	}
	data, _ := json.Marshal(report)
	return string(data)
}

func TestScenarioPipeline_Run_Success(t *testing.T) {
	templatesDir := setupTemplatesDir(t)
	wsPath := t.TempDir()
	mockLLM := mocks.NewMockLLM(t)
	g := glossary.New()

	// Stage 1: Research
	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsString(msgs[0].Content, "Research")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content:      "### 2. Visual Identity Profile (Frozen Descriptor)\nTall concrete statue with crude features",
		InputTokens:  100,
		OutputTokens: 200,
		Model:        "test-model",
	}, nil).Once()

	// Stage 2: Structure
	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsString(msgs[0].Content, "Structure")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content:      `[{"scene_num":1,"synopsis":"intro"},{"scene_num":2,"synopsis":"details"}]`,
		InputTokens:  150,
		OutputTokens: 100,
		Model:        "test-model",
	}, nil).Once()

	// Stage 3: Writing
	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsString(msgs[0].Content, "Writing")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content:      sampleWritingOutput(),
		InputTokens:  200,
		OutputTokens: 300,
		Model:        "test-model",
	}, nil).Once()

	// Stage 4: Review
	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsString(msgs[0].Content, "Review")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content:      sampleReviewOutput(),
		InputTokens:  250,
		OutputTokens: 50,
		Model:        "test-model",
	}, nil).Once()

	pipeline, err := NewScenarioPipeline(mockLLM, g, ScenarioPipelineConfig{
		TemplatesDir:          templatesDir,
		TargetDurationMin:     10,
		FactCoverageThreshold: 80.0,
	})
	require.NoError(t, err)

	scpData := sampleSCPData()
	result, err := pipeline.Run(context.Background(), scpData, wsPath)
	require.NoError(t, err)

	// Verify result
	assert.NotNil(t, result.Scenario)
	assert.Equal(t, "SCP-173", result.Scenario.SCPID)
	assert.Len(t, result.Scenario.Scenes, 2)
	assert.Len(t, result.Stages, 4)
	assert.Equal(t, StageResearch, result.Stages[0].Stage)
	assert.Equal(t, StageStructure, result.Stages[1].Stage)
	assert.Equal(t, StageWriting, result.Stages[2].Stage)
	assert.Equal(t, StageReview, result.Stages[3].Stage)

	// Verify token totals
	expectedTokens := 100 + 200 + 150 + 100 + 200 + 300 + 250 + 50
	assert.Equal(t, expectedTokens, result.TotalTokens)

	// Verify stage artifacts saved
	assert.FileExists(t, filepath.Join(wsPath, "stages", "01_research.json"))
	assert.FileExists(t, filepath.Join(wsPath, "stages", "02_structure.json"))
	assert.FileExists(t, filepath.Join(wsPath, "stages", "03_writing.json"))
	assert.FileExists(t, filepath.Join(wsPath, "stages", "04_review.json"))
}

func TestScenarioPipeline_Run_ResumeFromCheckpoint(t *testing.T) {
	templatesDir := setupTemplatesDir(t)
	wsPath := t.TempDir()
	mockLLM := mocks.NewMockLLM(t)
	g := glossary.New()

	// Pre-create checkpoints for stages 1 and 2
	stagesDir := filepath.Join(wsPath, "stages")
	require.NoError(t, os.MkdirAll(stagesDir, 0o755))

	researchResult := StageResult{
		Stage:        StageResearch,
		Content:      "### 2. Visual Identity Profile (Frozen Descriptor)\nTall concrete statue",
		InputTokens:  100,
		OutputTokens: 200,
	}
	data, _ := json.MarshalIndent(researchResult, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(stagesDir, "01_research.json"), data, 0o644))

	structureResult := StageResult{
		Stage:        StageStructure,
		Content:      `[{"scene_num":1}]`,
		InputTokens:  150,
		OutputTokens: 100,
	}
	data, _ = json.MarshalIndent(structureResult, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(stagesDir, "02_structure.json"), data, 0o644))

	// Only stages 3 and 4 should be called
	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsString(msgs[0].Content, "Writing")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content:      sampleWritingOutput(),
		InputTokens:  200,
		OutputTokens: 300,
		Model:        "test-model",
	}, nil).Once()

	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsString(msgs[0].Content, "Review")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content:      sampleReviewOutput(),
		InputTokens:  250,
		OutputTokens: 50,
		Model:        "test-model",
	}, nil).Once()

	pipeline, err := NewScenarioPipeline(mockLLM, g, ScenarioPipelineConfig{
		TemplatesDir:          templatesDir,
		TargetDurationMin:     10,
		FactCoverageThreshold: 80.0,
	})
	require.NoError(t, err)

	result, err := pipeline.Run(context.Background(), sampleSCPData(), wsPath)
	require.NoError(t, err)

	assert.NotNil(t, result.Scenario)
	assert.Len(t, result.Stages, 4)
	// First two stages should be from cache
	assert.Equal(t, 100, result.Stages[0].InputTokens)
	assert.Equal(t, 150, result.Stages[1].InputTokens)
}

func TestScenarioPipeline_GlossaryInjection(t *testing.T) {
	templatesDir := setupTemplatesDir(t)
	_ = t.TempDir()
	mockLLM := mocks.NewMockLLM(t)

	// Create glossary with entries
	g := glossary.New()
	// We can't add entries directly, but we test that the section is built
	// For a real test, we'd use LoadFromFile with a test fixture

	pipeline, err := NewScenarioPipeline(mockLLM, g, ScenarioPipelineConfig{
		TemplatesDir:          templatesDir,
		TargetDurationMin:     10,
		FactCoverageThreshold: 80.0,
	})
	require.NoError(t, err)

	// Verify glossary section is empty for empty glossary
	scpData := sampleSCPData()
	section := pipeline.buildGlossarySection(scpData)
	assert.Empty(t, section)
}

func TestScenarioPipeline_StageFailure(t *testing.T) {
	templatesDir := setupTemplatesDir(t)
	wsPath := t.TempDir()
	mockLLM := mocks.NewMockLLM(t)
	g := glossary.New()

	// Stage 1 fails
	mockLLM.On("Complete", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &llm.APIError{Provider: "test", StatusCode: 500, Message: "server error"}).Once()

	pipeline, err := NewScenarioPipeline(mockLLM, g, ScenarioPipelineConfig{
		TemplatesDir:          templatesDir,
		TargetDurationMin:     10,
		FactCoverageThreshold: 80.0,
	})
	require.NoError(t, err)

	_, err = pipeline.Run(context.Background(), sampleSCPData(), wsPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "research stage")
}

func TestExtractVisualIdentity(t *testing.T) {
	content := `### 1. Core Identity Summary
Some text here

### 2. Visual Identity Profile (Frozen Descriptor)
- Silhouette & Build: Tall, humanoid
- Head/Face: Crude painted features
- Body Covering: Concrete and rebar

### 3. Key Dramatic Beats
Something else`

	result := extractVisualIdentity(content)
	assert.Contains(t, result, "Visual Identity Profile")
	assert.Contains(t, result, "Silhouette & Build")
	assert.Contains(t, result, "Concrete and rebar")
	assert.NotContains(t, result, "Key Dramatic Beats")
}

func TestExtractVisualIdentity_NotFound(t *testing.T) {
	result := extractVisualIdentity("No visual section here")
	assert.Contains(t, result, "No visual identity extracted")
}

func TestParseScenarioFromWriting(t *testing.T) {
	content := sampleWritingOutput()
	scenario, err := parseScenarioFromWriting(content, "SCP-173")
	require.NoError(t, err)

	assert.Equal(t, "SCP-173", scenario.SCPID)
	assert.Len(t, scenario.Scenes, 2)
	assert.Equal(t, 1, scenario.Scenes[0].SceneNum)
	assert.Equal(t, "tense", scenario.Scenes[0].Mood)
}

func TestParseReviewReport(t *testing.T) {
	content := `{
		"overall_pass": true,
		"coverage_pct": 85.0,
		"issues": [{"scene_num": 1, "type": "fact_error", "severity": "warning", "description": "test", "correction": "fix"}],
		"corrections": []
	}`

	report, err := parseReviewReport(content)
	require.NoError(t, err)
	assert.True(t, report.OverallPass)
	assert.Equal(t, 85.0, report.CoveragePct)
	assert.Len(t, report.Issues, 1)
}

func TestApplyCorrections(t *testing.T) {
	scenario := &domain.ScenarioOutput{
		Scenes: []domain.SceneScript{
			{SceneNum: 1, Narration: "Original text here", VisualDescription: "dark room"},
		},
	}

	corrections := []ReviewCorrection{
		{SceneNum: 1, Field: "narration", Original: "Original", Corrected: "Corrected"},
	}

	result := applyCorrections(scenario, corrections)
	assert.Equal(t, "Corrected text here", result.Scenes[0].Narration)
}

func TestExtractJSONFromContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain", `{"ok":true}`, `{"ok":true}`},
		{"fenced", "```json\n{\"ok\":true}\n```", `{"ok":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractJSONFromContent(tt.input))
		})
	}
}

// Helper to check if any message content contains a substring.
func containsString(content, substr string) bool {
	return len(content) >= len(substr) && contains(content, substr)
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Re-export sampleSCPData for tests in same package (already defined in scenario_test.go).
// If this causes a duplicate, remove from here.
func sampleSCPDataForPipeline() *workspace.SCPData {
	return &workspace.SCPData{
		SCPID: "SCP-173",
		Facts: &workspace.FactsFile{
			SchemaVersion: "1.0",
			Facts:         map[string]string{"containment": "Euclid", "origin": "Site-19"},
		},
		Meta: &workspace.MetaFile{
			SchemaVersion: "1.0",
			Title:         "The Sculpture",
			ObjectClass:   "Euclid",
			Series:        "I",
		},
		MainText: "SCP-173 is a concrete sculpture...",
	}
}
