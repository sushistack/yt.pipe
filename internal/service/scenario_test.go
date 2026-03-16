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
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/sushistack/yt.pipe/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupScenarioService(t *testing.T) (*ScenarioService, *mocks.MockLLM, string) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	mockLLM := mocks.NewMockLLM(t)
	projectSvc := NewProjectService(s)
	svc := NewScenarioService(s, mockLLM, projectSvc)

	// Create templates dir with minimal templates for 4-stage pipeline
	tplDir := t.TempDir()
	scenarioDir := filepath.Join(tplDir, "scenario")
	require.NoError(t, os.MkdirAll(scenarioDir, 0o755))
	templates := map[string]string{
		"01_research.md":  "Research {scp_id}\n{scp_fact_sheet}\n{main_text}\n{glossary_section}\n{format_guide}",
		"02_structure.md": "Structure {scp_id}\n{research_packet}\n{scp_visual_reference}\n{target_duration}\n{glossary_section}\n{format_guide}",
		"03_writing.md":   "Writing {scp_id}\n{scene_structure}\n{scp_visual_reference}\n{glossary_section}\n{format_guide}\n{quality_feedback}",
		"04_review.md":    "Review {scp_id}\n{narration_script}\n{scp_visual_reference}\n{scp_fact_sheet}\n{glossary_section}",
	}
	for name, content := range templates {
		require.NoError(t, os.WriteFile(filepath.Join(scenarioDir, name), []byte(content), 0o644))
	}
	svc.SetTemplatesDir(tplDir)
	svc.SetGlossary(glossary.New())

	return svc, mockLLM, tplDir
}

// setup4StageMocks configures mock LLM to handle all 4 stages of the pipeline.
func setup4StageMocks(mockLLM *mocks.MockLLM) {
	// Stage 1: Research
	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsString(msgs[0].Content, "Research")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content: "### Visual Identity Profile\nTall concrete statue", InputTokens: 100, OutputTokens: 200, Model: "test",
	}, nil).Maybe()

	// Stage 2: Structure
	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsString(msgs[0].Content, "Structure")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content: `[{"scene_num":1},{"scene_num":2}]`, InputTokens: 100, OutputTokens: 100, Model: "test",
	}, nil).Maybe()

	// Stage 3: Writing
	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsString(msgs[0].Content, "Writing")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content: sampleWritingOutput(), InputTokens: 200, OutputTokens: 300, Model: "test",
	}, nil).Maybe()

	// Stage 4: Review
	mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(msgs []llm.Message) bool {
		return len(msgs) > 0 && containsString(msgs[0].Content, "Review")
	}), mock.Anything).Return(&llm.CompletionResult{
		Content: sampleReviewOutput(), InputTokens: 100, OutputTokens: 50, Model: "test",
	}, nil).Maybe()
}

func sampleSCPData() *workspace.SCPData {
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

func sampleScenarioOutput() *domain.ScenarioOutput {
	return &domain.ScenarioOutput{
		SCPID: "SCP-173",
		Title: "The Sculpture - SCP-173",
		Scenes: []domain.SceneScript{
			{
				SceneNum:          1,
				Narration:         "In a dimly lit containment cell...",
				VisualDescription: "A dark concrete room with a strange figure",
				FactTags:          []domain.FactTag{{Key: "containment", Content: "Euclid"}},
				Mood:              "tense",
			},
			{
				SceneNum:          2,
				Narration:         "Personnel must maintain visual contact...",
				VisualDescription: "Security cameras focused on the sculpture",
				FactTags:          []domain.FactTag{{Key: "origin", Content: "Site-19"}},
				Mood:              "suspenseful",
			},
		},
		Metadata: map[string]any{"template_version": "1.0"},
	}
}

func TestGenerateScenario_Success(t *testing.T) {
	svc, mockLLM, _ := setupScenarioService(t)
	ctx := context.Background()
	wsPath := t.TempDir()

	setup4StageMocks(mockLLM)

	scenario, project, err := svc.GenerateScenario(ctx, sampleSCPData(), wsPath)
	require.NoError(t, err)

	assert.Equal(t, "SCP-173", scenario.SCPID)
	assert.Len(t, scenario.Scenes, 2)
	assert.Equal(t, domain.StageScenario, project.Status)
	assert.Equal(t, 2, project.SceneCount)
	assert.Equal(t, "4-stage", scenario.Metadata["pipeline_mode"])

	// Verify files were created
	assert.FileExists(t, filepath.Join(wsPath, "scenario.json"))
	assert.FileExists(t, filepath.Join(wsPath, "scenario.md"))
}

func TestGenerateScenario_NoTemplatesPath(t *testing.T) {
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	mockLLM := mocks.NewMockLLM(t)
	projectSvc := NewProjectService(s)
	svc := NewScenarioService(s, mockLLM, projectSvc)
	// NOT setting templates dir → should fail with clear error

	_, _, err = svc.GenerateScenario(context.Background(), sampleSCPData(), t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "templates_path is not configured")
}

func TestApproveScenario_Success(t *testing.T) {
	svc, mockLLM, _ := setupScenarioService(t)
	ctx := context.Background()
	wsPath := t.TempDir()

	setup4StageMocks(mockLLM)

	_, project, err := svc.GenerateScenario(ctx, sampleSCPData(), wsPath)
	require.NoError(t, err)

	approved, err := svc.ApproveScenario(ctx, project.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StageScenario, approved.Status)
}

func TestApproveScenario_NoStateGate(t *testing.T) {
	svc, _, _ := setupScenarioService(t)
	ctx := context.Background()

	project, err := svc.projectSvc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	got, err := svc.ApproveScenario(ctx, project.ID)
	require.NoError(t, err)
	assert.Equal(t, project.ID, got.ID)
}

func TestRegenerateSection_Success(t *testing.T) {
	svc, mockLLM, _ := setupScenarioService(t)
	ctx := context.Background()
	wsPath := t.TempDir()

	setup4StageMocks(mockLLM)

	newScene := &domain.SceneScript{
		SceneNum:          1,
		Narration:         "UPDATED narration",
		VisualDescription: "UPDATED visual",
		Mood:              "horror",
	}
	mockLLM.On("RegenerateSection", mock.Anything, mock.Anything, 1, "make it scarier").
		Return(newScene, nil)

	_, project, err := svc.GenerateScenario(ctx, sampleSCPData(), wsPath)
	require.NoError(t, err)

	result, err := svc.RegenerateSection(ctx, project.ID, 1, "make it scarier")
	require.NoError(t, err)
	assert.Equal(t, "UPDATED narration", result.Narration)
}

func TestRegenerateSection_WrongState(t *testing.T) {
	svc, _, _ := setupScenarioService(t)
	ctx := context.Background()

	project, err := svc.projectSvc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	_, err = svc.RegenerateSection(ctx, project.ID, 1, "test")
	require.Error(t, err)
}

func TestLoadScenarioFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scenario.json")

	scenario := sampleScenarioOutput()
	data, err := json.MarshalIndent(scenario, "", "  ")
	require.NoError(t, err)
	require.NoError(t, workspace.WriteFileAtomic(path, data))

	loaded, err := LoadScenarioFromFile(path)
	require.NoError(t, err)
	assert.Equal(t, "SCP-173", loaded.SCPID)
	assert.Len(t, loaded.Scenes, 2)
}
