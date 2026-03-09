package service

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentHash(t *testing.T) {
	h1 := ContentHash([]byte("hello"))
	h2 := ContentHash([]byte("hello"))
	h3 := ContentHash([]byte("world"))

	assert.Equal(t, h1, h2, "same input should produce same hash")
	assert.NotEqual(t, h1, h3, "different input should produce different hash")
	assert.Len(t, h1, 64, "SHA-256 hex should be 64 chars")
}

func TestContentHash_Empty(t *testing.T) {
	h := ContentHash([]byte{})
	assert.NotEmpty(t, h)
	assert.Len(t, h, 64)
}

func TestPipelineCheckpoint_HasCompletedStage(t *testing.T) {
	cp := &PipelineCheckpoint{
		ProjectID: "test-project",
		Stages: []StageCheckpoint{
			{Stage: StageDataLoad, ScenesDone: 5},
			{Stage: StageScenarioGenerate, ScenesDone: 5},
		},
		LastStage: StageScenarioGenerate,
	}

	assert.True(t, cp.HasCompletedStage(StageDataLoad))
	assert.True(t, cp.HasCompletedStage(StageScenarioGenerate))
	assert.False(t, cp.HasCompletedStage(StageImageGenerate))
	assert.False(t, cp.HasCompletedStage(StageAssemble))
}

func TestPipelineCheckpoint_HasCompletedStage_Empty(t *testing.T) {
	cp := &PipelineCheckpoint{ProjectID: "empty"}
	assert.False(t, cp.HasCompletedStage(StageDataLoad))
}

func TestPipelineCheckpoint_RecordStage(t *testing.T) {
	cp := &PipelineCheckpoint{ProjectID: "test-project"}

	cp.RecordStage(StageDataLoad, 5)
	assert.Len(t, cp.Stages, 1)
	assert.Equal(t, StageDataLoad, cp.LastStage)
	assert.Equal(t, 5, cp.Stages[0].ScenesDone)
	assert.False(t, cp.Stages[0].CompletedAt.IsZero())

	cp.RecordStage(StageScenarioGenerate, 5)
	assert.Len(t, cp.Stages, 2)
	assert.Equal(t, StageScenarioGenerate, cp.LastStage)
}

func TestSaveAndLoadCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()

	cp := &PipelineCheckpoint{
		ProjectID: "scp-173",
		LastStage: StageImageGenerate,
	}
	cp.RecordStage(StageDataLoad, 5)
	cp.RecordStage(StageScenarioGenerate, 5)
	cp.RecordStage(StageImageGenerate, 5)

	err := SaveCheckpoint(tmpDir, cp)
	require.NoError(t, err)

	loaded, err := LoadCheckpoint(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "scp-173", loaded.ProjectID)
	assert.Equal(t, StageImageGenerate, loaded.LastStage)
	assert.Len(t, loaded.Stages, 3)
	assert.False(t, loaded.UpdatedAt.IsZero())
}

func TestLoadCheckpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := LoadCheckpoint(tmpDir)
	assert.Error(t, err)
}

func TestLoadCheckpoint_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "checkpoint.json"), []byte("not json"), 0o644)
	require.NoError(t, err)

	_, err = LoadCheckpoint(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestSaveAndLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()

	m := &SceneManifest{
		ProjectID: "scp-173",
		Entries: []SceneManifestEntry{
			{SceneNum: 1, NarrationHash: "abc", PromptHash: "def", Status: "current"},
			{SceneNum: 2, NarrationHash: "ghi", PromptHash: "jkl", Status: "current"},
		},
	}

	err := SaveManifest(tmpDir, m)
	require.NoError(t, err)

	loaded, err := LoadManifest(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "scp-173", loaded.ProjectID)
	assert.Len(t, loaded.Entries, 2)
	assert.False(t, loaded.UpdatedAt.IsZero())
}

func TestLoadManifest_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := LoadManifest(tmpDir)
	assert.Error(t, err)
}

func TestLoadManifest_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "manifest.json"), []byte("{invalid"), 0o644)
	require.NoError(t, err)

	_, err = LoadManifest(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestSceneManifest_NeedsRegeneration(t *testing.T) {
	m := &SceneManifest{
		Entries: []SceneManifestEntry{
			{
				SceneNum:      1,
				NarrationHash: "narr-hash",
				PromptHash:    "prompt-hash",
				ImageHash:     "image-hash",
				AudioHash:     "audio-hash",
				SubtitleHash:  "sub-hash",
				Status:        "current",
			},
		},
	}

	tests := []struct {
		name      string
		sceneNum  int
		assetType string
		hash      string
		expected  bool
	}{
		{"same narration hash", 1, "narration", "narr-hash", false},
		{"different narration hash", 1, "narration", "new-hash", true},
		{"same prompt hash", 1, "prompt", "prompt-hash", false},
		{"different prompt hash", 1, "prompt", "new-hash", true},
		{"same image hash", 1, "image", "image-hash", false},
		{"different image hash", 1, "image", "new-hash", true},
		{"same audio hash", 1, "audio", "audio-hash", false},
		{"different audio hash", 1, "audio", "new-hash", true},
		{"same subtitle hash", 1, "subtitle", "sub-hash", false},
		{"different subtitle hash", 1, "subtitle", "new-hash", true},
		{"scene not found", 99, "narration", "any", true},
		{"unknown asset type", 1, "unknown", "any", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.NeedsRegeneration(tt.sceneNum, tt.assetType, tt.hash)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSceneManifest_InvalidateDownstream_Narration(t *testing.T) {
	m := &SceneManifest{
		Entries: []SceneManifestEntry{
			{
				SceneNum:      1,
				NarrationHash: "narr",
				PromptHash:    "prompt",
				ImageHash:     "image",
				AudioHash:     "audio",
				SubtitleHash:  "sub",
				Status:        "current",
			},
		},
	}

	m.InvalidateDownstream(1, "narration")

	e := m.Entries[0]
	assert.Equal(t, "narr", e.NarrationHash, "narration hash should be unchanged")
	assert.Empty(t, e.PromptHash, "prompt should be invalidated")
	assert.Empty(t, e.ImageHash, "image should be invalidated")
	assert.Empty(t, e.AudioHash, "audio should be invalidated")
	assert.Empty(t, e.SubtitleHash, "subtitle should be invalidated")
	assert.Equal(t, "stale", e.Status)
}

func TestSceneManifest_InvalidateDownstream_Prompt(t *testing.T) {
	m := &SceneManifest{
		Entries: []SceneManifestEntry{
			{
				SceneNum:      1,
				NarrationHash: "narr",
				PromptHash:    "prompt",
				ImageHash:     "image",
				AudioHash:     "audio",
				SubtitleHash:  "sub",
				Status:        "current",
			},
		},
	}

	m.InvalidateDownstream(1, "prompt")

	e := m.Entries[0]
	assert.Equal(t, "narr", e.NarrationHash, "narration unchanged")
	assert.Equal(t, "prompt", e.PromptHash, "prompt unchanged")
	assert.Empty(t, e.ImageHash, "image should be invalidated")
	assert.Equal(t, "audio", e.AudioHash, "audio unchanged")
	assert.Equal(t, "sub", e.SubtitleHash, "subtitle unchanged")
	assert.Equal(t, "stale", e.Status)
}

func TestSceneManifest_InvalidateDownstream_Audio(t *testing.T) {
	m := &SceneManifest{
		Entries: []SceneManifestEntry{
			{
				SceneNum:      1,
				NarrationHash: "narr",
				PromptHash:    "prompt",
				ImageHash:     "image",
				AudioHash:     "audio",
				SubtitleHash:  "sub",
				Status:        "current",
			},
		},
	}

	m.InvalidateDownstream(1, "audio")

	e := m.Entries[0]
	assert.Equal(t, "narr", e.NarrationHash, "narration unchanged")
	assert.Equal(t, "prompt", e.PromptHash, "prompt unchanged")
	assert.Equal(t, "image", e.ImageHash, "image unchanged")
	assert.Equal(t, "audio", e.AudioHash, "audio unchanged")
	assert.Empty(t, e.SubtitleHash, "subtitle should be invalidated")
	assert.Equal(t, "stale", e.Status)
}

func TestSceneManifest_InvalidateDownstream_NonMatchingScene(t *testing.T) {
	m := &SceneManifest{
		Entries: []SceneManifestEntry{
			{SceneNum: 1, NarrationHash: "narr", PromptHash: "prompt", Status: "current"},
		},
	}

	m.InvalidateDownstream(99, "narration")
	assert.Equal(t, "narr", m.Entries[0].NarrationHash, "non-matching scene should be untouched")
	assert.Equal(t, "current", m.Entries[0].Status)
}

func TestPipelineError_Error(t *testing.T) {
	err := &PipelineError{
		Stage:      StageImageGenerate,
		Cause:      "API rate limit exceeded",
		RecoverCmd: "yt-pipe run --resume --stage image_generate",
	}
	assert.Contains(t, err.Error(), "image_generate")
	assert.Contains(t, err.Error(), "API rate limit exceeded")
	assert.Contains(t, err.Error(), "Recover with:")
}

func TestPipelineError_Error_WithSceneNum(t *testing.T) {
	err := &PipelineError{
		Stage:      StageTTSSynthesize,
		SceneNum:   3,
		Cause:      "timeout",
		RecoverCmd: "yt-pipe run --resume",
	}
	assert.Contains(t, err.Error(), "scene 3")
	assert.Contains(t, err.Error(), "tts_synthesize")
}

func TestPipelineError_Unwrap(t *testing.T) {
	inner := errors.New("connection refused")
	err := &PipelineError{
		Stage: StageDataLoad,
		Cause: "network error",
		Err:   inner,
	}
	assert.Equal(t, inner, errors.Unwrap(err))
}

func TestPipelineStageConstants(t *testing.T) {
	stages := []PipelineStage{
		StageDataLoad,
		StageScenarioGenerate,
		StageScenarioApproval,
		StageImageGenerate,
		StageTTSSynthesize,
		StageTimingResolve,
		StageSubtitleGenerate,
		StageAssemble,
	}

	for _, s := range stages {
		assert.NotEmpty(t, string(s))
	}
	assert.Len(t, stages, 8, "should have 8 pipeline stages")
}

func TestCheckpointRoundTrip_JSON(t *testing.T) {
	cp := &PipelineCheckpoint{
		ProjectID: "scp-999",
	}
	cp.RecordStage(StageDataLoad, 3)
	cp.RecordStage(StageScenarioGenerate, 3)

	data, err := json.Marshal(cp)
	require.NoError(t, err)

	var decoded PipelineCheckpoint
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "scp-999", decoded.ProjectID)
	assert.Len(t, decoded.Stages, 2)
	assert.Equal(t, StageScenarioGenerate, decoded.LastStage)
}
