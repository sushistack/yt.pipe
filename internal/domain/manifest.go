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

// ShotManifest tracks the generation state of a single shot for incremental builds.
type ShotManifest struct {
	ProjectID   string
	SceneNum    int
	ShotNum     int
	ContentHash string // SHA-256 of sentence text (input)
	ImageHash   string // SHA-256 of generated image file
	GenMethod   string // "image_edit", "text_to_image", "fallback_t2i"
	Status      string // "pending", "generated", "failed"
	UpdatedAt   time.Time
}
