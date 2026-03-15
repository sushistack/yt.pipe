package domain

import (
	"testing"
)

func TestIsValidStage(t *testing.T) {
	tests := []struct {
		name     string
		stage    string
		expected bool
	}{
		{"pending is valid", StagePending, true},
		{"scenario is valid", StageScenario, true},
		{"images is valid", StageImages, true},
		{"tts is valid", StageTTS, true},
		{"complete is valid", StageComplete, true},
		{"empty string is invalid", "", false},
		{"unknown is invalid", "unknown", false},
		{"old status generating_assets is invalid", "generating_assets", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidStage(tt.stage); got != tt.expected {
				t.Errorf("IsValidStage(%q) = %v, want %v", tt.stage, got, tt.expected)
			}
		})
	}
}

func TestStageIndex(t *testing.T) {
	tests := []struct {
		name     string
		stage    string
		expected int
	}{
		{"pending is 0", StagePending, 0},
		{"scenario is 1", StageScenario, 1},
		{"character is 2", StageCharacter, 2},
		{"images is 3", StageImages, 3},
		{"tts is 4", StageTTS, 4},
		{"complete is 5", StageComplete, 5},
		{"unknown returns -1", "unknown", -1},
		{"empty returns -1", "", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StageIndex(tt.stage); got != tt.expected {
				t.Errorf("StageIndex(%q) = %d, want %d", tt.stage, got, tt.expected)
			}
		})
	}
}

func TestProject_SetStage(t *testing.T) {
	p := &Project{Status: StagePending}
	p.SetStage(StageScenario)

	if p.Status != StageScenario {
		t.Errorf("expected status %q, got %q", StageScenario, p.Status)
	}
	if p.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set after SetStage")
	}
}

func TestProject_SetStage_UpdatesTimestamp(t *testing.T) {
	p := &Project{Status: StagePending}
	p.SetStage(StageImages)
	first := p.UpdatedAt

	p.SetStage(StageTTS)
	if p.UpdatedAt.Before(first) {
		t.Error("expected UpdatedAt to advance on subsequent SetStage call")
	}
	if p.Status != StageTTS {
		t.Errorf("expected status %q, got %q", StageTTS, p.Status)
	}
}
