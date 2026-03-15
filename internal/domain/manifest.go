package domain

import "time"

// SceneManifest tracks incremental build state for a scene
type SceneManifest struct {
	ProjectID        string
	SceneNum         int
	ContentHash      string
	ImageHash        string
	AudioHash        string
	SubtitleHash     string
	Status           string
	GenerationMethod string // "image_edit", "text_to_image", "fallback_t2i"
	UpdatedAt        time.Time
}
