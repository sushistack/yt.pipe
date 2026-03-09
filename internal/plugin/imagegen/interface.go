// Package imagegen defines the interface for image generation plugins.
package imagegen

import "context"

//go:generate go run github.com/vektra/mockery/v2@latest --name=ImageGen --output=../../../internal/mocks --outpkg=mocks

// ImageResult holds the output of image generation.
type ImageResult struct {
	ImageData []byte
	Format    string // "png", "jpg", "webp"
	Width     int
	Height    int
}

// CharacterRef holds a character visual reference for consistent image generation.
type CharacterRef struct {
	Name             string // canonical character name (e.g. "SCP-173")
	VisualDescriptor string // visual appearance description
	ImagePromptBase  string // base prompt fragment for this character
}

// GenerateOptions holds optional parameters for image generation.
type GenerateOptions struct {
	Width         int
	Height        int
	Model         string
	Style         string
	Seed          int64
	CharacterRefs []CharacterRef // nil or empty means no character references
}

// ImageGen defines the interface for image generation plugins.
type ImageGen interface {
	// Generate creates a single image from a prompt.
	Generate(ctx context.Context, prompt string, opts GenerateOptions) (*ImageResult, error)
}
