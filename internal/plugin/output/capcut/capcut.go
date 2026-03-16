// Package capcut implements the CapCut project format assembler.
package capcut

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/plugin/output"
	"github.com/sushistack/yt.pipe/internal/workspace"
)

// Assembler implements the output.Assembler interface for CapCut project format.
type Assembler struct{}

// New creates a new CapCut Assembler.
func New() *Assembler {
	return &Assembler{}
}

// Factory creates a CapCut Assembler from configuration (for plugin registry).
func Factory(_ map[string]interface{}) (interface{}, error) {
	return New(), nil
}

// Assemble creates a CapCut project from scene assets.
// It generates draft_content.json and draft_meta_info.json in the output directory.
func (a *Assembler) Assemble(ctx context.Context, input output.AssembleInput) (*output.AssembleResult, error) {
	now := time.Now()

	canvas := input.Canvas
	if canvas.Width == 0 {
		canvas = output.DefaultCanvasConfig()
	}

	project := buildDraftProject(input.Scenes, canvas, now, input.BGMAssignments)

	// Serialize draft_content.json
	contentPath := filepath.Join(input.OutputDir, "draft_content.json")
	contentData, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("capcut: marshal draft_content: %w", err)
	}
	if err := workspace.WriteFileAtomic(contentPath, contentData); err != nil {
		return nil, fmt.Errorf("capcut: write draft_content.json: %w", err)
	}

	// Build and serialize draft_meta_info.json
	meta := buildDraftMeta(input, project, now)
	metaPath := filepath.Join(input.OutputDir, "draft_meta_info.json")
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("capcut: marshal draft_meta_info: %w", err)
	}
	if err := workspace.WriteFileAtomic(metaPath, metaData); err != nil {
		return nil, fmt.Errorf("capcut: write draft_meta_info.json: %w", err)
	}

	result := &output.AssembleResult{
		OutputPath:    contentPath,
		SceneCount:    len(input.Scenes),
		TotalDuration: float64(project.Duration) / MicrosecondsPerSecond,
		ImageCount:    len(project.Materials.Videos),
		AudioCount:    len(project.Materials.Audios),
		SubtitleCount: len(project.Materials.Texts),
	}

	slog.Info("capcut project assembled",
		"output", contentPath,
		"scenes", result.SceneCount,
		"duration_sec", result.TotalDuration)

	return result, nil
}

// buildDraftProject constructs the full CapCut DraftProject from scenes with optional BGM tracks.
func buildDraftProject(scenes []domain.Scene, canvas output.CanvasConfig, now time.Time, bgmAssignments []output.BGMAssignment) *DraftProject {
	projectID := newID()
	nowUnix := now.Unix()

	dp := &DraftProject{
		ID:         projectID,
		Version:    FormatVersion,
		NewVersion: FormatNewVersion,
		Name:       "yt-pipe-project",
		CreateTime: nowUnix,
		UpdateTime: nowUnix,
		FPS:        canvas.FPS,
		CanvasConfig: &CanvasConfig{
			Ratio:      "original",
			Width:      canvas.Width,
			Height:     canvas.Height,
			Background: map[string]interface{}{},
		},
		Materials: &Materials{
			Videos:   make([]VideoMaterial, 0),
			Audios:   make([]AudioMaterial, 0),
			Texts:    make([]TextMaterial, 0),
			Canvases: make([]CanvasMaterial, 0),
		},
		Platform: &Platform{
			AppVersion: FormatNewVersion,
			OSName:     "linux",
			OSVersion:  "1.0",
		},
	}

	videoTrack := Track{
		ID:            newID(),
		Type:          "video",
		Segments:      make([]Segment, 0),
		Flag:          0,
		Attribute:     0,
		Name:          "",
		IsDefaultName: true,
	}

	audioTrack := Track{
		ID:            newID(),
		Type:          "audio",
		Segments:      make([]Segment, 0),
		Flag:          0,
		Attribute:     0,
		Name:          "",
		IsDefaultName: true,
	}

	textTrack := Track{
		ID:            newID(),
		Type:          "text",
		Segments:      make([]Segment, 0),
		Flag:          0,
		Attribute:     0,
		Name:          "",
		IsDefaultName: true,
	}

	var timelinePos int64

	for _, scene := range scenes {
		dur := scene.AudioDuration
		if dur <= 0 {
			dur = DefaultSceneDurationSec
		}
		audioDur := secsToMicro(dur)

		// Video track: one clip per shot (or single image fallback)
		if len(scene.Shots) > 0 {
			// Compute shot timeline positions within this scene
			var shotOffset int64
			for si, shot := range scene.Shots {
				imagePath := shot.ImagePath
				if shot.VideoPath != "" {
					imagePath = shot.VideoPath
				}

				shotDur := secsToMicro(shot.EndSec - shot.StartSec)
				if shotDur <= 0 {
					// Equal distribution fallback when timings not resolved
					shotDur = audioDur / int64(len(scene.Shots))
					// Last shot gets remaining duration to avoid rounding gaps
					if si == len(scene.Shots)-1 {
						shotDur = audioDur - shotOffset
					}
				}
				shotStart := timelinePos + shotOffset

				videoMat := VideoMaterial{
					ID:           newID(),
					Type:         "photo",
					Duration:     shotDur,
					Path:         imagePath,
					Width:        canvas.Width,
					Height:       canvas.Height,
					MaterialName: fmt.Sprintf("scene_%d_shot_%d", scene.SceneNum, shot.ShotNum),
					CategoryName: "local",
				}
				dp.Materials.Videos = append(dp.Materials.Videos, videoMat)

				videoSeg := Segment{
					ID:              newID(),
					SourceTimerange: &TimeRange{Start: 0, Duration: shotDur},
					TargetTimerange: &TimeRange{Start: shotStart, Duration: shotDur},
					Speed:           1.0,
					Volume:          1.0,
					Clip: &Clip{
						Scale:     &XY{X: 1.0, Y: 1.0},
						Rotation:  0,
						Transform: &XY{X: 0, Y: 0},
						Flip:      &Flip{},
						Alpha:     1.0,
					},
					MaterialID:        videoMat.ID,
					ExtraMaterialRefs: []string{},
					RenderIndex:       0,
					Visible:           true,
				}
				videoTrack.Segments = append(videoTrack.Segments, videoSeg)
				shotOffset += shotDur
			}
		} else {
			// Backward compat: no shots = single image clip
			videoMat := VideoMaterial{
				ID:           newID(),
				Type:         "photo",
				Duration:     audioDur,
				Path:         scene.ImagePath,
				Width:        canvas.Width,
				Height:       canvas.Height,
				MaterialName: fmt.Sprintf("scene_%d", scene.SceneNum),
				CategoryName: "local",
			}
			dp.Materials.Videos = append(dp.Materials.Videos, videoMat)

			videoSeg := Segment{
				ID:              newID(),
				SourceTimerange: &TimeRange{Start: 0, Duration: audioDur},
				TargetTimerange: &TimeRange{Start: timelinePos, Duration: audioDur},
				Speed:           1.0,
				Volume:          1.0,
				Clip: &Clip{
					Scale:     &XY{X: 1.0, Y: 1.0},
					Rotation:  0,
					Transform: &XY{X: 0, Y: 0},
					Flip:      &Flip{},
					Alpha:     1.0,
				},
				MaterialID:        videoMat.ID,
				ExtraMaterialRefs: []string{},
				RenderIndex:       0,
				Visible:           true,
			}
			videoTrack.Segments = append(videoTrack.Segments, videoSeg)
		}

		// Audio material + segment (skip for scenes without narration)
		if scene.AudioPath != "" {
			audioMat := AudioMaterial{
				ID:       newID(),
				Type:     "extract_music",
				Name:     fmt.Sprintf("scene_%d_audio", scene.SceneNum),
				Duration: audioDur,
				Path:     scene.AudioPath,
			}
			dp.Materials.Audios = append(dp.Materials.Audios, audioMat)

			audioSeg := Segment{
				ID:              newID(),
				SourceTimerange: &TimeRange{Start: 0, Duration: audioDur},
				TargetTimerange: &TimeRange{Start: timelinePos, Duration: audioDur},
				Speed:           1.0,
				Volume:          1.0,
				MaterialID:      audioMat.ID,
				ExtraMaterialRefs: []string{},
				RenderIndex:     0,
				Visible:         true,
			}
			audioTrack.Segments = append(audioTrack.Segments, audioSeg)
		}

		// Text materials + segments from word timings
		for _, wt := range scene.WordTimings {
			wordStart := secsToMicro(wt.StartSec)
			wordDur := secsToMicro(wt.EndSec - wt.StartSec)
			if wordDur <= 0 {
				continue
			}

			textContent := TextContent{
				Text: wt.Word,
				Styles: []TextStyle{
					{
						Fill: &TextFill{
							Content: &FillContent{
								RenderType: "solid",
								Solid:      &SolidFill{Color: [3]float64{1.0, 1.0, 1.0}},
							},
						},
						Size:  8.0,
						Bold:  true,
						Range: [2]int{0, len(wt.Word)},
					},
				},
			}
			contentJSON, _ := json.Marshal(textContent)

			textMat := TextMaterial{
				ID:      newID(),
				Type:    "text",
				Content: string(contentJSON),
			}
			dp.Materials.Texts = append(dp.Materials.Texts, textMat)

			textSeg := Segment{
				ID:              newID(),
				SourceTimerange: &TimeRange{Start: 0, Duration: wordDur},
				TargetTimerange: &TimeRange{Start: timelinePos + wordStart, Duration: wordDur},
				Speed:           1.0,
				Volume:          0,
				Clip: &Clip{
					Scale:     &XY{X: 1.0, Y: 1.0},
					Rotation:  0,
					Transform: &XY{X: 0, Y: 0.85},
					Flip:      &Flip{},
					Alpha:     1.0,
				},
				MaterialID:        textMat.ID,
				ExtraMaterialRefs: []string{},
				RenderIndex:       0,
				Visible:           true,
			}
			textTrack.Segments = append(textTrack.Segments, textSeg)
		}

		timelinePos += audioDur
	}

	dp.Duration = timelinePos
	tracks := []Track{videoTrack, audioTrack, textTrack}

	// Add BGM track if assignments are provided
	if len(bgmAssignments) > 0 {
		bgmTrack := buildBGMTrack(scenes, bgmAssignments, dp.Materials)
		if len(bgmTrack.Segments) > 0 {
			tracks = append(tracks, bgmTrack)
		}
	}

	dp.Tracks = tracks
	return dp
}

// buildBGMTrack creates a separate audio track for BGM with per-scene volume/fade/ducking.
func buildBGMTrack(scenes []domain.Scene, assignments []output.BGMAssignment, materials *Materials) Track {
	bgmTrack := Track{
		ID:            newID(),
		Type:          "audio",
		Segments:      make([]Segment, 0),
		Flag:          0,
		Attribute:     0,
		Name:          "bgm",
		IsDefaultName: false,
	}

	// Build a map of scene positions and durations
	type sceneTimeline struct {
		start    int64
		duration int64
	}
	sceneMap := make(map[int]sceneTimeline)
	var pos int64
	for _, scene := range scenes {
		dur := scene.AudioDuration
		if dur <= 0 {
			dur = DefaultSceneDurationSec
		}
		microDur := secsToMicro(dur)
		sceneMap[scene.SceneNum] = sceneTimeline{start: pos, duration: microDur}
		pos += microDur
	}

	for _, a := range assignments {
		st, ok := sceneMap[a.SceneNum]
		if !ok {
			continue
		}

		// Create BGM audio material
		bgmMat := AudioMaterial{
			ID:       newID(),
			Type:     "extract_music",
			Name:     fmt.Sprintf("bgm_scene_%d", a.SceneNum),
			Duration: st.duration,
			Path:     a.FilePath,
		}
		materials.Audios = append(materials.Audios, bgmMat)

		// Convert dB volume to linear (0 dB = 1.0, -6 dB ≈ 0.5)
		volume := dbToLinear(a.VolumeDB)

		bgmSeg := Segment{
			ID:              newID(),
			SourceTimerange: &TimeRange{Start: 0, Duration: st.duration},
			TargetTimerange: &TimeRange{Start: st.start, Duration: st.duration},
			Speed:           1.0,
			Volume:          volume,
			MaterialID:      bgmMat.ID,
			ExtraMaterialRefs: []string{},
			RenderIndex:     0,
			Visible:         true,
		}
		bgmTrack.Segments = append(bgmTrack.Segments, bgmSeg)
	}

	return bgmTrack
}

// buildDraftMeta constructs the draft_meta_info.json structure.
func buildDraftMeta(input output.AssembleInput, project *DraftProject, now time.Time) *DraftMeta {
	nowUnix := now.Unix()

	meta := &DraftMeta{
		DraftID:         project.ID,
		DraftName:       project.Name,
		DraftFoldPath:   input.OutputDir,
		TMDraftCreate:   nowUnix,
		TMDraftModified: nowUnix,
		TMDuration:      project.Duration,
	}

	// Add image materials to meta
	var imageEntries []DraftMatEntry
	for _, v := range project.Materials.Videos {
		imageEntries = append(imageEntries, DraftMatEntry{
			FilePath: v.Path,
			Duration: v.Duration,
			Height:   v.Height,
			Width:    v.Width,
			ID:       v.ID,
			Metetype: "photo",
			Type:     0,
		})
	}
	if len(imageEntries) > 0 {
		meta.DraftMaterials = append(meta.DraftMaterials, DraftMatGroup{
			Type:  0, // photo type
			Value: imageEntries,
		})
	}

	// Add audio materials to meta
	var audioEntries []DraftMatEntry
	for _, a := range project.Materials.Audios {
		audioEntries = append(audioEntries, DraftMatEntry{
			FilePath: a.Path,
			Duration: a.Duration,
			ID:       a.ID,
			Metetype: "music",
			Type:     1,
		})
	}
	if len(audioEntries) > 0 {
		meta.DraftMaterials = append(meta.DraftMaterials, DraftMatGroup{
			Type:  1, // audio type
			Value: audioEntries,
		})
	}

	return meta
}
