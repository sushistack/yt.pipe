// Package output defines the interface for output assembly plugins.
package output

import (
	"context"

	"github.com/jay/youtube-pipeline/internal/domain"
)

//go:generate go run github.com/vektra/mockery/v2@latest --name=Assembler --output=../../../internal/mocks --outpkg=mocks

// CanvasConfig holds output canvas dimensions and frame rate.
type CanvasConfig struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	FPS    float64 `json:"fps"`
}

// DefaultCanvasConfig returns the standard 1080p 30fps canvas configuration.
func DefaultCanvasConfig() CanvasConfig {
	return CanvasConfig{Width: 1920, Height: 1080, FPS: 30.0}
}

// AssembleInput contains all assets needed for final project assembly.
type AssembleInput struct {
	Project      domain.Project
	Scenes       []domain.Scene
	OutputDir    string
	TemplatePath string       // Path to CapCut draft template JSON
	MetaPath     string       // Path to CapCut draft meta info JSON
	Canvas       CanvasConfig // Output canvas configuration
}

// AssembleResult contains the output summary after successful assembly.
type AssembleResult struct {
	OutputPath     string  `json:"output_path"`
	SceneCount     int     `json:"scene_count"`
	TotalDuration  float64 `json:"total_duration_sec"`
	ImageCount     int     `json:"image_count"`
	AudioCount     int     `json:"audio_count"`
	SubtitleCount  int     `json:"subtitle_count"`
}

// Assembler defines the interface for output format assembly plugins.
type Assembler interface {
	// Assemble creates the final output project from all scene assets.
	Assemble(ctx context.Context, input AssembleInput) (*AssembleResult, error)

	// Validate checks if a previously assembled output is still valid.
	Validate(ctx context.Context, outputPath string) error
}
