package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/store"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <scp-id>",
	Short: "Show project status and progress",
	Long:  "Display the current state, progress, and asset status for an SCP project.",
	Args:  cobra.ExactArgs(1),
	RunE:  runStatusCmd,
}

func init() {
	statusCmd.Flags().Bool("scenes", false, "show per-scene asset detail table")
	rootCmd.AddCommand(statusCmd)
}

// ProjectStatus is the JSON output format for yt-pipe status.
type ProjectStatus struct {
	ProjectID   string        `json:"project_id"`
	SCPID       string        `json:"scp_id"`
	Status      string        `json:"status"`
	SceneCount  int           `json:"scene_count"`
	Scenes      []SceneStatus `json:"scenes,omitempty"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
}

// SceneStatus describes per-scene asset status.
type SceneStatus struct {
	SceneNum     int    `json:"scene_num"`
	ImageFile    string `json:"image_file"`
	ImageStatus  string `json:"image_status"`
	AudioFile    string `json:"audio_file"`
	AudioStatus  string `json:"audio_status"`
	SubtitleFile string `json:"subtitle_file"`
	SubStatus    string `json:"subtitle_status"`
	PromptFile   string `json:"prompt_file,omitempty"`
	Timestamp    string `json:"timestamp,omitempty"`
}

func runStatusCmd(cmd *cobra.Command, args []string) error {
	scpID := args[0]
	cmd.SilenceUsage = true

	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("status: configuration not loaded")
	}
	c := cfg.Config

	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = c.WorkspacePath + "/yt-pipe.db"
	}
	db, err := store.New(dbPath)
	if err != nil {
		return fmt.Errorf("status: open database: %w", err)
	}
	defer db.Close()

	// Find the most recent project for this SCP ID
	project, err := findProjectBySCPID(db, scpID)
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}

	showScenes, _ := cmd.Flags().GetBool("scenes")
	scenes := collectSceneStatuses(project.WorkspacePath, project.SceneCount)

	ps := ProjectStatus{
		ProjectID:  project.ID,
		SCPID:      project.SCPID,
		Status:     project.Status,
		SceneCount: project.SceneCount,
		CreatedAt:  project.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  project.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if showScenes {
		ps.Scenes = scenes
	}

	jsonOutput, _ := cmd.Flags().GetBool("json-output")
	if jsonOutput {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(ps)
	}

	return outputStatusHuman(cmd, ps, scenes, showScenes)
}

func outputStatusHuman(cmd *cobra.Command, ps ProjectStatus, scenes []SceneStatus, showScenes bool) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "\n=== Project Status: %s ===\n\n", ps.SCPID)
	fmt.Fprintf(w, "  Project ID:  %s\n", ps.ProjectID)
	fmt.Fprintf(w, "  Status:      %s\n", ps.Status)
	fmt.Fprintf(w, "  Scenes:      %d\n", ps.SceneCount)
	fmt.Fprintf(w, "  Created:     %s\n", ps.CreatedAt)
	fmt.Fprintf(w, "  Updated:     %s\n", ps.UpdatedAt)
	fmt.Fprintln(w)

	if !showScenes || len(scenes) == 0 {
		return nil
	}

	// Scene table
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "Scene\tImage\tAudio\tSubtitle\tTimestamp")
	fmt.Fprintln(tw, "-----\t-----\t-----\t--------\t---------")
	for _, s := range scenes {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			s.SceneNum, s.ImageStatus, s.AudioStatus, s.SubStatus, s.Timestamp)
	}
	tw.Flush()
	fmt.Fprintln(w)

	return nil
}

func findProjectBySCPID(db *store.Store, scpID string) (*domain.Project, error) {
	projects, err := db.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	for _, p := range projects {
		if p.SCPID == scpID {
			return p, nil
		}
	}
	return nil, &domain.NotFoundError{Resource: "project", ID: scpID}
}

func collectSceneStatuses(workspacePath string, sceneCount int) []SceneStatus {
	var scenes []SceneStatus
	scenesDir := filepath.Join(workspacePath, "scenes")

	entries, err := os.ReadDir(scenesDir)
	if err != nil {
		// No scenes directory yet, return numbered empty entries
		for i := 1; i <= sceneCount; i++ {
			scenes = append(scenes, SceneStatus{
				SceneNum:    i,
				ImageStatus: "pending",
				AudioStatus: "pending",
				SubStatus:   "pending",
			})
		}
		return scenes
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		var num int
		if _, err := fmt.Sscanf(entry.Name(), "%d", &num); err != nil {
			continue
		}

		sceneDir := filepath.Join(scenesDir, entry.Name())
		ss := SceneStatus{SceneNum: num}

		// Check image
		ss.ImageFile, ss.ImageStatus = checkAsset(sceneDir, "image.png", "image.jpg", "image.webp")
		// Check audio
		ss.AudioFile, ss.AudioStatus = checkAsset(sceneDir, "audio.mp3", "audio.wav")
		// Check subtitle
		ss.SubtitleFile, ss.SubStatus = checkAsset(sceneDir, "subtitle.json", "subtitle.srt")
		// Check prompt
		ss.PromptFile, _ = checkAsset(sceneDir, "prompt.txt")

		// Get modification time
		if info, err := entry.Info(); err == nil {
			ss.Timestamp = info.ModTime().Format("2006-01-02T15:04:05Z")
		}

		scenes = append(scenes, ss)
	}

	sort.Slice(scenes, func(i, j int) bool {
		return scenes[i].SceneNum < scenes[j].SceneNum
	})

	return scenes
}

func checkAsset(dir string, names ...string) (string, string) {
	for _, name := range names {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil {
			if info.Size() > 0 {
				return path, "generated"
			}
			return path, "empty"
		}
	}
	return "", "pending"
}

func truncatePrompt(prompt string, maxLen int) string {
	if len(prompt) <= maxLen {
		return prompt
	}
	return strings.TrimSpace(prompt[:maxLen]) + "..."
}
