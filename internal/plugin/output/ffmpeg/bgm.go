package ffmpeg

import (
	"fmt"
	"math"
	"strings"

	"github.com/sushistack/yt.pipe/internal/plugin/output"
)

const (
	defaultFadeMs   = 2000
	defaultDuckDB   = -12.0
)

// bgmFilterResult holds the generated filter and the list of BGM input files.
type bgmFilterResult struct {
	filterComplex string   // FFmpeg -filter_complex value
	inputFiles    []string // additional -i arguments for BGM files
}

// generateBGMFilter creates an FFmpeg complex filter expression for BGM mixing.
// Returns empty result (no filter) when bgmAssignments is nil/empty.
//
// narrationDurations maps scene number to narration audio duration in seconds,
// used for ducking timing. totalDuration is the total output duration in seconds.
func generateBGMFilter(bgmAssignments []output.BGMAssignment, narrationDurations map[int]float64, totalDuration float64) bgmFilterResult {
	if len(bgmAssignments) == 0 {
		return bgmFilterResult{}
	}

	var filters []string
	var inputFiles []string

	// Audio input indices: 0 = concat narration, 1 = first subtitle (unused in audio),
	// BGM inputs start at index 2 in the overall FFmpeg command.
	// The caller is responsible for mapping these indices in the final command.
	// Here we use relative labels [bgm0], [bgm1], etc.

	for i, bgm := range bgmAssignments {
		inputFiles = append(inputFiles, bgm.FilePath)
		inputLabel := fmt.Sprintf("[bgm_in_%d]", i)
		outLabel := fmt.Sprintf("[bgm_%d]", i)

		// Volume adjustment: convert dB to FFmpeg volume filter value
		volDB := bgm.VolumeDB
		volFilter := fmt.Sprintf("volume=%.1fdB", volDB)

		// Fade-in
		fadeIn := bgm.FadeInMs
		if fadeIn <= 0 {
			fadeIn = defaultFadeMs
		}
		fadeInSec := float64(fadeIn) / 1000.0

		// Fade-out
		fadeOut := bgm.FadeOutMs
		if fadeOut <= 0 {
			fadeOut = defaultFadeMs
		}
		fadeOutSec := float64(fadeOut) / 1000.0
		fadeOutStart := totalDuration - fadeOutSec
		if fadeOutStart < 0 {
			fadeOutStart = 0
		}

		// Ducking: lower BGM volume during narration
		duckDB := bgm.DuckingDB
		if duckDB == 0 {
			duckDB = defaultDuckDB
		}
		duckRatio := dbToRatio(duckDB)

		// Build filter chain for this BGM track
		// Step 1: volume + fade
		fadeFilter := fmt.Sprintf("%s%s,afade=t=in:st=0:d=%.2f,afade=t=out:st=%.2f:d=%.2f",
			inputLabel, volFilter, fadeInSec, fadeOutStart, fadeOutSec)

		// Step 2: ducking via sidechaincompress or volume automation
		// Using a simpler approach: lower volume to duck ratio for the entire track,
		// then the narration naturally dominates. For precise per-scene ducking,
		// we use the volume filter with enable expressions.
		if len(narrationDurations) > 0 {
			// Build enable expressions for ducking during each narration segment
			var duckExprs []string
			var offset float64
			for sceneNum := 1; sceneNum <= len(narrationDurations)+100; sceneNum++ {
				dur, ok := narrationDurations[sceneNum]
				if !ok {
					continue
				}
				start := offset
				end := offset + dur
				duckExprs = append(duckExprs, fmt.Sprintf("between(t\\,%.2f\\,%.2f)", start, end))
				offset = end
			}
			if len(duckExprs) > 0 {
				// During narration: apply ducking. Between narration: full volume.
				duckExpr := strings.Join(duckExprs, "+")
				duckVolFilter := fmt.Sprintf(",volume='if(%s,%s,1)':eval=frame",
					duckExpr, formatFloat(duckRatio))
				fadeFilter += duckVolFilter
			}
		}

		filters = append(filters, fmt.Sprintf("%s%s", fadeFilter, outLabel))
	}

	// Mix all BGM tracks together if multiple
	if len(bgmAssignments) == 1 {
		return bgmFilterResult{
			filterComplex: strings.Join(filters, ";"),
			inputFiles:    inputFiles,
		}
	}

	// Multiple BGM: amix them together
	var mixInputs string
	for i := range bgmAssignments {
		mixInputs += fmt.Sprintf("[bgm_%d]", i)
	}
	mixFilter := fmt.Sprintf("%samix=inputs=%d:duration=longest[bgm_mixed]",
		mixInputs, len(bgmAssignments))
	filters = append(filters, mixFilter)

	return bgmFilterResult{
		filterComplex: strings.Join(filters, ";"),
		inputFiles:    inputFiles,
	}
}

// dbToRatio converts a dB value to a linear ratio.
// e.g., -12dB → ~0.251
func dbToRatio(db float64) float64 {
	return math.Pow(10, db/20.0)
}

// formatFloat formats a float without trailing zeros.
func formatFloat(f float64) string {
	s := fmt.Sprintf("%.6f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
