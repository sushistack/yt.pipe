package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sushistack/yt.pipe/internal/domain"
)

const defaultMaxWordsPerLine = 8

// srtEntry represents a single SRT subtitle entry.
type srtEntry struct {
	index    int
	startSec float64
	endSec   float64
	text     string
}

// generateSRT creates an SRT subtitle file from scene WordTimings.
// Timings are accumulated across scenes to produce absolute timestamps.
func generateSRT(scenes []domain.Scene, outputDir string) (string, error) {
	if len(scenes) == 0 {
		return "", fmt.Errorf("no scenes to render")
	}

	sorted := make([]domain.Scene, len(scenes))
	copy(sorted, scenes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SceneNum < sorted[j].SceneNum
	})

	var entries []srtEntry
	var timeOffset float64

	for _, sc := range sorted {
		sceneEntries := groupWordTimings(sc.WordTimings, timeOffset, defaultMaxWordsPerLine)
		entries = append(entries, sceneEntries...)
		timeOffset += sc.AudioDuration
	}

	var b strings.Builder
	for i, e := range entries {
		e.index = i + 1
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n",
			e.index,
			formatSRTTime(e.startSec),
			formatSRTTime(e.endSec),
			e.text,
		)
	}

	outPath := filepath.Join(outputDir, "subtitles.srt")
	if err := os.WriteFile(outPath, []byte(b.String()), 0644); err != nil {
		return "", fmt.Errorf("write subtitles.srt: %w", err)
	}
	return outPath, nil
}

// groupWordTimings groups word timings into subtitle segments of maxWords each.
// timeOffset is added to all timestamps to produce absolute positioning.
func groupWordTimings(timings []domain.WordTiming, timeOffset float64, maxWords int) []srtEntry {
	if len(timings) == 0 {
		return nil
	}

	var entries []srtEntry
	for i := 0; i < len(timings); i += maxWords {
		end := i + maxWords
		if end > len(timings) {
			end = len(timings)
		}
		chunk := timings[i:end]

		var words []string
		for _, w := range chunk {
			words = append(words, w.Word)
		}

		entries = append(entries, srtEntry{
			startSec: chunk[0].StartSec + timeOffset,
			endSec:   chunk[len(chunk)-1].EndSec + timeOffset,
			text:     strings.Join(words, " "),
		})
	}
	return entries
}

// formatSRTTime converts seconds to SRT time format: HH:MM:SS,mmm
func formatSRTTime(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	totalMs := int(sec * 1000)
	h := totalMs / 3600000
	totalMs %= 3600000
	m := totalMs / 60000
	totalMs %= 60000
	s := totalMs / 1000
	ms := totalMs % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}
