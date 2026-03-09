// Package tts defines the interface for text-to-speech plugins.
package tts

import (
	"context"

	"github.com/sushistack/yt.pipe/internal/domain"
)

//go:generate go run github.com/vektra/mockery/v2@latest --name=TTS --output=../../../internal/mocks --outpkg=mocks

// MoodPreset contains voice parameters for mood-specific narration.
type MoodPreset struct {
	Speed   float64        `json:"speed"`
	Emotion string         `json:"emotion"`
	Pitch   float64        `json:"pitch"`
	Params  map[string]any `json:"params,omitempty"`
}

// TTSOptions configures optional parameters for TTS synthesis.
type TTSOptions struct {
	MoodPreset *MoodPreset
}

// SynthesisResult holds the output of TTS synthesis.
type SynthesisResult struct {
	AudioData   []byte
	WordTimings []domain.WordTiming
	DurationSec float64
}

// TTS defines the interface for text-to-speech plugins.
type TTS interface {
	// Synthesize converts text to speech audio with word-level timing.
	// opts may be nil for default TTS tone (backward compatible).
	Synthesize(ctx context.Context, text string, voice string, opts *TTSOptions) (*SynthesisResult, error)

	// SynthesizeWithOverrides applies pronunciation overrides from the glossary.
	// opts may be nil for default TTS tone (backward compatible).
	SynthesizeWithOverrides(ctx context.Context, text string, voice string, overrides map[string]string, opts *TTSOptions) (*SynthesisResult, error)
}
