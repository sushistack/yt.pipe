// Package ffmpeg implements the output.Assembler interface using FFmpeg
// for direct MP4 video rendering from scene assets.
package ffmpeg

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sushistack/yt.pipe/internal/config"
	"github.com/sushistack/yt.pipe/internal/plugin/output"
)

// Assembler implements output.Assembler using FFmpeg to render MP4 video.
type Assembler struct {
	ffmpegPath string
	logger     *slog.Logger
	cfg        config.FFmpegConfig
}

// New creates a new FFmpegAssembler after verifying ffmpeg availability.
func New(logger *slog.Logger, cfg config.FFmpegConfig) (*Assembler, error) {
	path, err := checkFFmpegAvailable()
	if err != nil {
		return nil, err
	}
	cfg = applyDefaults(cfg)
	return &Assembler{
		ffmpegPath: path,
		logger:     logger,
		cfg:        cfg,
	}, nil
}

// Factory creates an FFmpegAssembler from configuration (for plugin registry).
func Factory(cfgMap map[string]interface{}) (interface{}, error) {
	cfg := config.FFmpegConfig{}
	if v, ok := cfgMap["preset"].(string); ok {
		cfg.Preset = v
	}
	if v, ok := cfgMap["crf"].(int); ok {
		cfg.CRF = v
	}
	if v, ok := cfgMap["audio_bitrate"].(string); ok {
		cfg.AudioBitrate = v
	}
	if v, ok := cfgMap["resolution"].(string); ok {
		cfg.Resolution = v
	}
	if v, ok := cfgMap["fps"].(int); ok {
		cfg.FPS = v
	}
	if v, ok := cfgMap["subtitle_font_size"].(int); ok {
		cfg.SubtitleFontSize = v
	}
	return New(slog.Default(), cfg)
}

// Assemble renders scene assets into an MP4 video using FFmpeg.
func (a *Assembler) Assemble(ctx context.Context, input output.AssembleInput) (*output.AssembleResult, error) {
	start := time.Now()
	a.logger.Info("ffmpeg assembly started",
		"project_id", input.Project.ID,
		"scenes", len(input.Scenes),
		"output_dir", input.OutputDir)

	if len(input.Scenes) == 0 {
		return nil, fmt.Errorf("ffmpeg: no scenes to render")
	}

	// Step 1: Generate image concat file
	imgConcatPath, err := generateImageConcat(input.Scenes, input.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: generate image concat: %w", err)
	}

	// Step 2: Generate audio concat file
	audioConcatPath, err := generateAudioConcat(input.Scenes, input.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("ffmpeg: generate audio concat: %w", err)
	}

	// Step 3: Generate SRT subtitles (optional — graceful degradation)
	srtPath, srtErr := generateSRT(input.Scenes, input.OutputDir)
	hasSubtitles := srtErr == nil && srtPath != ""

	// Step 4: Generate BGM filter (optional — graceful degradation)
	var narrationDurations map[int]float64
	var totalDuration float64
	if len(input.BGMAssignments) > 0 {
		narrationDurations = make(map[int]float64)
		for _, sc := range input.Scenes {
			narrationDurations[sc.SceneNum] = sc.AudioDuration
			totalDuration += sc.AudioDuration
		}
	} else {
		for _, sc := range input.Scenes {
			totalDuration += sc.AudioDuration
		}
	}
	bgmResult := generateBGMFilter(input.BGMAssignments, narrationDurations, totalDuration)

	// Step 5: Build FFmpeg command
	outputPath := filepath.Join(input.OutputDir, "output.mp4")
	args := a.buildFFmpegArgs(imgConcatPath, audioConcatPath, srtPath, hasSubtitles, bgmResult, outputPath)

	a.logger.Info("ffmpeg command",
		"args_count", len(args),
		"has_subtitles", hasSubtitles,
		"has_bgm", len(bgmResult.inputFiles) > 0)

	// Step 6: Execute FFmpeg
	cmd := exec.CommandContext(ctx, a.ffmpegPath, args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		a.logger.Error("ffmpeg execution failed",
			"error", err,
			"output", string(cmdOutput))
		return nil, fmt.Errorf("ffmpeg: render failed: %w\n%s", err, string(cmdOutput))
	}

	elapsed := time.Since(start)
	a.logger.Info("ffmpeg assembly complete",
		"output", outputPath,
		"duration_sec", elapsed.Seconds(),
		"scenes", len(input.Scenes))

	// Count assets
	imageCount := 0
	audioCount := 0
	subtitleCount := 0
	for _, sc := range input.Scenes {
		if len(sc.Shots) > 0 {
			for _, sh := range sc.Shots {
				if sh.ImagePath != "" {
					imageCount++
				}
			}
		} else if sc.ImagePath != "" {
			imageCount++
		}
		if sc.AudioPath != "" {
			audioCount++
		}
		if len(sc.WordTimings) > 0 {
			subtitleCount++
		}
	}

	return &output.AssembleResult{
		OutputPath:    outputPath,
		SceneCount:    len(input.Scenes),
		TotalDuration: totalDuration,
		ImageCount:    imageCount,
		AudioCount:    audioCount,
		SubtitleCount: subtitleCount,
	}, nil
}

// Validate checks that the rendered MP4 file exists and is non-empty.
func (a *Assembler) Validate(_ context.Context, outputPath string) error {
	cmd := exec.Command(a.ffmpegPath, "-v", "error", "-i", outputPath, "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg validate: %w\n%s", err, string(out))
	}
	return nil
}

// buildFFmpegArgs constructs the FFmpeg command arguments.
func (a *Assembler) buildFFmpegArgs(
	imgConcat, audioConcat, srtPath string,
	hasSubtitles bool,
	bgm bgmFilterResult,
	outputPath string,
) []string {
	args := []string{
		"-y", // overwrite output
		"-f", "concat", "-safe", "0", "-i", imgConcat, // images
		"-f", "concat", "-safe", "0", "-i", audioConcat, // audio
	}

	// Add BGM input files
	for _, bgmFile := range bgm.inputFiles {
		args = append(args, "-i", bgmFile)
	}

	// Video filter: subtitles overlay
	var vf string
	if hasSubtitles {
		// Escape path for FFmpeg subtitle filter (colons, backslashes)
		escapedSRT := strings.ReplaceAll(srtPath, "\\", "\\\\")
		escapedSRT = strings.ReplaceAll(escapedSRT, ":", "\\:")
		vf = fmt.Sprintf("subtitles=%s:force_style='FontSize=%d'", escapedSRT, a.cfg.SubtitleFontSize)
	}

	// Build filter_complex for BGM mixing
	if bgm.filterComplex != "" {
		filterComplex := bgm.filterComplex
		// The BGM filter ends with [bgm_0] (single) or [bgm_mixed] (multiple)
		bgmOutLabel := "[bgm_0]"
		if len(bgm.inputFiles) > 1 {
			bgmOutLabel = "[bgm_mixed]"
		}
		// Mix narration [1:a] with BGM
		filterComplex += fmt.Sprintf(";[1:a]%s amix=inputs=2:duration=first[audio_out]", bgmOutLabel)
		args = append(args, "-filter_complex", filterComplex)
		args = append(args, "-map", "0:v", "-map", "[audio_out]")
	} else {
		args = append(args, "-map", "0:v", "-map", "1:a")
	}

	if vf != "" {
		args = append(args, "-vf", vf)
	}

	// Video codec settings
	args = append(args,
		"-c:v", "libx264",
		"-preset", a.cfg.Preset,
		"-crf", fmt.Sprintf("%d", a.cfg.CRF),
		"-pix_fmt", "yuv420p",
	)

	// Set resolution and FPS
	args = append(args,
		"-s", a.cfg.Resolution,
		"-r", fmt.Sprintf("%d", a.cfg.FPS),
	)

	// Audio codec settings
	args = append(args,
		"-c:a", "aac",
		"-b:a", a.cfg.AudioBitrate,
	)

	args = append(args, outputPath)
	return args
}

// checkFFmpegAvailable verifies that ffmpeg is installed and accessible in PATH.
// Returns the absolute path to the ffmpeg binary on success.
func checkFFmpegAvailable() (string, error) {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("ffmpeg binary not found in PATH: install ffmpeg or use Docker image with ffmpeg included")
	}
	return path, nil
}

// applyDefaults fills in zero-value config fields with sensible defaults.
func applyDefaults(cfg config.FFmpegConfig) config.FFmpegConfig {
	if cfg.Preset == "" {
		cfg.Preset = "medium"
	}
	if cfg.CRF == 0 {
		cfg.CRF = 23
	}
	if cfg.AudioBitrate == "" {
		cfg.AudioBitrate = "192k"
	}
	if cfg.Resolution == "" {
		cfg.Resolution = "1920x1080"
	}
	if cfg.FPS == 0 {
		cfg.FPS = 30
	}
	if cfg.SubtitleFontSize == 0 {
		cfg.SubtitleFontSize = 24
	}
	return cfg
}
