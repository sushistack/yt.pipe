package pipeline

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// stageOrder defines the display order for progress stages.
var stageOrder = []service.PipelineStage{
	service.StageDataLoad,
	service.StageScenarioGenerate,
	service.StageScenarioApproval,
	service.StageImageGenerate,
	service.StageTTSSynthesize,
	service.StageTimingResolve,
	service.StageSubtitleGenerate,
	service.StageAssemble,
}

// stageState tracks the progress of a single pipeline stage.
type stageState struct {
	Stage          service.PipelineStage
	ScenesTotal    int
	ScenesComplete int
	Status         string // "waiting", "running", "done"
	StartedAt      time.Time
	CompletedAt    time.Time
}

// ProgressTracker writes real-time progress updates to stderr during pipeline execution.
// It supports multi-stage parallel display with TTY detection.
type ProgressTracker struct {
	w           io.Writer
	startedAt   time.Time
	isTTY       bool
	mu          sync.Mutex
	stages      map[service.PipelineStage]*stageState
	linesDrawn  int
	projectPath string // optional: write progress.json for status command
}

// NewProgressTracker creates a ProgressTracker that writes to the given writer.
func NewProgressTracker(w io.Writer) *ProgressTracker {
	return &ProgressTracker{
		w:         w,
		startedAt: time.Now(),
		isTTY:     isTerminal(w),
		stages:    make(map[service.PipelineStage]*stageState),
	}
}

// SetProjectPath sets the project workspace path for writing progress.json.
func (pt *ProgressTracker) SetProjectPath(path string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.projectPath = path
}

// OnProgress handles a progress update from the pipeline.
func (pt *ProgressTracker) OnProgress(p service.PipelineProgress) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	// Update stage state
	state, exists := pt.stages[p.Stage]
	if !exists {
		state = &stageState{
			Stage:     p.Stage,
			Status:    "running",
			StartedAt: time.Now(),
		}
		pt.stages[p.Stage] = state
	}

	state.ScenesTotal = p.ScenesTotal
	state.ScenesComplete = p.ScenesComplete
	if p.ScenesTotal > 0 && p.ScenesComplete >= p.ScenesTotal {
		state.Status = "done"
		state.CompletedAt = time.Now()
	} else {
		state.Status = "running"
	}

	// Render
	if pt.isTTY {
		pt.renderMultiLine()
	} else {
		pt.renderSimpleLine(p)
	}

	// Write progress.json for status command
	pt.writeProgressFile()
}

// MarkStageDone marks a stage as completed.
func (pt *ProgressTracker) MarkStageDone(stage service.PipelineStage) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	state, exists := pt.stages[stage]
	if !exists {
		state = &stageState{Stage: stage, StartedAt: time.Now()}
		pt.stages[stage] = state
	}
	state.Status = "done"
	state.CompletedAt = time.Now()
	if state.ScenesTotal > 0 {
		state.ScenesComplete = state.ScenesTotal
	}
}

// Finish writes the final progress line with a newline.
func (pt *ProgressTracker) Finish(status string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	elapsed := time.Since(pt.startedAt).Seconds()
	if pt.isTTY && pt.linesDrawn > 0 {
		fmt.Fprintf(pt.w, "\n")
	}
	fmt.Fprintf(pt.w, "\n[✓] Pipeline %s in %.1fs\n", status, elapsed)
}

// renderMultiLine draws a multi-line progress display with ANSI escape codes.
func (pt *ProgressTracker) renderMultiLine() {
	// Move cursor up to overwrite previous lines
	if pt.linesDrawn > 0 {
		fmt.Fprintf(pt.w, "\033[%dF", pt.linesDrawn)
	}

	lines := 0
	for _, stage := range stageOrder {
		state, exists := pt.stages[stage]
		if !exists {
			continue
		}

		fmt.Fprintf(pt.w, "\033[2K") // Clear line

		label := fmt.Sprintf("%-12s", stageName(stage))
		switch state.Status {
		case "done":
			bar := progressBar(1.0, 20)
			elapsed := state.CompletedAt.Sub(state.StartedAt).Seconds()
			if state.ScenesTotal > 0 {
				fmt.Fprintf(pt.w, "[%s] %s 100%% (%d/%d scenes, %.0fs)\n",
					label, bar, state.ScenesTotal, state.ScenesTotal, elapsed)
			} else {
				fmt.Fprintf(pt.w, "[%s] %s 100%% (%.0fs)\n", label, bar, elapsed)
			}
		case "running":
			elapsed := time.Since(state.StartedAt).Seconds()
			if state.ScenesTotal > 0 {
				pct := float64(state.ScenesComplete) / float64(state.ScenesTotal)
				bar := progressBar(pct, 20)
				fmt.Fprintf(pt.w, "[%s] %s %3.0f%% (%d/%d scenes, %.0fs)\n",
					label, bar, pct*100, state.ScenesComplete, state.ScenesTotal, elapsed)
			} else {
				fmt.Fprintf(pt.w, "[%s] running... (%.0fs)\n", label, elapsed)
			}
		default:
			fmt.Fprintf(pt.w, "[%s] waiting...\n", label)
		}
		lines++
	}

	pt.linesDrawn = lines
}

// renderSimpleLine writes a single progress line for non-TTY output.
func (pt *ProgressTracker) renderSimpleLine(p service.PipelineProgress) {
	elapsed := time.Since(pt.startedAt).Seconds()
	if p.ScenesTotal > 0 {
		pct := float64(p.ScenesComplete) / float64(p.ScenesTotal) * 100
		fmt.Fprintf(pt.w, "[%s] %s: %d/%d scenes (%.0f%%) — %.0fs elapsed\n",
			stageIcon(p.Stage), p.Stage, p.ScenesComplete, p.ScenesTotal, pct, elapsed)
	} else {
		fmt.Fprintf(pt.w, "[%s] %s — %.0fs elapsed\n",
			stageIcon(p.Stage), p.Stage, elapsed)
	}
}

// writeProgressFile writes current progress state to progress.json for the status command.
func (pt *ProgressTracker) writeProgressFile() {
	if pt.projectPath == "" {
		return
	}

	type progressEntry struct {
		Stage          string  `json:"stage"`
		Status         string  `json:"status"`
		ScenesTotal    int     `json:"scenes_total,omitempty"`
		ScenesComplete int     `json:"scenes_complete,omitempty"`
		ElapsedSec     float64 `json:"elapsed_sec"`
	}

	var entries []progressEntry
	for _, stage := range stageOrder {
		state, exists := pt.stages[stage]
		if !exists {
			continue
		}
		elapsed := time.Since(state.StartedAt).Seconds()
		if state.Status == "done" {
			elapsed = state.CompletedAt.Sub(state.StartedAt).Seconds()
		}
		entries = append(entries, progressEntry{
			Stage:          string(state.Stage),
			Status:         state.Status,
			ScenesTotal:    state.ScenesTotal,
			ScenesComplete: state.ScenesComplete,
			ElapsedSec:     elapsed,
		})
	}

	data, err := json.Marshal(entries)
	if err != nil {
		return
	}
	_ = workspace.WriteFileAtomic(pt.projectPath+"/progress.json", data)
}

// progressBar generates a Unicode progress bar of the given width.
func progressBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	empty := width - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

// stageName returns a short display name for a stage.
func stageName(stage service.PipelineStage) string {
	switch stage {
	case service.StageDataLoad:
		return "data"
	case service.StageScenarioGenerate:
		return "scenario"
	case service.StageScenarioApproval:
		return "approval"
	case service.StageImageGenerate:
		return "image"
	case service.StageTTSSynthesize:
		return "tts"
	case service.StageTimingResolve:
		return "timing"
	case service.StageSubtitleGenerate:
		return "subtitle"
	case service.StageAssemble:
		return "assembly"
	default:
		return string(stage)
	}
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

// isTerminal checks if the writer is connected to a terminal.
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		stat, err := f.Stat()
		if err != nil {
			return false
		}
		return (stat.Mode() & os.ModeCharDevice) != 0
	}
	return false
}
