package imagegen_test

import (
	"testing"

	imagegenp "github.com/sushistack/yt.pipe/internal/plugin/imagegen"
)

// Compile-time interface check
var _ imagegenp.ImageGen = (imagegenp.ImageGen)(nil)

func TestImageGenInterfaceCompiles(t *testing.T) {
	t.Log("ImageGen interface compiles successfully")
}
