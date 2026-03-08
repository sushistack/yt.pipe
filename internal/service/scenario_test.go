package service

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/mocks"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/jay/youtube-pipeline/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupScenarioService(t *testing.T) (*ScenarioService, *mocks.MockLLM) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	mockLLM := mocks.NewMockLLM(t)
	projectSvc := NewProjectService(s)
	return NewScenarioService(s, mockLLM, projectSvc), mockLLM
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
		Metadata: map[string]string{"template_version": "1.0"},
	}
}

func TestGenerateScenario_Success(t *testing.T) {
	svc, mockLLM := setupScenarioService(t)
	ctx := context.Background()
	wsPath := t.TempDir()
	scpData := sampleSCPData()

	mockLLM.On("GenerateScenario", mock.Anything, "SCP-173", scpData.MainText, mock.Anything, mock.Anything).
		Return(sampleScenarioOutput(), nil)

	scenario, project, err := svc.GenerateScenario(ctx, scpData, wsPath)
	require.NoError(t, err)

	assert.Equal(t, "SCP-173", scenario.SCPID)
	assert.Len(t, scenario.Scenes, 2)
	assert.Equal(t, domain.StatusScenarioReview, project.Status)
	assert.Equal(t, 2, project.SceneCount)

	// Verify files were created
	assert.FileExists(t, filepath.Join(wsPath, "scenario.json"))
	assert.FileExists(t, filepath.Join(wsPath, "scenario.md"))
}

func TestGenerateScenario_LLMError(t *testing.T) {
	svc, mockLLM := setupScenarioService(t)
	ctx := context.Background()
	wsPath := t.TempDir()

	mockLLM.On("GenerateScenario", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &domain.PluginError{Plugin: "openai", Operation: "generate", Err: assert.AnError})

	_, _, err := svc.GenerateScenario(ctx, sampleSCPData(), wsPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "llm generation")
}

func TestApproveScenario_Success(t *testing.T) {
	svc, mockLLM := setupScenarioService(t)
	ctx := context.Background()
	wsPath := t.TempDir()

	mockLLM.On("GenerateScenario", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(sampleScenarioOutput(), nil)

	_, project, err := svc.GenerateScenario(ctx, sampleSCPData(), wsPath)
	require.NoError(t, err)

	approved, err := svc.ApproveScenario(ctx, project.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusApproved, approved.Status)
}

func TestApproveScenario_WrongState(t *testing.T) {
	svc, _ := setupScenarioService(t)
	ctx := context.Background()

	// Create project but don't generate (status = pending)
	project, err := svc.projectSvc.CreateProject(ctx, "SCP-173", "/tmp/ws")
	require.NoError(t, err)

	_, err = svc.ApproveScenario(ctx, project.ID)
	require.Error(t, err)
	var te *domain.TransitionError
	assert.ErrorAs(t, err, &te)
}

func TestRegenerateSection_Success(t *testing.T) {
	svc, mockLLM := setupScenarioService(t)
	ctx := context.Background()
	wsPath := t.TempDir()

	mockLLM.On("GenerateScenario", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(sampleScenarioOutput(), nil)

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
	svc, _ := setupScenarioService(t)
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
