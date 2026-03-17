package domain

import (
	"strings"
)

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
	Shots         []Shot          `json:"shots,omitempty"`
	VisualMeta    SceneVisualMeta `json:"visual_meta,omitempty"`
}

// WordTiming represents a single word's timing within TTS audio output
type WordTiming struct {
	Word     string
	StartSec float64
	EndSec   float64
}

// Shot represents a single visual shot (cut) within a scene.
// A cut may cover one or more sentences (merge) or a sentence may produce multiple cuts (split).
type Shot struct {
	ShotNum        int     `json:"shot_num"`         // sequential index (deprecated, kept for backward compat)
	SentenceStart  int     `json:"sentence_start"`   // first sentence covered (1-based)
	SentenceEnd    int     `json:"sentence_end"`     // last sentence covered (start == end for split, start < end for merge)
	CutNum         int     `json:"cut_num"`           // cut number within the sentence range
	Role           string  `json:"role"`
	CameraType     string  `json:"camera_type"`
	EntityVisible  bool    `json:"entity_visible"`
	ImagePrompt    string  `json:"image_prompt"`
	NegativePrompt string  `json:"negative_prompt"`
	ImagePath      string  `json:"image_path"`
	VideoPath      string  `json:"video_path"`
	StartSec       float64 `json:"start_sec"`
	EndSec         float64 `json:"end_sec"`
	SentenceText   string  `json:"sentence_text"`
}

// ShotKey uniquely identifies a cut within a project using 3-level addressing.
type ShotKey struct {
	SceneNum      int
	SentenceStart int
	CutNum        int
	ShotNum       int // deprecated: kept for backward compat with old skip maps
}

// SceneVisualMeta holds structured visual metadata for a scene.
type SceneVisualMeta struct {
	Location          string   `json:"location"`
	CharactersPresent []string `json:"characters_present"`
	ColorPalette      string   `json:"color_palette"`
	Atmosphere        string   `json:"atmosphere"`
}

// SplitNarrationSentences splits Korean narration into sentences.
// Splits on sentence-ending punctuation: 다. 요. 까? 죠. etc.
// Preserves quoted text as part of the containing sentence.
func SplitNarrationSentences(narration string) []string {
	text := strings.TrimSpace(narration)
	if text == "" {
		return nil
	}

	var sentences []string
	var current strings.Builder
	runes := []rune(text)
	inQuote := false

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Track quote state
		if r == '"' || r == '\u201C' || r == '\u201D' || r == '\'' {
			inQuote = !inQuote
			current.WriteRune(r)
			continue
		}

		current.WriteRune(r)

		// Don't split inside quotes
		if inQuote {
			continue
		}

		// Check for sentence-ending punctuation
		if r == '.' || r == '?' || r == '!' || r == '。' {
			// Don't split on ellipsis (...) — skip if adjacent to other dots
			if r == '.' {
				if i+1 < len(runes) && runes[i+1] == '.' {
					continue
				}
				if i > 0 && runes[i-1] == '.' {
					continue
				}
			}

			// Look ahead: if next char is whitespace or end of text, split
			if i+1 >= len(runes) {
				// End of text
				s := strings.TrimSpace(current.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				current.Reset()
			} else if runes[i+1] == ' ' || runes[i+1] == '\n' || runes[i+1] == '\t' {
				s := strings.TrimSpace(current.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				current.Reset()
				// Skip the whitespace
				i++
			}
		}
	}

	// Remaining text
	if s := strings.TrimSpace(current.String()); s != "" {
		sentences = append(sentences, s)
	}

	return sentences
}
