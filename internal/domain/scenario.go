package domain

// ScenarioOutput is the inter-module contract for scenario generation output
type ScenarioOutput struct {
	SCPID    string
	Title    string
	Scenes   []SceneScript
	Metadata map[string]any
}

// SceneScript represents a single scene's script from scenario generation
type SceneScript struct {
	SceneNum          int
	Narration         string
	VisualDescription string
	FactTags          []FactTag
	Mood              string
	EntityVisible     bool
	Location          string   `json:"location"`
	CharactersPresent []string `json:"characters_present"`
	ColorPalette      string   `json:"color_palette"`
	Atmosphere        string   `json:"atmosphere"`
}

// FactTag represents a tagged fact reference
type FactTag struct {
	Key     string
	Content string
}
