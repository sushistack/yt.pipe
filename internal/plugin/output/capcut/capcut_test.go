package capcut

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/jay/youtube-pipeline/internal/plugin/output"
)

func TestAssemble_SingleScene(t *testing.T) {
	dir := t.TempDir()
	a := New()

	scenes := []domain.Scene{
		{
			SceneNum:      1,
			ImagePath:     "/tmp/scene1.png",
			AudioPath:     "/tmp/scene1.mp3",
			AudioDuration: 5.0,
			SubtitlePath:  "/tmp/scene1.srt",
			WordTimings: []domain.WordTiming{
				{Word: "Hello", StartSec: 0.0, EndSec: 0.5},
				{Word: "world", StartSec: 0.5, EndSec: 1.0},
			},
		},
	}

	input := output.AssembleInput{
		Project:   domain.Project{SCPID: "scp-001"},
		Scenes:    scenes,
		OutputDir: dir,
		Canvas:    output.DefaultCanvasConfig(),
	}

	result, err := a.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SceneCount != 1 {
		t.Errorf("scene count = %d, want 1", result.SceneCount)
	}
	if result.ImageCount != 1 {
		t.Errorf("image count = %d, want 1", result.ImageCount)
	}
	if result.AudioCount != 1 {
		t.Errorf("audio count = %d, want 1", result.AudioCount)
	}
	if result.SubtitleCount != 2 {
		t.Errorf("subtitle count = %d, want 2", result.SubtitleCount)
	}
	if result.TotalDuration != 5.0 {
		t.Errorf("total duration = %f, want 5.0", result.TotalDuration)
	}

	// Verify draft_content.json was created
	contentPath := filepath.Join(dir, "draft_content.json")
	if _, err := os.Stat(contentPath); err != nil {
		t.Fatalf("draft_content.json not created: %v", err)
	}

	// Verify draft_meta_info.json was created
	metaPath := filepath.Join(dir, "draft_meta_info.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("draft_meta_info.json not created: %v", err)
	}
}

func TestAssemble_MultipleScenes(t *testing.T) {
	dir := t.TempDir()
	a := New()

	scenes := []domain.Scene{
		{
			SceneNum: 1, ImagePath: "/tmp/s1.png", AudioPath: "/tmp/s1.mp3",
			AudioDuration: 3.0, SubtitlePath: "/tmp/s1.srt",
			WordTimings: []domain.WordTiming{{Word: "A", StartSec: 0, EndSec: 0.5}},
		},
		{
			SceneNum: 2, ImagePath: "/tmp/s2.png", AudioPath: "/tmp/s2.mp3",
			AudioDuration: 4.0, SubtitlePath: "/tmp/s2.srt",
			WordTimings: []domain.WordTiming{{Word: "B", StartSec: 0, EndSec: 0.5}},
		},
		{
			SceneNum: 3, ImagePath: "/tmp/s3.png", AudioPath: "/tmp/s3.mp3",
			AudioDuration: 2.0, SubtitlePath: "/tmp/s3.srt",
			WordTimings: []domain.WordTiming{{Word: "C", StartSec: 0, EndSec: 0.3}},
		},
	}

	input := output.AssembleInput{
		Project:   domain.Project{SCPID: "scp-002"},
		Scenes:    scenes,
		OutputDir: dir,
		Canvas:    output.DefaultCanvasConfig(),
	}

	result, err := a.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SceneCount != 3 {
		t.Errorf("scene count = %d, want 3", result.SceneCount)
	}
	if result.TotalDuration != 9.0 {
		t.Errorf("total duration = %f, want 9.0", result.TotalDuration)
	}
	if result.ImageCount != 3 {
		t.Errorf("image count = %d, want 3", result.ImageCount)
	}
	if result.AudioCount != 3 {
		t.Errorf("audio count = %d, want 3", result.AudioCount)
	}
}

func TestAssemble_DraftContentStructure(t *testing.T) {
	dir := t.TempDir()
	a := New()

	scenes := []domain.Scene{
		{
			SceneNum: 1, ImagePath: "/img.png", AudioPath: "/audio.mp3",
			AudioDuration: 2.0, SubtitlePath: "/sub.srt",
			WordTimings: []domain.WordTiming{{Word: "Test", StartSec: 0, EndSec: 0.5}},
		},
	}

	input := output.AssembleInput{
		Project:   domain.Project{SCPID: "scp-test"},
		Scenes:    scenes,
		OutputDir: dir,
		Canvas:    output.CanvasConfig{Width: 1280, Height: 720, FPS: 24.0},
	}

	_, err := a.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "draft_content.json"))
	var project DraftProject
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if project.Version != FormatVersion {
		t.Errorf("version = %d, want %d", project.Version, FormatVersion)
	}
	if project.NewVersion != FormatNewVersion {
		t.Errorf("new_version = %s, want %s", project.NewVersion, FormatNewVersion)
	}
	if project.CanvasConfig.Width != 1280 {
		t.Errorf("canvas width = %d, want 1280", project.CanvasConfig.Width)
	}
	if project.CanvasConfig.Height != 720 {
		t.Errorf("canvas height = %d, want 720", project.CanvasConfig.Height)
	}
	if project.FPS != 24.0 {
		t.Errorf("fps = %f, want 24.0", project.FPS)
	}
	if len(project.Tracks) != 3 {
		t.Fatalf("tracks = %d, want 3", len(project.Tracks))
	}

	// Check track types
	trackTypes := map[string]bool{}
	for _, tr := range project.Tracks {
		trackTypes[tr.Type] = true
	}
	for _, expected := range []string{"video", "audio", "text"} {
		if !trackTypes[expected] {
			t.Errorf("missing track type: %s", expected)
		}
	}

	// Check materials
	if len(project.Materials.Videos) != 1 {
		t.Errorf("video materials = %d, want 1", len(project.Materials.Videos))
	}
	if len(project.Materials.Audios) != 1 {
		t.Errorf("audio materials = %d, want 1", len(project.Materials.Audios))
	}
	if len(project.Materials.Texts) != 1 {
		t.Errorf("text materials = %d, want 1", len(project.Materials.Texts))
	}
}

func TestAssemble_TimingMicroseconds(t *testing.T) {
	dir := t.TempDir()
	a := New()

	scenes := []domain.Scene{
		{
			SceneNum: 1, ImagePath: "/img.png", AudioPath: "/audio.mp3",
			AudioDuration: 3.5, SubtitlePath: "/sub.srt",
			WordTimings: []domain.WordTiming{
				{Word: "First", StartSec: 0.0, EndSec: 1.0},
				{Word: "Second", StartSec: 1.0, EndSec: 2.5},
			},
		},
	}

	input := output.AssembleInput{
		Scenes:    scenes,
		OutputDir: dir,
		Canvas:    output.DefaultCanvasConfig(),
	}

	_, err := a.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "draft_content.json"))
	var project DraftProject
	json.Unmarshal(data, &project)

	// Duration should be 3.5 seconds in microseconds
	expectedDur := int64(3_500_000)
	if project.Duration != expectedDur {
		t.Errorf("duration = %d, want %d", project.Duration, expectedDur)
	}

	// Video segment target_timerange
	videoTrack := project.Tracks[0]
	if videoTrack.Segments[0].TargetTimerange.Start != 0 {
		t.Errorf("video segment start = %d, want 0", videoTrack.Segments[0].TargetTimerange.Start)
	}
	if videoTrack.Segments[0].TargetTimerange.Duration != expectedDur {
		t.Errorf("video segment duration = %d, want %d", videoTrack.Segments[0].TargetTimerange.Duration, expectedDur)
	}
}

func TestAssemble_DefaultCanvas(t *testing.T) {
	dir := t.TempDir()
	a := New()

	scenes := []domain.Scene{
		{
			SceneNum: 1, ImagePath: "/img.png", AudioPath: "/a.mp3",
			AudioDuration: 1.0, SubtitlePath: "/s.srt",
		},
	}

	// Zero-value canvas should get defaults
	input := output.AssembleInput{
		Scenes:    scenes,
		OutputDir: dir,
		Canvas:    output.CanvasConfig{},
	}

	_, err := a.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "draft_content.json"))
	var project DraftProject
	json.Unmarshal(data, &project)

	if project.CanvasConfig.Width != 1920 {
		t.Errorf("default canvas width = %d, want 1920", project.CanvasConfig.Width)
	}
	if project.CanvasConfig.Height != 1080 {
		t.Errorf("default canvas height = %d, want 1080", project.CanvasConfig.Height)
	}
}

func TestAssemble_DraftMetaInfo(t *testing.T) {
	dir := t.TempDir()
	a := New()

	scenes := []domain.Scene{
		{
			SceneNum: 1, ImagePath: "/img.png", AudioPath: "/a.mp3",
			AudioDuration: 2.0, SubtitlePath: "/s.srt",
		},
	}

	input := output.AssembleInput{
		Scenes:    scenes,
		OutputDir: dir,
		Canvas:    output.DefaultCanvasConfig(),
	}

	_, err := a.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "draft_meta_info.json"))
	var meta DraftMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("invalid meta JSON: %v", err)
	}

	if meta.DraftFoldPath != dir {
		t.Errorf("draft_fold_path = %s, want %s", meta.DraftFoldPath, dir)
	}
	if meta.TMDuration != 2_000_000 {
		t.Errorf("tm_duration = %d, want 2000000", meta.TMDuration)
	}
	if len(meta.DraftMaterials) < 1 {
		t.Error("expected at least 1 draft material group")
	}
}

func TestValidate_ValidProject(t *testing.T) {
	dir := t.TempDir()
	a := New()

	scenes := []domain.Scene{
		{
			SceneNum: 1, ImagePath: "/img.png", AudioPath: "/a.mp3",
			AudioDuration: 1.0, SubtitlePath: "/s.srt",
			WordTimings: []domain.WordTiming{{Word: "Hi", StartSec: 0, EndSec: 0.5}},
		},
	}

	input := output.AssembleInput{
		Scenes:    scenes,
		OutputDir: dir,
		Canvas:    output.DefaultCanvasConfig(),
	}

	result, err := a.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("assemble error: %v", err)
	}

	if err := a.Validate(context.Background(), result.OutputPath); err != nil {
		t.Errorf("validation should pass: %v", err)
	}
}

func TestValidate_MissingFile(t *testing.T) {
	a := New()
	err := a.Validate(context.Background(), "/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestValidate_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0o644)

	a := New()
	err := a.Validate(context.Background(), path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestValidate_MissingTracks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no_tracks.json")

	project := DraftProject{
		Version:    FormatVersion,
		Duration:   1_000_000,
		CanvasConfig: &CanvasConfig{Width: 1920, Height: 1080},
		Tracks:     []Track{},
		Materials:  &Materials{},
	}
	data, _ := json.Marshal(project)
	os.WriteFile(path, data, 0o644)

	a := New()
	err := a.Validate(context.Background(), path)
	if err == nil {
		t.Error("expected error for missing tracks")
	}
}

func TestValidate_MissingCanvasConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no_canvas.json")

	project := DraftProject{
		Version:  FormatVersion,
		Duration: 1_000_000,
		Tracks: []Track{
			{Type: "video"}, {Type: "audio"}, {Type: "text"},
		},
		Materials: &Materials{},
	}
	data, _ := json.Marshal(project)
	os.WriteFile(path, data, 0o644)

	a := New()
	err := a.Validate(context.Background(), path)
	if err == nil {
		t.Error("expected error for missing canvas_config")
	}
}

func TestFactory(t *testing.T) {
	plugin, err := Factory(nil)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if _, ok := plugin.(output.Assembler); !ok {
		t.Error("factory should return output.Assembler")
	}
}

func TestSecsToMicro(t *testing.T) {
	tests := []struct {
		secs float64
		want int64
	}{
		{1.0, 1_000_000},
		{0.5, 500_000},
		{3.5, 3_500_000},
		{0.0, 0},
	}
	for _, tt := range tests {
		got := secsToMicro(tt.secs)
		if got != tt.want {
			t.Errorf("secsToMicro(%f) = %d, want %d", tt.secs, got, tt.want)
		}
	}
}

func TestAssemble_SkipZeroDurationWords(t *testing.T) {
	dir := t.TempDir()
	a := New()

	scenes := []domain.Scene{
		{
			SceneNum: 1, ImagePath: "/img.png", AudioPath: "/a.mp3",
			AudioDuration: 1.0, SubtitlePath: "/s.srt",
			WordTimings: []domain.WordTiming{
				{Word: "Good", StartSec: 0.0, EndSec: 0.5},
				{Word: "Zero", StartSec: 0.5, EndSec: 0.5}, // zero duration
			},
		},
	}

	input := output.AssembleInput{
		Scenes:    scenes,
		OutputDir: dir,
		Canvas:    output.DefaultCanvasConfig(),
	}

	result, err := a.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Zero-duration word should be skipped
	if result.SubtitleCount != 1 {
		t.Errorf("subtitle count = %d, want 1 (zero-duration word skipped)", result.SubtitleCount)
	}
}
