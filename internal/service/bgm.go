package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/plugin/output"
	"github.com/sushistack/yt.pipe/internal/store"
)

// BGMService manages the BGM library and auto-recommendation.
type BGMService struct {
	store *store.Store
	llm   llm.LLM
}

// NewBGMService creates a new BGMService.
func NewBGMService(s *store.Store, l llm.LLM) *BGMService {
	return &BGMService{store: s, llm: l}
}

// CreateBGM validates inputs and creates a new BGM entry.
func (bs *BGMService) CreateBGM(name, filePath string, moodTags []string, durationMs int64, licenseType domain.LicenseType, licenseSource, creditText string) (*domain.BGM, error) {
	if name == "" {
		return nil, &domain.ValidationError{Field: "name", Message: "must not be empty"}
	}
	if filePath == "" {
		return nil, &domain.ValidationError{Field: "file_path", Message: "must not be empty"}
	}
	if _, err := os.Stat(filePath); err != nil {
		return nil, &domain.ValidationError{Field: "file_path", Message: fmt.Sprintf("file not found: %s", filePath)}
	}
	if err := domain.ValidateLicenseType(licenseType); err != nil {
		return nil, &domain.ValidationError{Field: "license_type", Message: err.Error()}
	}

	b := &domain.BGM{
		ID:            uuid.New().String(),
		Name:          name,
		FilePath:      filePath,
		MoodTags:      moodTags,
		DurationMs:    durationMs,
		LicenseType:   licenseType,
		LicenseSource: licenseSource,
		CreditText:    creditText,
	}
	if err := bs.store.CreateBGM(b); err != nil {
		return nil, fmt.Errorf("service: create bgm: %w", err)
	}
	return b, nil
}

// GetBGM retrieves a BGM by ID.
func (bs *BGMService) GetBGM(id string) (*domain.BGM, error) {
	return bs.store.GetBGM(id)
}

// ListBGMs returns all BGMs.
func (bs *BGMService) ListBGMs() ([]*domain.BGM, error) {
	return bs.store.ListBGMs()
}

// UpdateBGM updates a BGM's fields.
func (bs *BGMService) UpdateBGM(id string, name string, moodTags []string, licenseType domain.LicenseType, creditText string) (*domain.BGM, error) {
	b, err := bs.store.GetBGM(id)
	if err != nil {
		return nil, err
	}

	if name != "" {
		b.Name = name
	}
	if moodTags != nil {
		b.MoodTags = moodTags
	}
	if licenseType != "" {
		if err := domain.ValidateLicenseType(licenseType); err != nil {
			return nil, &domain.ValidationError{Field: "license_type", Message: err.Error()}
		}
		b.LicenseType = licenseType
	}
	if creditText != "" {
		b.CreditText = creditText
	}

	if err := bs.store.UpdateBGM(b); err != nil {
		return nil, fmt.Errorf("service: update bgm: %w", err)
	}
	return b, nil
}

// DeleteBGM removes a BGM by ID.
func (bs *BGMService) DeleteBGM(id string) error {
	return bs.store.DeleteBGM(id)
}

// AutoRecommendBGMs uses LLM to analyze scenes and recommend BGMs.
func (bs *BGMService) AutoRecommendBGMs(ctx context.Context, projectID string, scenes []domain.Scene) error {
	if len(scenes) == 0 {
		return nil
	}

	// Build scene summaries for LLM prompt
	sceneSummaries := ""
	for _, s := range scenes {
		sceneSummaries += fmt.Sprintf("Scene %d: %s\n", s.SceneNum, s.Narration)
	}

	// Get available mood tags from all BGMs
	allBGMs, err := bs.store.ListBGMs()
	if err != nil {
		return fmt.Errorf("service: auto-recommend: list bgms: %w", err)
	}
	if len(allBGMs) == 0 {
		return nil // No BGMs available, nothing to recommend
	}

	tagSet := make(map[string]bool)
	for _, b := range allBGMs {
		for _, tag := range b.MoodTags {
			tagSet[tag] = true
		}
	}
	var availableTags []string
	for tag := range tagSet {
		availableTags = append(availableTags, tag)
	}

	prompt := fmt.Sprintf(`Analyze the mood and atmosphere of each scene and recommend suitable background music mood tags.

Available mood tags: %v

Scenes:
%s

Return a JSON array where each element has "scene_num" (int) and "mood_tags" (string array from available tags).
Return ONLY the JSON array, no other text.`, availableTags, sceneSummaries)

	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}

	result, err := bs.llm.Complete(ctx, messages, llm.CompletionOptions{})
	if err != nil {
		return fmt.Errorf("service: auto-recommend: llm complete: %w", err)
	}

	// Parse LLM response
	var recommendations []struct {
		SceneNum int      `json:"scene_num"`
		MoodTags []string `json:"mood_tags"`
	}
	if err := json.Unmarshal([]byte(result.Content), &recommendations); err != nil {
		return fmt.Errorf("service: auto-recommend: parse llm response: %w", err)
	}

	// For each scene recommendation, find matching BGMs and assign
	for _, rec := range recommendations {
		if len(rec.MoodTags) == 0 {
			continue
		}

		matches, err := bs.store.SearchByMoodTags(rec.MoodTags)
		if err != nil {
			return fmt.Errorf("service: auto-recommend: search tags for scene %d: %w", rec.SceneNum, err)
		}
		if len(matches) == 0 {
			continue
		}

		// Assign the top match with default parameters
		assignment := &domain.SceneBGMAssignment{
			ProjectID:       projectID,
			SceneNum:        rec.SceneNum,
			BGMID:           matches[0].ID,
			VolumeDB:        0,
			FadeInMs:        2000,
			FadeOutMs:       2000,
			DuckingDB:       -12,
			AutoRecommended: true,
			Confirmed:       false,
		}
		if err := bs.store.AssignBGMToScene(assignment); err != nil {
			return fmt.Errorf("service: auto-recommend: assign scene %d: %w", rec.SceneNum, err)
		}
	}

	return nil
}

// GetPendingConfirmations returns all unconfirmed scene BGM assignments for a project.
func (bs *BGMService) GetPendingConfirmations(projectID string) ([]*domain.SceneBGMAssignment, error) {
	assignments, err := bs.store.ListSceneBGMAssignments(projectID)
	if err != nil {
		return nil, err
	}
	var pending []*domain.SceneBGMAssignment
	for _, a := range assignments {
		if !a.Confirmed {
			pending = append(pending, a)
		}
	}
	return pending, nil
}

// ConfirmBGM marks a scene BGM assignment as confirmed.
func (bs *BGMService) ConfirmBGM(projectID string, sceneNum int) error {
	return bs.store.ConfirmSceneBGM(projectID, sceneNum)
}

// ReassignBGM replaces the BGM for a scene and marks it confirmed.
func (bs *BGMService) ReassignBGM(projectID string, sceneNum int, newBGMID string) error {
	// Verify new BGM exists
	if _, err := bs.store.GetBGM(newBGMID); err != nil {
		return err
	}

	existing, err := bs.store.GetSceneBGMAssignment(projectID, sceneNum)
	if err != nil {
		return err
	}

	existing.BGMID = newBGMID
	existing.Confirmed = true
	return bs.store.AssignBGMToScene(existing)
}

// AdjustBGMParams updates placement parameters for a scene assignment.
func (bs *BGMService) AdjustBGMParams(projectID string, sceneNum int, volumeDB float64, fadeInMs, fadeOutMs int, duckingDB float64) error {
	existing, err := bs.store.GetSceneBGMAssignment(projectID, sceneNum)
	if err != nil {
		return err
	}

	existing.VolumeDB = volumeDB
	existing.FadeInMs = fadeInMs
	existing.FadeOutMs = fadeOutMs
	existing.DuckingDB = duckingDB
	return bs.store.AssignBGMToScene(existing)
}

// GetCredits returns credit entries for all confirmed BGM assignments in a project.
func (bs *BGMService) GetCredits(projectID string) ([]output.CreditEntry, error) {
	assignments, err := bs.store.ListSceneBGMAssignments(projectID)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var credits []output.CreditEntry
	for _, a := range assignments {
		if !a.Confirmed || seen[a.BGMID] {
			continue
		}
		seen[a.BGMID] = true

		bgm, err := bs.store.GetBGM(a.BGMID)
		if err != nil {
			continue
		}
		if bgm.CreditText != "" {
			credits = append(credits, output.CreditEntry{
				Type: "bgm",
				Text: bgm.CreditText,
			})
		}
	}
	return credits, nil
}
