package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatDurationMs(t *testing.T) {
	tests := []struct {
		ms   int64
		want string
	}{
		{0, "0ms"},
		{500, "500ms"},
		{1500, "1.5s"},
		{30000, "30.0s"},
		{90000, "1.5m"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, formatDurationMs(tt.ms))
	}
}
