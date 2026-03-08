package output_test

import (
	"testing"

	outputp "github.com/sushistack/yt.pipe/internal/plugin/output"
)

// Compile-time interface check
var _ outputp.Assembler = (outputp.Assembler)(nil)

func TestAssemblerInterfaceCompiles(t *testing.T) {
	t.Log("Assembler interface compiles successfully")
}
