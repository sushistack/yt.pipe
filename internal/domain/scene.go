package domain

// Scene represents a single scene in the content pipeline (pipe-filter pattern)
type Scene struct {
	SceneNum      int
	Narration     string
	VisualDesc    string
	FactTags      []string
	Mood          string
	ImagePrompt   string
	ImagePath     string
	AudioPath     string
	AudioDuration float64
	WordTimings   []WordTiming
	SubtitlePath  string
}

// WordTiming represents a single word's timing within TTS audio output
type WordTiming struct {
	Word     string
	StartSec float64
	EndSec   float64
}
