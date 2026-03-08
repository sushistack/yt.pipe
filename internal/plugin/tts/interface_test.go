package tts_test

import (
	"testing"

	ttsplugin "github.com/jay/youtube-pipeline/internal/plugin/tts"
)

// Compile-time interface check
var _ ttsplugin.TTS = (ttsplugin.TTS)(nil)

func TestTTSInterfaceCompiles(t *testing.T) {
	t.Log("TTS interface compiles successfully")
}
