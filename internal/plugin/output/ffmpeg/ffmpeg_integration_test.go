//go:build ffmpegtest

package ffmpeg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckFFmpegAvailable_WithFFmpeg(t *testing.T) {
	path, err := checkFFmpegAvailable()
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}
