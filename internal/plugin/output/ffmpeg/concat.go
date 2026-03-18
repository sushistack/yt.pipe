package ffmpeg

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sushistack/yt.pipe/internal/domain"
)

// generateImageConcat creates an FFmpeg concat demuxer file (images.txt) from scenes.
// Each entry specifies an image file path and its display duration.
// If a scene has Shots, each shot becomes a separate entry with shot-level timing.
// Otherwise the scene-level ImagePath and AudioDuration are used.
func generateImageConcat(scenes []domain.Scene, outputDir string) (string, error) {
	if len(scenes) == 0 {
		return "", fmt.Errorf("no scenes to render")
	}

	sorted := make([]domain.Scene, len(scenes))
	copy(sorted, scenes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SceneNum < sorted[j].SceneNum
	})

	var b strings.Builder
	for _, sc := range sorted {
		if len(sc.Shots) > 0 {
			shots := make([]domain.Shot, len(sc.Shots))
			copy(shots, sc.Shots)
			sort.Slice(shots, func(i, j int) bool {
				if shots[i].SentenceStart != shots[j].SentenceStart {
					return shots[i].SentenceStart < shots[j].SentenceStart
				}
				return shots[i].CutNum < shots[j].CutNum
			})
			for _, shot := range shots {
				if shot.ImagePath == "" {
					continue
				}
				dur := shot.EndSec - shot.StartSec
				if dur <= 0 {
					dur = 3.0 // fallback default
				}
				fmt.Fprintf(&b, "file '%s'\nduration %.3f\n", shot.ImagePath, dur)
			}
		} else {
			if sc.ImagePath == "" {
				continue
			}
			dur := sc.AudioDuration
			if dur <= 0 {
				dur = 3.0
			}
			fmt.Fprintf(&b, "file '%s'\nduration %.3f\n", sc.ImagePath, dur)
		}
	}

	outPath := filepath.Join(outputDir, "images.txt")
	if err := os.WriteFile(outPath, []byte(b.String()), 0644); err != nil {
		return "", fmt.Errorf("write images.txt: %w", err)
	}
	return outPath, nil
}

// generateAudioConcat creates an FFmpeg concat demuxer file (audio_concat.txt) from scenes.
// Each entry specifies an audio file path, ordered by scene number.
func generateAudioConcat(scenes []domain.Scene, outputDir string) (string, error) {
	if len(scenes) == 0 {
		return "", fmt.Errorf("no scenes to render")
	}

	sorted := make([]domain.Scene, len(scenes))
	copy(sorted, scenes)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].SceneNum < sorted[j].SceneNum
	})

	var b strings.Builder
	for _, sc := range sorted {
		if sc.AudioPath == "" {
			continue
		}
		fmt.Fprintf(&b, "file '%s'\n", sc.AudioPath)
	}

	outPath := filepath.Join(outputDir, "audio_concat.txt")
	if err := os.WriteFile(outPath, []byte(b.String()), 0644); err != nil {
		return "", fmt.Errorf("write audio_concat.txt: %w", err)
	}
	return outPath, nil
}
