package domain

// StyleConfig holds project-level visual style configuration.
type StyleConfig struct {
	ArtStyle     string `json:"art_style" yaml:"art_style" mapstructure:"art_style"`
	ColorPalette string `json:"color_palette" yaml:"color_palette" mapstructure:"color_palette"`
	Mood         string `json:"mood" yaml:"mood" mapstructure:"mood"`
	StyleSuffix  string `json:"style_suffix" yaml:"style_suffix" mapstructure:"style_suffix"`
}

// DefaultStyleSuffix is the legacy anime style suffix used when StyleConfig.StyleSuffix is empty.
const DefaultStyleSuffix = "anime illustration, dark horror anime style, highly detailed, vibrant colors, cel shading, sharp lines, dramatic lighting, 16:9 aspect ratio"

// MergeSceneStyle merges project-level style with scene-level visual metadata.
// Scene fields override project fields only when non-zero/non-empty.
func MergeSceneStyle(project StyleConfig, scene SceneVisualMeta) StyleConfig {
	merged := project
	if scene.ColorPalette != "" {
		merged.ColorPalette = scene.ColorPalette
	}
	if scene.Atmosphere != "" {
		merged.Mood = scene.Atmosphere
	}
	return merged
}
