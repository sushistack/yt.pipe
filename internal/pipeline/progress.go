package pipeline

import (
	"fmt"
	"io"
	"time"

	"github.com/jay/youtube-pipeline/internal/service"
)

// ProgressTracker writes real-time progress updates to stderr during pipeline execution.
type ProgressTracker struct {
	w         io.Writer
	startedAt time.Time
}

// NewProgressTracker creates a ProgressTracker that writes to the given writer.
func NewProgressTracker(w io.Writer) *ProgressTracker {
	return &ProgressTracker{w: w, startedAt: time.Now()}
}

// OnProgress handles a progress update by writing a formatted line to stderr.
func (pt *ProgressTracker) OnProgress(p service.PipelineProgress) {
	elapsed := time.Since(pt.startedAt).Seconds()
	if p.ScenesTotal > 0 {
		pct := float64(p.ScenesComplete) / float64(p.ScenesTotal) * 100
		fmt.Fprintf(pt.w, "\r[%s] %s: %d/%d scenes (%.0f%%) — %.0fs elapsed",
			stageIcon(p.Stage), p.Stage, p.ScenesComplete, p.ScenesTotal, pct, elapsed)
	} else {
		fmt.Fprintf(pt.w, "\r[%s] %s — %.0fs elapsed",
			stageIcon(p.Stage), p.Stage, elapsed)
	}
}

// Finish writes the final progress line with a newline.
func (pt *ProgressTracker) Finish(status string) {
	elapsed := time.Since(pt.startedAt).Seconds()
	fmt.Fprintf(pt.w, "\n[✓] Pipeline %s in %.1fs\n", status, elapsed)
}

func stageIcon(stage service.PipelineStage) string {
	switch stage {
	case service.StageDataLoad:
		return "1/8"
	case service.StageScenarioGenerate:
		return "2/8"
	case service.StageScenarioApproval:
		return "3/8"
	case service.StageImageGenerate:
		return "4/8"
	case service.StageTTSSynthesize:
		return "5/8"
	case service.StageTimingResolve:
		return "6/8"
	case service.StageSubtitleGenerate:
		return "7/8"
	case service.StageAssemble:
		return "8/8"
	default:
		return "..."
	}
}
