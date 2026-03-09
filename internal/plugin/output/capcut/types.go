// Package capcut implements the CapCut project format assembler.
// Based on CapCut format version 360000 (new_version 151.0.0).
// Timing unit: microseconds (1 second = 1_000_000).
package capcut

import (
	"math"

	"github.com/google/uuid"
)

// Format version constants matching video.pipeline templates.
const (
	FormatVersion            = 360000
	FormatNewVersion         = "151.0.0"
	DefaultFPS               = 30.0
	MicrosecondsPerSecond    = 1_000_000
	DefaultSceneDurationSec  = 3.0 // Fallback duration for scenes without narration
)

func newID() string {
	return uuid.New().String()
}

func secsToMicro(seconds float64) int64 {
	return int64(seconds * MicrosecondsPerSecond)
}

// dbToLinear converts decibels to linear volume (0 dB = 1.0, -6 dB ≈ 0.5).
func dbToLinear(db float64) float64 {
	return math.Pow(10, db/20.0)
}

// DraftProject is the top-level CapCut draft_content.json structure.
type DraftProject struct {
	ID           string        `json:"id"`
	Version      int           `json:"version"`
	NewVersion   string        `json:"new_version"`
	Name         string        `json:"name"`
	Duration     int64         `json:"duration"`
	CreateTime   int64         `json:"create_time"`
	UpdateTime   int64         `json:"update_time"`
	FPS          float64       `json:"fps"`
	CanvasConfig *CanvasConfig `json:"canvas_config"`
	Tracks       []Track       `json:"tracks"`
	Materials    *Materials    `json:"materials"`
	Platform     *Platform     `json:"platform,omitempty"`
}

// CanvasConfig defines output dimensions.
type CanvasConfig struct {
	Ratio      string      `json:"ratio"`
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	Background interface{} `json:"background"`
}

// Platform identifies the generating platform.
type Platform struct {
	AppVersion string `json:"app_version"`
	OSName     string `json:"os_name"`
	OSVersion  string `json:"os_version"`
}

// Track represents a timeline track (video, audio, or text).
type Track struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Segments      []Segment `json:"segments"`
	Flag          int       `json:"flag"`
	Attribute     int       `json:"attribute"`
	Name          string    `json:"name"`
	IsDefaultName bool      `json:"is_default_name"`
}

// Segment represents a clip on a track.
type Segment struct {
	ID                string      `json:"id"`
	SourceTimerange   *TimeRange  `json:"source_timerange"`
	TargetTimerange   *TimeRange  `json:"target_timerange"`
	RenderTimerange   *TimeRange  `json:"render_timerange"`
	Speed             float64     `json:"speed"`
	Volume            float64     `json:"volume"`
	Clip              *Clip       `json:"clip"`
	MaterialID        string      `json:"material_id"`
	ExtraMaterialRefs []string    `json:"extra_material_refs"`
	RenderIndex       int         `json:"render_index"`
	Visible           bool        `json:"visible"`
	Source            string      `json:"source"`
}

// TimeRange defines a start+duration in microseconds.
type TimeRange struct {
	Start    int64 `json:"start"`
	Duration int64 `json:"duration"`
}

// Clip holds position, scale, and rotation for a segment.
type Clip struct {
	Scale     *XY  `json:"scale"`
	Rotation  float64 `json:"rotation"`
	Transform *XY  `json:"transform"`
	Flip      *Flip `json:"flip"`
	Alpha     float64 `json:"alpha"`
}

// XY is a 2D coordinate pair.
type XY struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Flip holds horizontal/vertical flip state.
type Flip struct {
	Vertical   bool `json:"vertical"`
	Horizontal bool `json:"horizontal"`
}

// Materials holds all material resources referenced by segments.
type Materials struct {
	Videos   []VideoMaterial  `json:"videos"`
	Audios   []AudioMaterial  `json:"audios"`
	Texts    []TextMaterial   `json:"texts"`
	Canvases []CanvasMaterial `json:"canvases"`
}

// VideoMaterial represents an image or video asset.
type VideoMaterial struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Duration     int64  `json:"duration"`
	Path         string `json:"path"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	MaterialName string `json:"material_name"`
	CategoryName string `json:"category_name"`
}

// AudioMaterial represents a TTS audio asset.
type AudioMaterial struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Duration int64  `json:"duration"`
	Path     string `json:"path"`
}

// TextMaterial represents a subtitle text element.
type TextMaterial struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Content string `json:"content"` // JSON-encoded text+styles
}

// TextContent is the parsed content structure inside TextMaterial.Content.
type TextContent struct {
	Text   string      `json:"text"`
	Styles []TextStyle `json:"styles"`
}

// TextStyle defines formatting for a range of text characters.
type TextStyle struct {
	Fill  *TextFill `json:"fill,omitempty"`
	Size  float64   `json:"size"`
	Bold  bool      `json:"bold"`
	Range [2]int    `json:"range"`
}

// TextFill defines text color.
type TextFill struct {
	Content *FillContent `json:"content"`
}

// FillContent wraps the render type and color.
type FillContent struct {
	RenderType string     `json:"render_type"`
	Solid      *SolidFill `json:"solid"`
}

// SolidFill defines a solid color fill.
type SolidFill struct {
	Color [3]float64 `json:"color"`
}

// CanvasMaterial represents a background canvas.
type CanvasMaterial struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// DraftMeta is the draft_meta_info.json structure.
type DraftMeta struct {
	DraftID       string         `json:"draft_id"`
	DraftName     string         `json:"draft_name"`
	DraftFoldPath string         `json:"draft_fold_path"`
	DraftMaterials []DraftMatGroup `json:"draft_materials"`
	TMDraftCreate  int64          `json:"tm_draft_create"`
	TMDraftModified int64         `json:"tm_draft_modified"`
	TMDuration     int64          `json:"tm_duration"`
}

// DraftMatGroup groups materials by type in the meta file.
type DraftMatGroup struct {
	Type  int              `json:"type"`
	Value []DraftMatEntry  `json:"value"`
}

// DraftMatEntry is a material entry in the meta file.
type DraftMatEntry struct {
	FilePath  string `json:"file_Path"`
	Duration  int64  `json:"duration"`
	Height    int    `json:"height"`
	Width     int    `json:"width"`
	ID        string `json:"id"`
	Metetype  string `json:"metetype"`
	Type      int    `json:"type"`
}
