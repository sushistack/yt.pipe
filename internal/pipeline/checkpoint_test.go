package pipeline

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/jay/youtube-pipeline/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckpointManager_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cm := NewCheckpointManager(slog.Default())

	err := cm.SaveStageCheckpoint(dir, "proj-1", service.StageDataLoad, 0)
	require.NoError(t, err)

	cp := cm.LoadCheckpoint(dir)
	require.NotNil(t, cp)
	assert.Equal(t, "proj-1", cp.ProjectID)
	assert.Equal(t, service.StageDataLoad, cp.LastStage)
	assert.Len(t, cp.Stages, 1)
}

func TestCheckpointManager_MultipleStages(t *testing.T) {
	dir := t.TempDir()
	cm := NewCheckpointManager(slog.Default())

	require.NoError(t, cm.SaveStageCheckpoint(dir, "proj-1", service.StageDataLoad, 0))
	require.NoError(t, cm.SaveStageCheckpoint(dir, "proj-1", service.StageScenarioGenerate, 5))

	cp := cm.LoadCheckpoint(dir)
	require.NotNil(t, cp)
	assert.Len(t, cp.Stages, 2)
	assert.Equal(t, service.StageScenarioGenerate, cp.LastStage)
}

func TestCheckpointManager_LoadNoCheckpoint(t *testing.T) {
	dir := t.TempDir()
	cm := NewCheckpointManager(slog.Default())

	cp := cm.LoadCheckpoint(dir)
	assert.Nil(t, cp)
}

func TestCheckpointManager_GetResumeStage(t *testing.T) {
	cm := NewCheckpointManager(slog.Default())

	tests := []struct {
		name     string
		cp       *service.PipelineCheckpoint
		expected service.PipelineStage
	}{
		{
			name:     "nil checkpoint",
			cp:       nil,
			expected: service.StageDataLoad,
		},
		{
			name:     "empty checkpoint",
			cp:       &service.PipelineCheckpoint{},
			expected: service.StageDataLoad,
		},
		{
			name: "after data_load",
			cp: &service.PipelineCheckpoint{
				LastStage: service.StageDataLoad,
				Stages:    []service.StageCheckpoint{{Stage: service.StageDataLoad}},
			},
			expected: service.StageScenarioGenerate,
		},
		{
			name: "after image_generate",
			cp: &service.PipelineCheckpoint{
				LastStage: service.StageImageGenerate,
				Stages:    []service.StageCheckpoint{{Stage: service.StageImageGenerate}},
			},
			expected: service.StageTTSSynthesize,
		},
		{
			name: "after assemble (last stage)",
			cp: &service.PipelineCheckpoint{
				LastStage: service.StageAssemble,
				Stages:    []service.StageCheckpoint{{Stage: service.StageAssemble}},
			},
			expected: service.StageAssemble,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cm.GetResumeStage(tt.cp)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestCheckpointManager_ShouldSkipStage(t *testing.T) {
	cm := NewCheckpointManager(slog.Default())

	cp := &service.PipelineCheckpoint{
		Stages: []service.StageCheckpoint{
			{Stage: service.StageDataLoad},
			{Stage: service.StageScenarioGenerate},
		},
	}

	assert.True(t, cm.ShouldSkipStage(cp, service.StageDataLoad))
	assert.True(t, cm.ShouldSkipStage(cp, service.StageScenarioGenerate))
	assert.False(t, cm.ShouldSkipStage(cp, service.StageImageGenerate))
	assert.False(t, cm.ShouldSkipStage(nil, service.StageDataLoad))
}

func TestBuildRecoveryCommand(t *testing.T) {
	tests := []struct {
		stage    service.PipelineStage
		sceneNum int
		expected string
	}{
		{service.StageDataLoad, 0, "yt-pipe run SCP-173"},
		{service.StageImageGenerate, 5, "yt-pipe image generate SCP-173 --scene 5"},
		{service.StageImageGenerate, 0, "yt-pipe image generate SCP-173"},
		{service.StageTTSSynthesize, 3, "yt-pipe tts generate SCP-173 --scene 3"},
		{service.StageSubtitleGenerate, 0, "yt-pipe subtitle generate SCP-173"},
		{service.StageAssemble, 0, "yt-pipe assemble SCP-173"},
	}

	for _, tt := range tests {
		t.Run(string(tt.stage), func(t *testing.T) {
			cmd := BuildRecoveryCommand("SCP-173", tt.stage, tt.sceneNum)
			assert.Equal(t, tt.expected, cmd)
		})
	}
}

func TestCheckProjectIntegrity_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	// Create empty scenario.json
	err := os.WriteFile(filepath.Join(dir, "scenario.json"), []byte{}, 0644)
	require.NoError(t, err)

	err = CheckProjectIntegrity(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestCheckProjectIntegrity_ValidFile(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, "scenario.json"), []byte(`{"valid": true}`), 0644)
	require.NoError(t, err)

	err = CheckProjectIntegrity(dir)
	assert.NoError(t, err)
}

func TestCheckProjectIntegrity_NoFile(t *testing.T) {
	dir := t.TempDir()
	// No scenario.json yet — should be OK (early stage)
	err := CheckProjectIntegrity(dir)
	assert.NoError(t, err)
}
