package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/mocks"
	"github.com/jay/youtube-pipeline/internal/plugin/output"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/jay/youtube-pipeline/internal/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupAssemblerService(t *testing.T) (*AssemblerService, *mocks.MockAssembler, *ProjectService) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	mockAsm := mocks.NewMockAssembler(t)
	projectSvc := NewProjectService(s)
	return NewAssemblerService(mockAsm, projectSvc), mockAsm, projectSvc
}

// transitionToGeneratingAssets is a test helper to advance project state.
func transitionToGeneratingAssets(t *testing.T, ctx context.Context, projectSvc *ProjectService, projectID string) {
	t.Helper()
	_, err := projectSvc.TransitionProject(ctx, projectID, domain.StatusScenarioReview)
	require.NoError(t, err)
	_, err = projectSvc.TransitionProject(ctx, projectID, domain.StatusApproved)
	require.NoError(t, err)
	_, err = projectSvc.TransitionProject(ctx, projectID, domain.StatusGeneratingAssets)
	require.NoError(t, err)
}

func validScenes() []domain.Scene {
	return []domain.Scene{
		{SceneNum: 1, ImagePath: "/img/1.png", AudioPath: "/audio/1.mp3", SubtitlePath: "/sub/1.srt", AudioDuration: 5.0},
	}
}

func mockAssembleResult(outputPath string, sceneCount int) *output.AssembleResult {
	return &output.AssembleResult{
		OutputPath:    outputPath,
		SceneCount:    sceneCount,
		TotalDuration: 5.0 * float64(sceneCount),
		ImageCount:    sceneCount,
		AudioCount:    sceneCount,
		SubtitleCount: sceneCount,
	}
}

func TestAssemble_Success(t *testing.T) {
	svc, mockAsm, projectSvc := setupAssemblerService(t)
	ctx := context.Background()
	wsPath := t.TempDir()

	project, err := projectSvc.CreateProject(ctx, "SCP-173", wsPath)
	require.NoError(t, err)
	transitionToGeneratingAssets(t, ctx, projectSvc, project.ID)

	expectedOutput := filepath.Join(wsPath, "output", "draft_content.json")
	expectedResult := mockAssembleResult(expectedOutput, 1)
	mockAsm.On("Assemble", mock.Anything, mock.MatchedBy(func(input output.AssembleInput) bool {
		return input.Project.ID == project.ID && input.Canvas.Width == 1920
	})).Return(expectedResult, nil)
	mockAsm.On("Validate", mock.Anything, expectedOutput).Return(nil)

	result, err := svc.Assemble(ctx, project.ID, validScenes())
	require.NoError(t, err)
	assert.Equal(t, expectedOutput, result.OutputPath)
	assert.Equal(t, 1, result.SceneCount)
	assert.Equal(t, 5.0, result.TotalDuration)

	// Verify project reached complete state
	updated, err := projectSvc.GetProject(ctx, project.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusComplete, updated.Status)
}

func TestAssemble_MultipleScenes(t *testing.T) {
	svc, mockAsm, projectSvc := setupAssemblerService(t)
	ctx := context.Background()
	wsPath := t.TempDir()

	project, err := projectSvc.CreateProject(ctx, "SCP-999", wsPath)
	require.NoError(t, err)
	transitionToGeneratingAssets(t, ctx, projectSvc, project.ID)

	scenes := []domain.Scene{
		{SceneNum: 1, ImagePath: "/img/1.png", AudioPath: "/audio/1.mp3", SubtitlePath: "/sub/1.srt", AudioDuration: 5.0},
		{SceneNum: 2, ImagePath: "/img/2.png", AudioPath: "/audio/2.mp3", SubtitlePath: "/sub/2.srt", AudioDuration: 4.0},
		{SceneNum: 3, ImagePath: "/img/3.png", AudioPath: "/audio/3.mp3", SubtitlePath: "/sub/3.srt", AudioDuration: 6.0},
	}

	expectedOutput := filepath.Join(wsPath, "output", "draft_content.json")
	expectedResult := mockAssembleResult(expectedOutput, 3)
	mockAsm.On("Assemble", mock.Anything, mock.MatchedBy(func(input output.AssembleInput) bool {
		return len(input.Scenes) == 3
	})).Return(expectedResult, nil)
	mockAsm.On("Validate", mock.Anything, expectedOutput).Return(nil)

	result, err := svc.Assemble(ctx, project.ID, scenes)
	require.NoError(t, err)
	assert.Equal(t, 3, result.SceneCount)
}

func TestAssemble_EmptyScenes(t *testing.T) {
	svc, _, projectSvc := setupAssemblerService(t)
	ctx := context.Background()

	project, err := projectSvc.CreateProject(ctx, "SCP-173", t.TempDir())
	require.NoError(t, err)

	_, err = svc.Assemble(ctx, project.ID, nil)
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, "no scenes")
}

func TestAssemble_MissingImage(t *testing.T) {
	svc, _, projectSvc := setupAssemblerService(t)
	ctx := context.Background()

	project, err := projectSvc.CreateProject(ctx, "SCP-173", t.TempDir())
	require.NoError(t, err)

	scenes := []domain.Scene{
		{SceneNum: 1, ImagePath: "", AudioPath: "/audio/1.mp3", SubtitlePath: "/sub/1.srt"},
	}

	_, err = svc.Assemble(ctx, project.ID, scenes)
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, "missing image")
}

func TestAssemble_MissingAudio(t *testing.T) {
	svc, _, projectSvc := setupAssemblerService(t)
	ctx := context.Background()

	project, err := projectSvc.CreateProject(ctx, "SCP-173", t.TempDir())
	require.NoError(t, err)

	scenes := []domain.Scene{
		{SceneNum: 1, ImagePath: "/img/1.png", AudioPath: "", SubtitlePath: "/sub/1.srt"},
	}

	_, err = svc.Assemble(ctx, project.ID, scenes)
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, "missing audio")
}

func TestAssemble_MissingSubtitle(t *testing.T) {
	svc, _, projectSvc := setupAssemblerService(t)
	ctx := context.Background()

	project, err := projectSvc.CreateProject(ctx, "SCP-173", t.TempDir())
	require.NoError(t, err)

	scenes := []domain.Scene{
		{SceneNum: 1, ImagePath: "/img/1.png", AudioPath: "/audio/1.mp3", SubtitlePath: ""},
	}

	_, err = svc.Assemble(ctx, project.ID, scenes)
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, "missing subtitle")
}

func TestAssemble_MultipleAssetErrors(t *testing.T) {
	svc, _, projectSvc := setupAssemblerService(t)
	ctx := context.Background()

	project, err := projectSvc.CreateProject(ctx, "SCP-173", t.TempDir())
	require.NoError(t, err)

	scenes := []domain.Scene{
		{SceneNum: 1, ImagePath: "", AudioPath: "", SubtitlePath: ""},
	}

	_, err = svc.Assemble(ctx, project.ID, scenes)
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, "missing image")
	assert.Contains(t, ve.Message, "missing audio")
	assert.Contains(t, ve.Message, "missing subtitle")
}

func TestAssemble_ValidationFailure(t *testing.T) {
	svc, mockAsm, projectSvc := setupAssemblerService(t)
	ctx := context.Background()
	wsPath := t.TempDir()

	project, err := projectSvc.CreateProject(ctx, "SCP-173", wsPath)
	require.NoError(t, err)
	transitionToGeneratingAssets(t, ctx, projectSvc, project.ID)

	expectedOutput := filepath.Join(wsPath, "output", "draft_content.json")
	expectedResult := mockAssembleResult(expectedOutput, 1)
	mockAsm.On("Assemble", mock.Anything, mock.Anything).Return(expectedResult, nil)
	mockAsm.On("Validate", mock.Anything, expectedOutput).Return(
		&domain.ValidationError{Field: "tracks", Message: "expected 3 tracks, got 0"})

	_, err = svc.Assemble(ctx, project.ID, validScenes())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "assembler validation")
}

func TestAssemble_WithConfig(t *testing.T) {
	svc, mockAsm, projectSvc := setupAssemblerService(t)
	ctx := context.Background()
	wsPath := t.TempDir()

	svc.WithConfig("/templates/draft.json", "/templates/meta.json", output.CanvasConfig{
		Width: 1280, Height: 720, FPS: 24.0,
	})

	project, err := projectSvc.CreateProject(ctx, "SCP-173", wsPath)
	require.NoError(t, err)
	transitionToGeneratingAssets(t, ctx, projectSvc, project.ID)

	expectedOutput := filepath.Join(wsPath, "output", "draft_content.json")
	expectedResult := mockAssembleResult(expectedOutput, 1)
	mockAsm.On("Assemble", mock.Anything, mock.MatchedBy(func(input output.AssembleInput) bool {
		return input.TemplatePath == "/templates/draft.json" &&
			input.MetaPath == "/templates/meta.json" &&
			input.Canvas.Width == 1280 &&
			input.Canvas.FPS == 24.0
	})).Return(expectedResult, nil)
	mockAsm.On("Validate", mock.Anything, expectedOutput).Return(nil)

	result, err := svc.Assemble(ctx, project.ID, validScenes())
	require.NoError(t, err)
	assert.Equal(t, expectedOutput, result.OutputPath)
}

func TestGenerateCopyrightNotice(t *testing.T) {
	svc, _, _ := setupAssemblerService(t)
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "output"), 0o755))

	err := svc.GenerateCopyrightNotice(dir, "SCP-173", "Moto42")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "output", "description.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "CC-BY-SA 3.0")
	assert.Contains(t, string(data), "Moto42")
	assert.Contains(t, string(data), "SCP-173")
}

func TestGenerateCopyrightNotice_EmptyAuthor(t *testing.T) {
	svc, _, _ := setupAssemblerService(t)
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "output"), 0o755))

	err := svc.GenerateCopyrightNotice(dir, "SCP-173", "")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "output", "description.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "Unknown")
}

func TestCheckSpecialCopyright_None(t *testing.T) {
	meta := &workspace.MetaFile{Title: "Test"}
	_, hasCopyright := CheckSpecialCopyright(meta)
	assert.False(t, hasCopyright)
}

func TestCheckSpecialCopyright_HasNotes(t *testing.T) {
	meta := &workspace.MetaFile{
		Title:          "SCP-999",
		CopyrightNotes: "Images licensed under CC-BY-NC 4.0",
	}
	notes, hasCopyright := CheckSpecialCopyright(meta)
	assert.True(t, hasCopyright)
	assert.Equal(t, "Images licensed under CC-BY-NC 4.0", notes)
}

func TestLogSpecialCopyright_NoSpecialConditions(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "output"), 0o755))

	meta := &workspace.MetaFile{Title: "Test"}
	err := LogSpecialCopyright(dir, "SCP-173", meta)
	require.NoError(t, err)

	// No warning file should be created
	_, err = os.Stat(filepath.Join(dir, "output", "copyright_warning.json"))
	assert.True(t, os.IsNotExist(err))
}

func TestLogSpecialCopyright_WithSpecialConditions(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "output"), 0o755))

	meta := &workspace.MetaFile{
		Title:          "SCP-999",
		CopyrightNotes: "Images licensed under CC-BY-NC 4.0",
	}
	err := LogSpecialCopyright(dir, "SCP-999", meta)
	require.NoError(t, err)

	// Warning file should exist with copyright details
	data, err := os.ReadFile(filepath.Join(dir, "output", "copyright_warning.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "SCP-999")
	assert.Contains(t, string(data), "CC-BY-NC 4.0")
	assert.Contains(t, string(data), "additional copyright conditions")
}

func TestGenerateCopyrightNotice_IncludesSourceURL(t *testing.T) {
	svc, _, _ := setupAssemblerService(t)
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "output"), 0o755))

	err := svc.GenerateCopyrightNotice(dir, "SCP-173", "Moto42")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "output", "description.txt"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "scp-wiki.wikidot.com/SCP-173")
}
