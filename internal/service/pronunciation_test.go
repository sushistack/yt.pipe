package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sushistack/yt.pipe/internal/glossary"
)

func createTestGlossary(t *testing.T, entries []glossary.Entry) *glossary.Glossary {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.json")
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal glossary: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write glossary: %v", err)
	}
	return glossary.LoadFromFile(path)
}

func TestPronunciationService_ApplyGlossary(t *testing.T) {
	g := createTestGlossary(t, []glossary.Entry{
		{Term: "SCP", Pronunciation: "에스씨피"},
		{Term: "Keter", Pronunciation: "케테르"},
		{Term: "Euclid", Pronunciation: "유클리드"},
	})

	svc := &PronunciationService{glossary: g}

	text := "SCP-173은 Euclid 등급이며, Keter 등급은 아닙니다."
	result, applied := svc.applyGlossary(text)

	expected := "에스씨피-173은 유클리드 등급이며, 케테르 등급은 아닙니다."
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
	if len(applied) != 3 {
		t.Errorf("expected 3 applied terms, got %d", len(applied))
	}
}

func TestPronunciationService_ApplyGlossary_NilGlossary(t *testing.T) {
	svc := &PronunciationService{}
	text := "no changes"
	result, applied := svc.applyGlossary(text)
	if result != text {
		t.Errorf("expected %q, got %q", text, result)
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied terms, got %d", len(applied))
	}
}

func TestExtractNarratorText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with narrator tags",
			input:    "<script>\n<narrator>\n변환된 텍스트\n</narrator>\n</script>",
			expected: "변환된 텍스트",
		},
		{
			name:     "with script tags only",
			input:    "<script>\n내용\n</script>",
			expected: "내용",
		},
		{
			name:     "plain text fallback",
			input:    "그냥 텍스트",
			expected: "그냥 텍스트",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNarratorText(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
