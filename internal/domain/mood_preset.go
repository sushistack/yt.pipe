package domain

import (
	"fmt"
	"strings"
	"time"
)

// MoodPreset represents a TTS mood preset with voice parameters.
type MoodPreset struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Speed       float64        `json:"speed"`
	Emotion     string         `json:"emotion"`
	Pitch       float64        `json:"pitch"`
	ParamsJSON  map[string]any `json:"params_json,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ValidateMoodPreset validates required fields for a mood preset.
func ValidateMoodPreset(name, emotion string, speed, pitch float64) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name must not be empty")
	}
	if strings.TrimSpace(emotion) == "" {
		return fmt.Errorf("emotion must not be empty")
	}
	if speed <= 0 {
		return fmt.Errorf("speed must be positive, got %f", speed)
	}
	if pitch <= 0 {
		return fmt.Errorf("pitch must be positive, got %f", pitch)
	}
	return nil
}

// SceneMoodAssignment links a scene to a mood preset.
type SceneMoodAssignment struct {
	ProjectID  string `json:"project_id"`
	SceneNum   int    `json:"scene_num"`
	PresetID   string `json:"preset_id"`
	AutoMapped bool   `json:"auto_mapped"`
	Confirmed  bool   `json:"confirmed"`
}
