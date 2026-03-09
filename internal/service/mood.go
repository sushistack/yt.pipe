package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/store"
)

// MoodService manages mood preset lifecycle and LLM-based auto-mapping.
type MoodService struct {
	store  *store.Store
	llm    llm.LLM
	logger *slog.Logger
}

// NewMoodService creates a new MoodService.
func NewMoodService(s *store.Store, l llm.LLM, logger *slog.Logger) *MoodService {
	return &MoodService{store: s, llm: l, logger: logger}
}

// CreatePreset validates and creates a mood preset.
func (ms *MoodService) CreatePreset(name, description string, speed float64, emotion string, pitch float64, params map[string]any) (*domain.MoodPreset, error) {
	if err := domain.ValidateMoodPreset(name, emotion, speed, pitch); err != nil {
		return nil, &domain.ValidationError{Field: "mood_preset", Message: err.Error()}
	}

	p := &domain.MoodPreset{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Speed:       speed,
		Emotion:     emotion,
		Pitch:       pitch,
		ParamsJSON:  params,
	}
	if err := ms.store.CreateMoodPreset(p); err != nil {
		return nil, fmt.Errorf("service: create mood preset: %w", err)
	}
	return p, nil
}

// GetPreset retrieves a mood preset by ID.
func (ms *MoodService) GetPreset(id string) (*domain.MoodPreset, error) {
	return ms.store.GetMoodPreset(id)
}

// ListPresets returns all mood presets.
func (ms *MoodService) ListPresets() ([]*domain.MoodPreset, error) {
	return ms.store.ListMoodPresets()
}

// UpdatePreset updates a mood preset's fields.
func (ms *MoodService) UpdatePreset(id string, name *string, description *string, speed *float64, emotion *string, pitch *float64) (*domain.MoodPreset, error) {
	p, err := ms.store.GetMoodPreset(id)
	if err != nil {
		return nil, err
	}

	if name != nil {
		p.Name = *name
	}
	if description != nil {
		p.Description = *description
	}
	if speed != nil {
		p.Speed = *speed
	}
	if emotion != nil {
		p.Emotion = *emotion
	}
	if pitch != nil {
		p.Pitch = *pitch
	}

	if err := domain.ValidateMoodPreset(p.Name, p.Emotion, p.Speed, p.Pitch); err != nil {
		return nil, &domain.ValidationError{Field: "mood_preset", Message: err.Error()}
	}

	if err := ms.store.UpdateMoodPreset(p); err != nil {
		return nil, fmt.Errorf("service: update mood preset: %w", err)
	}
	return p, nil
}

// DeletePreset removes a mood preset by ID.
func (ms *MoodService) DeletePreset(id string) error {
	return ms.store.DeleteMoodPreset(id)
}

// moodAnalysisResult represents the LLM's mood analysis for a single scene.
type moodAnalysisResult struct {
	SceneNum int    `json:"scene_num"`
	Mood     string `json:"mood"`
}

// AutoMapMoods uses LLM to analyze scene text and map mood presets.
func (ms *MoodService) AutoMapMoods(ctx context.Context, projectID string, scenes []domain.SceneScript) (int, error) {
	if ms.llm == nil {
		return 0, fmt.Errorf("service: auto-map moods: LLM plugin not configured")
	}

	presets, err := ms.store.ListMoodPresets()
	if err != nil {
		return 0, fmt.Errorf("service: auto-map moods: list presets: %w", err)
	}
	if len(presets) == 0 {
		return 0, fmt.Errorf("service: auto-map moods: no mood presets defined")
	}

	presetNames := make([]string, len(presets))
	presetByName := make(map[string]*domain.MoodPreset)
	for i, p := range presets {
		presetNames[i] = p.Name
		presetByName[strings.ToLower(p.Name)] = p
	}

	mapped := 0
	for _, scene := range scenes {
		mood, err := ms.analyzeSceneMood(ctx, scene, presetNames)
		if err != nil {
			ms.logger.Warn("mood auto-map failed for scene",
				"project_id", projectID,
				"scene_num", scene.SceneNum,
				"err", err,
			)
			continue
		}

		preset, ok := presetByName[strings.ToLower(mood)]
		if !ok {
			ms.logger.Warn("mood auto-map: no matching preset",
				"project_id", projectID,
				"scene_num", scene.SceneNum,
				"recommended_mood", mood,
			)
			continue
		}

		if err := ms.store.AssignMoodToScene(projectID, scene.SceneNum, preset.ID, true); err != nil {
			ms.logger.Error("mood auto-map: assign failed",
				"project_id", projectID,
				"scene_num", scene.SceneNum,
				"err", err,
			)
			continue
		}
		mapped++
	}

	ms.logger.Info("mood auto-map complete",
		"project_id", projectID,
		"total_scenes", len(scenes),
		"mapped", mapped,
	)

	return mapped, nil
}

func (ms *MoodService) analyzeSceneMood(ctx context.Context, scene domain.SceneScript, presetNames []string) (string, error) {
	prompt := fmt.Sprintf(
		`Analyze the mood of the following narration text and return ONLY the mood name that best matches from this list: [%s]

Narration text:
%s

Return ONLY the mood name, nothing else.`,
		strings.Join(presetNames, ", "),
		scene.Narration,
	)

	result, err := ms.llm.Complete(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, llm.CompletionOptions{Temperature: 0.3, MaxTokens: 50})
	if err != nil {
		return "", fmt.Errorf("llm mood analysis: %w", err)
	}

	mood := strings.TrimSpace(result.Content)
	return mood, nil
}

// GetPendingConfirmations returns all unconfirmed scene mood assignments for a project.
func (ms *MoodService) GetPendingConfirmations(projectID string) ([]*domain.SceneMoodAssignment, error) {
	assignments, err := ms.store.ListSceneMoodAssignments(projectID)
	if err != nil {
		return nil, err
	}

	var pending []*domain.SceneMoodAssignment
	for _, a := range assignments {
		if !a.Confirmed {
			pending = append(pending, a)
		}
	}
	return pending, nil
}

// ConfirmScene confirms a scene mood assignment.
func (ms *MoodService) ConfirmScene(projectID string, sceneNum int) error {
	return ms.store.ConfirmSceneMood(projectID, sceneNum)
}

// ConfirmAll confirms all pending scene mood assignments for a project.
func (ms *MoodService) ConfirmAll(projectID string) (int, error) {
	pending, err := ms.GetPendingConfirmations(projectID)
	if err != nil {
		return 0, err
	}
	confirmed := 0
	for _, a := range pending {
		if err := ms.store.ConfirmSceneMood(projectID, a.SceneNum); err != nil {
			ms.logger.Error("confirm scene mood failed",
				"project_id", projectID,
				"scene_num", a.SceneNum,
				"err", err,
			)
			continue
		}
		confirmed++
	}
	return confirmed, nil
}

// ReassignScene changes a scene's mood preset.
func (ms *MoodService) ReassignScene(projectID string, sceneNum int, presetID string) error {
	return ms.store.AssignMoodToScene(projectID, sceneNum, presetID, false)
}

// GetSceneAssignment retrieves a scene's mood assignment.
func (ms *MoodService) GetSceneAssignment(projectID string, sceneNum int) (*domain.SceneMoodAssignment, error) {
	return ms.store.GetSceneMoodAssignment(projectID, sceneNum)
}

// parseMoodAnalysisResults parses JSON array of mood analysis results from LLM.
func parseMoodAnalysisResults(content string) ([]moodAnalysisResult, error) {
	var results []moodAnalysisResult
	if err := json.Unmarshal([]byte(content), &results); err != nil {
		return nil, fmt.Errorf("parse mood analysis: %w", err)
	}
	return results, nil
}
