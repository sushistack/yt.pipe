package domain

// ScenarioOutput is the inter-module contract for scenario generation output
type ScenarioOutput struct {
	SCPID    string
	Title    string
	Scenes   []SceneScript
	Metadata map[string]string
}

// SceneScript represents a single scene's script from scenario generation
type SceneScript struct {
	SceneNum          int
	Narration         string
	VisualDescription string
	FactTags          []FactTag
	Mood              string
	EntityVisible     bool
}

// FactTag represents a tagged fact reference
type FactTag struct {
	Key     string
	Content string
}
