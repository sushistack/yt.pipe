package service

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
)

// SceneDashboardEntry represents a single scene in the dashboard view.
type SceneDashboardEntry struct {
	SceneNum      int    `json:"scene_num"`
	TextExcerpt   string `json:"text_excerpt"`
	ImageStatus   string `json:"image_status"`
	ImagePath     string `json:"image_path,omitempty"`
	ImageAttempts int    `json:"image_attempts"`
	ImageApproved bool   `json:"image_approved"`
	TTSStatus     string `json:"tts_status"`
	TTSPath       string `json:"tts_path,omitempty"`
	TTSAttempts   int    `json:"tts_attempts"`
	TTSApproved   bool   `json:"tts_approved"`
	Prompt        string `json:"prompt"`
	ImagePrompt   string `json:"image_prompt,omitempty"`
	Assets        *SceneAssets `json:"assets"`
	MoodPreset    string `json:"mood_preset,omitempty"`
	BGMName       string `json:"bgm_name,omitempty"`
}

// SceneAssets holds file paths for a scene's generated assets.
type SceneAssets struct {
	ImagePath    string `json:"image_path,omitempty"`
	AudioPath    string `json:"audio_path,omitempty"`
	SubtitlePath string `json:"subtitle_path,omitempty"`
}

// SceneDashboard contains the full dashboard with summary and n8n polling aggregate flags.
type SceneDashboard struct {
	ProjectID          string                 `json:"project_id"`
	ProjectStatus      string                 `json:"project_status"`
	TotalScenes        int                    `json:"total_scenes"`
	ApprovedImageCount int                    `json:"approved_image_count"`
	ApprovedTTSCount   int                    `json:"approved_tts_count"`
	AllImagesApproved  bool                   `json:"all_images_approved"`
	AllTTSApproved     bool                   `json:"all_tts_approved"`
	AllApproved        bool                   `json:"all_approved"`
	Scenes             []*SceneDashboardEntry `json:"scenes"`
	ImageSummary       *ApprovalStatus        `json:"image_summary,omitempty"`
	TTSSummary         *ApprovalStatus        `json:"tts_summary,omitempty"`
}

// SceneDetail provides full detail for a single scene.
type SceneDetail struct {
	SceneDashboardEntry
	FullText       string `json:"full_text"`
	VisualDesc     string `json:"visual_desc,omitempty"`
	SubtitlePath   string `json:"subtitle_path,omitempty"`
	AudioDuration  float64 `json:"audio_duration,omitempty"`
}

// SceneDashboardService provides scene-level dashboard views.
type SceneDashboardService struct {
	store  *store.Store
	logger *slog.Logger
}

// NewSceneDashboardService creates a new SceneDashboardService.
func NewSceneDashboardService(s *store.Store, logger *slog.Logger) *SceneDashboardService {
	return &SceneDashboardService{store: s, logger: logger}
}

// GetDashboard returns the scene dashboard for a project.
func (svc *SceneDashboardService) GetDashboard(projectID string) (*SceneDashboard, error) {
	project, err := svc.store.GetProject(projectID)
	if err != nil {
		return nil, err
	}

	approvalSvc := NewApprovalService(svc.store, svc.logger)

	// Load scenario from DB manifests
	manifests, err := svc.store.ListManifestsByProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("dashboard: list manifests: %w", err)
	}

	// Load image approvals
	imageApprovals, _ := svc.store.ListApprovalsByProject(projectID, domain.AssetTypeImage)
	imageMap := make(map[int]*domain.SceneApproval)
	for _, a := range imageApprovals {
		imageMap[a.SceneNum] = a
	}

	// Load TTS approvals
	ttsApprovals, _ := svc.store.ListApprovalsByProject(projectID, domain.AssetTypeTTS)
	ttsMap := make(map[int]*domain.SceneApproval)
	for _, a := range ttsApprovals {
		ttsMap[a.SceneNum] = a
	}

	// Optionally load scenario for prompt/narration text
	scenarioPath := filepath.Join(project.WorkspacePath, "scenario.json")
	scenario, _ := LoadScenarioFromFile(scenarioPath)
	sceneTextMap := make(map[int]string)
	if scenario != nil {
		for _, sc := range scenario.Scenes {
			sceneTextMap[sc.SceneNum] = sc.Narration
		}
	}

	// Build entries
	sceneCount := project.SceneCount
	if len(manifests) > sceneCount {
		sceneCount = len(manifests)
	}

	approvedImageCount := 0
	approvedTTSCount := 0

	scenes := make([]*SceneDashboardEntry, 0, sceneCount)
	for i := 1; i <= sceneCount; i++ {
		entry := &SceneDashboardEntry{SceneNum: i}

		// Build asset paths
		sceneDir := filepath.Join(project.WorkspacePath, "scenes", fmt.Sprintf("%d", i))
		imgPath := filepath.Join(sceneDir, "image.png")
		audioPath := filepath.Join(sceneDir, "audio.wav")
		subtitlePath := filepath.Join(sceneDir, "subtitle.srt")

		// Set asset paths from manifest (if available)
		for _, m := range manifests {
			if m.SceneNum == i {
				entry.ImagePath = imgPath
				entry.TTSPath = audioPath
				break
			}
		}

		entry.Assets = &SceneAssets{
			ImagePath:    imgPath,
			AudioPath:    audioPath,
			SubtitlePath: subtitlePath,
		}

		// Load image prompt from prompt.txt if it exists
		promptPath := filepath.Join(sceneDir, "prompt.txt")
		if data, err := os.ReadFile(promptPath); err == nil {
			entry.ImagePrompt = string(data)
		}

		// Scene prompt/narration text
		if text, ok := sceneTextMap[i]; ok {
			entry.Prompt = text
			// Excerpt: first 100 chars
			if len(text) > 100 {
				entry.TextExcerpt = text[:100] + "..."
			} else {
				entry.TextExcerpt = text
			}
		}

		// Image approval status
		if ia, ok := imageMap[i]; ok {
			entry.ImageStatus = ia.Status
			entry.ImageAttempts = ia.Attempts
			entry.ImageApproved = ia.Status == domain.ApprovalApproved
		} else {
			entry.ImageStatus = "none"
		}

		// TTS approval status
		if ta, ok := ttsMap[i]; ok {
			entry.TTSStatus = ta.Status
			entry.TTSAttempts = ta.Attempts
			entry.TTSApproved = ta.Status == domain.ApprovalApproved
		} else {
			entry.TTSStatus = "none"
		}

		if entry.ImageApproved {
			approvedImageCount++
		}
		if entry.TTSApproved {
			approvedTTSCount++
		}

		scenes = append(scenes, entry)
	}

	allImagesApproved := sceneCount > 0 && approvedImageCount == sceneCount
	allTTSApproved := sceneCount > 0 && approvedTTSCount == sceneCount

	dashboard := &SceneDashboard{
		ProjectID:          projectID,
		ProjectStatus:      project.Status,
		TotalScenes:        sceneCount,
		ApprovedImageCount: approvedImageCount,
		ApprovedTTSCount:   approvedTTSCount,
		AllImagesApproved:  allImagesApproved,
		AllTTSApproved:     allTTSApproved,
		AllApproved:        allImagesApproved && allTTSApproved,
		Scenes:             scenes,
	}

	// Always compute summaries for n8n polling (not just in review states)
	dashboard.ImageSummary, _ = approvalSvc.GetApprovalStatus(projectID, domain.AssetTypeImage)
	dashboard.TTSSummary, _ = approvalSvc.GetApprovalStatus(projectID, domain.AssetTypeTTS)

	return dashboard, nil
}
