package capcut

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// Validate checks that a previously assembled CapCut project file is structurally valid.
func (a *Assembler) Validate(_ context.Context, outputPath string) error {
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("capcut validate: read file: %w", err)
	}

	var project DraftProject
	if err := json.Unmarshal(data, &project); err != nil {
		return fmt.Errorf("capcut validate: invalid JSON: %w", err)
	}

	var errs []string

	// Version check
	if project.Version != FormatVersion {
		errs = append(errs, fmt.Sprintf("version: expected %d, got %d", FormatVersion, project.Version))
	}

	// Canvas config required
	if project.CanvasConfig == nil {
		errs = append(errs, "canvas_config: missing")
	} else {
		if project.CanvasConfig.Width <= 0 {
			errs = append(errs, "canvas_config.width: must be positive")
		}
		if project.CanvasConfig.Height <= 0 {
			errs = append(errs, "canvas_config.height: must be positive")
		}
	}

	// Tracks required
	if len(project.Tracks) == 0 {
		errs = append(errs, "tracks: no tracks found")
	} else {
		hasVideo, hasAudio, hasText := false, false, false
		for _, t := range project.Tracks {
			switch t.Type {
			case "video":
				hasVideo = true
			case "audio":
				hasAudio = true
			case "text":
				hasText = true
			}
		}
		if !hasVideo {
			errs = append(errs, "tracks: missing video track")
		}
		if !hasAudio {
			errs = append(errs, "tracks: missing audio track")
		}
		if !hasText {
			errs = append(errs, "tracks: missing text track")
		}
	}

	// Materials required
	if project.Materials == nil {
		errs = append(errs, "materials: missing")
	}

	// Duration check
	if project.Duration <= 0 {
		errs = append(errs, "duration: must be positive")
	}

	// Segment timing validation
	for _, track := range project.Tracks {
		for i, seg := range track.Segments {
			if seg.TargetTimerange != nil {
				if seg.TargetTimerange.Start < 0 {
					errs = append(errs, fmt.Sprintf("track %s segment %d: negative start time", track.Type, i))
				}
				if seg.TargetTimerange.Duration <= 0 {
					errs = append(errs, fmt.Sprintf("track %s segment %d: non-positive duration", track.Type, i))
				}
			}
			if seg.MaterialID == "" {
				errs = append(errs, fmt.Sprintf("track %s segment %d: missing material_id", track.Type, i))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("capcut validate: %d errors: %v", len(errs), errs)
	}

	return nil
}
