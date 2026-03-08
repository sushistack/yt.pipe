package domain

import "time"

// Feedback represents a quality feedback entry for a scene asset.
type Feedback struct {
	ID        int
	ProjectID string
	SceneNum  int
	AssetType string // "image", "audio", "subtitle", "scenario"
	Rating    string // "good", "bad", "neutral"
	Comment   string
	CreatedAt time.Time
}

// ValidAssetTypes defines allowed asset types for feedback.
var ValidAssetTypes = map[string]bool{
	"image":    true,
	"audio":    true,
	"subtitle": true,
	"scenario": true,
}

// ValidRatings defines allowed rating values.
var ValidRatings = map[string]bool{
	"good":    true,
	"bad":     true,
	"neutral": true,
}
