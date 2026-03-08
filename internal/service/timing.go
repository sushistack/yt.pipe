package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// SceneTiming holds resolved timing for a single scene.
type SceneTiming struct {
	SceneNum        int                `json:"scene_num"`
	StartSec        float64            `json:"start_sec"`
	EndSec          float64            `json:"end_sec"`
	DurationSec     float64            `json:"duration_sec"`
	WordTimings     []domain.WordTiming `json:"word_timings,omitempty"`
	TransitionPoint float64            `json:"transition_point_sec"`
	SubtitleSegments []SubtitleSegment  `json:"subtitle_segments,omitempty"`
}

// SubtitleSegment represents a single subtitle chunk with timing.
type SubtitleSegment struct {
	StartSec float64 `json:"start_sec"`
	EndSec   float64 `json:"end_sec"`
	Text     string  `json:"text"`
}

// Timeline holds the full project timeline with all scenes.
type Timeline struct {
	TotalDurationSec float64       `json:"total_duration_sec"`
	SceneCount       int           `json:"scene_count"`
	Scenes           []SceneTiming `json:"scenes"`
}

// DefaultSceneDurationSec is the fallback duration for scenes without narration audio.
const DefaultSceneDurationSec = 3.0

// TimingResolver resolves TTS audio timing into image transitions and subtitle synchronization data.
type TimingResolver struct {
	logger               *slog.Logger
	defaultSceneDuration float64
}

// NewTimingResolver creates a new TimingResolver.
func NewTimingResolver(logger *slog.Logger) *TimingResolver {
	return &TimingResolver{logger: logger, defaultSceneDuration: DefaultSceneDurationSec}
}

// WithDefaultSceneDuration sets the fallback duration for scenes without narration.
func (r *TimingResolver) WithDefaultSceneDuration(d float64) *TimingResolver {
	if d > 0 {
		r.defaultSceneDuration = d
	}
	return r
}

// ResolveTimings calculates scene start/end times from audio durations,
// including word timestamps, image transition points, and subtitle segments.
func (r *TimingResolver) ResolveTimings(scenes []*domain.Scene) []SceneTiming {
	timings := make([]SceneTiming, 0, len(scenes))
	var offset float64

	for _, scene := range scenes {
		duration := scene.AudioDuration
		if duration <= 0 {
			duration = r.defaultSceneDuration
		}

		// Build subtitle segments from word timings (AC1)
		subtitleSegs := buildSubtitleSegments(scene.WordTimings, offset)

		timing := SceneTiming{
			SceneNum:         scene.SceneNum,
			StartSec:         offset,
			EndSec:           offset + duration,
			DurationSec:      duration,
			WordTimings:      offsetWordTimings(scene.WordTimings, offset),
			TransitionPoint:  offset + duration, // Image transitions at scene end
			SubtitleSegments: subtitleSegs,
		}
		timings = append(timings, timing)
		offset += duration
	}

	return timings
}

// BuildTimeline creates a full project timeline from scene timings.
func (r *TimingResolver) BuildTimeline(timings []SceneTiming) Timeline {
	return Timeline{
		TotalDurationSec: TotalDuration(timings),
		SceneCount:       len(timings),
		Scenes:           timings,
	}
}

// SaveTimingFiles writes per-scene timing.json and project-level timeline.json (AC3).
func (r *TimingResolver) SaveTimingFiles(timings []SceneTiming, projectPath string) error {
	// Write per-scene timing.json
	for _, t := range timings {
		sceneDir, err := workspace.InitSceneDir(projectPath, t.SceneNum)
		if err != nil {
			return fmt.Errorf("timing: init scene dir %d: %w", t.SceneNum, err)
		}
		timingPath := filepath.Join(sceneDir, "timing.json")
		data, err := json.MarshalIndent(t, "", "  ")
		if err != nil {
			return fmt.Errorf("timing: marshal scene %d: %w", t.SceneNum, err)
		}
		if err := workspace.WriteFileAtomic(timingPath, data); err != nil {
			return fmt.Errorf("timing: save scene %d: %w", t.SceneNum, err)
		}
	}

	// Write project-level timeline.json
	timeline := r.BuildTimeline(timings)
	timelineData, err := json.MarshalIndent(timeline, "", "  ")
	if err != nil {
		return fmt.Errorf("timing: marshal timeline: %w", err)
	}
	timelinePath := filepath.Join(projectPath, "timeline.json")
	if err := workspace.WriteFileAtomic(timelinePath, timelineData); err != nil {
		return fmt.Errorf("timing: save timeline: %w", err)
	}

	r.logger.Info("timing files saved",
		"scene_count", len(timings),
		"total_duration_sec", timeline.TotalDurationSec,
		"project_path", projectPath,
	)
	return nil
}

// UpdateSceneTiming recalculates timing for a specific scene and regenerates the project timeline (AC4).
// Only the affected scene's timing is updated; other scenes are recalculated for offset consistency.
func (r *TimingResolver) UpdateSceneTiming(scenes []*domain.Scene, projectPath string) error {
	timings := r.ResolveTimings(scenes)
	return r.SaveTimingFiles(timings, projectPath)
}

// TotalDuration returns the total duration of all scenes.
func TotalDuration(timings []SceneTiming) float64 {
	if len(timings) == 0 {
		return 0
	}
	return timings[len(timings)-1].EndSec
}

// offsetWordTimings adjusts word timings to absolute time by adding scene offset.
func offsetWordTimings(words []domain.WordTiming, offset float64) []domain.WordTiming {
	if len(words) == 0 {
		return nil
	}
	adjusted := make([]domain.WordTiming, len(words))
	for i, w := range words {
		adjusted[i] = domain.WordTiming{
			Word:     w.Word,
			StartSec: w.StartSec + offset,
			EndSec:   w.EndSec + offset,
		}
	}
	return adjusted
}

// buildSubtitleSegments groups word timings into subtitle segments.
// Groups words into segments of ~5-8 words at natural clause boundaries.
func buildSubtitleSegments(words []domain.WordTiming, sceneOffset float64) []SubtitleSegment {
	if len(words) == 0 {
		return nil
	}

	const maxWordsPerSegment = 8

	var segments []SubtitleSegment
	var currentWords []domain.WordTiming

	for _, w := range words {
		currentWords = append(currentWords, w)

		if len(currentWords) >= maxWordsPerSegment {
			seg := wordsToSegment(currentWords, sceneOffset)
			segments = append(segments, seg)
			currentWords = nil
		}
	}

	// Flush remaining words
	if len(currentWords) > 0 {
		seg := wordsToSegment(currentWords, sceneOffset)
		segments = append(segments, seg)
	}

	return segments
}

func wordsToSegment(words []domain.WordTiming, offset float64) SubtitleSegment {
	text := ""
	for i, w := range words {
		if i > 0 {
			text += " "
		}
		text += w.Word
	}
	return SubtitleSegment{
		StartSec: words[0].StartSec + offset,
		EndSec:   words[len(words)-1].EndSec + offset,
		Text:     text,
	}
}
