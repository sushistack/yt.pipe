// Package tts defines the interface for text-to-speech plugins.
package tts

import (
	"context"

	"github.com/sushistack/yt.pipe/internal/domain"
)

//go:generate go run github.com/vektra/mockery/v2@latest --name=TTS --output=../../../internal/mocks --outpkg=mocks

// SynthesisResult holds the output of TTS synthesis.
type SynthesisResult struct {
	AudioData   []byte
	WordTimings []domain.WordTiming
	DurationSec float64
}

// TTS defines the interface for text-to-speech plugins.
type TTS interface {
	// Synthesize converts text to speech audio with word-level timing.
	Synthesize(ctx context.Context, text string, voice string) (*SynthesisResult, error)

	// SynthesizeWithOverrides applies pronunciation overrides from the glossary.
	SynthesizeWithOverrides(ctx context.Context, text string, voice string, overrides map[string]string) (*SynthesisResult, error)
}
