package domain

import "time"

// Project represents an SCP YouTube content project
type Project struct {
	ID            string
	SCPID         string
	Status        string
	SceneCount    int
	WorkspacePath string
	ReviewToken   string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Stage constants — dependency-based model replaces old state machine.
// Status is a progress marker ("highest stage reached"), not a gate.
const (
	StagePending  = "pending"
	StageScenario = "scenario"
	StageImages   = "images"
	StageTTS      = "tts"
	StageComplete = "complete"
)

// ValidStages maps valid stage strings for validation.
var ValidStages = map[string]bool{
	StagePending:  true,
	StageScenario: true,
	StageImages:   true,
	StageTTS:      true,
	StageComplete: true,
}

// StageOrder defines the rendering order for progress bar display.
// images and tts are parallel — StageIndex should NOT be used for dependency logic.
var StageOrder = []string{StagePending, StageScenario, StageImages, StageTTS, StageComplete}

// IsValidStage checks if a stage string is valid.
func IsValidStage(stage string) bool {
	return ValidStages[stage]
}

// StageIndex returns the position of a stage in StageOrder for progress bar rendering.
// Returns -1 for unknown stages.
func StageIndex(stage string) int {
	for i, s := range StageOrder {
		if s == stage {
			return i
		}
	}
	return -1
}

// SetStage sets the project's status to the given stage and updates the timestamp.
func (p *Project) SetStage(stage string) {
	p.Status = stage
	p.UpdatedAt = time.Now().UTC()
}
