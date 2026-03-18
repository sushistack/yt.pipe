package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/store"
)

// GlossaryService handles LLM-based glossary term extraction and suggestion management.
type GlossaryService struct {
	store  *store.Store
	llm    llm.LLM
	logger *slog.Logger
}

// NewGlossaryService creates a new GlossaryService.
func NewGlossaryService(s *store.Store, l llm.LLM, logger *slog.Logger) *GlossaryService {
	return &GlossaryService{store: s, llm: l, logger: logger}
}

// llmTermSuggestion is the JSON structure expected from LLM response.
type llmTermSuggestion struct {
	Term          string `json:"term"`
	Pronunciation string `json:"pronunciation"`
	Definition    string `json:"definition"`
	Category      string `json:"category"`
}

// SuggestTerms extracts SCP terms from scenario text via LLM,
// diffs against existing glossary, and stores new suggestions.
func (gs *GlossaryService) SuggestTerms(ctx context.Context, projectID string, scenarioText string, existingGlossary *glossary.Glossary) ([]*domain.GlossarySuggestion, error) {
	// Build existing terms list for context
	var existingTerms []string
	for _, e := range existingGlossary.Entries() {
		existingTerms = append(existingTerms, e.Term)
	}

	prompt := buildGlossaryPrompt(scenarioText, existingTerms)

	result, err := gs.llm.Complete(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	}, llm.CompletionOptions{Temperature: 0.3})
	if err != nil {
		return nil, fmt.Errorf("glossary suggest: llm complete: %w", err)
	}

	// Parse LLM response
	content := strings.TrimSpace(result.Content)
	// Strip markdown code block if present
	content = stripCodeBlock(content)

	var suggestions []llmTermSuggestion
	if err := json.Unmarshal([]byte(content), &suggestions); err != nil {
		return nil, fmt.Errorf("glossary suggest: parse llm response: %w", err)
	}

	if len(suggestions) == 0 {
		return nil, nil
	}

	// Diff against existing glossary — only store new terms
	var stored []*domain.GlossarySuggestion
	for _, s := range suggestions {
		if s.Term == "" || s.Pronunciation == "" {
			continue
		}
		// Skip if already in glossary
		if _, exists := existingGlossary.Lookup(s.Term); exists {
			continue
		}

		sg := &domain.GlossarySuggestion{
			ProjectID:     projectID,
			Term:          s.Term,
			Pronunciation: s.Pronunciation,
			Definition:    s.Definition,
			Category:      s.Category,
		}
		if err := gs.store.CreateGlossarySuggestion(sg); err != nil {
			// Skip duplicates (UNIQUE constraint) gracefully
			if strings.Contains(err.Error(), "UNIQUE constraint") {
				gs.logger.Debug("skipping duplicate suggestion", "term", s.Term)
				continue
			}
			return nil, fmt.Errorf("glossary suggest: store suggestion: %w", err)
		}
		stored = append(stored, sg)
	}

	gs.logger.Info("glossary suggestions stored",
		"project_id", projectID,
		"llm_returned", len(suggestions),
		"new_stored", len(stored),
	)

	return stored, nil
}

// ApproveSuggestion approves a suggestion and adds it to the glossary.
func (gs *GlossaryService) ApproveSuggestion(ctx context.Context, id int, g *glossary.Glossary) error {
	sg, err := gs.store.GetGlossarySuggestion(id)
	if err != nil {
		return fmt.Errorf("glossary approve: %w", err)
	}
	if err := gs.store.UpdateGlossarySuggestionStatus(id, domain.SuggestionApproved); err != nil {
		return fmt.Errorf("glossary approve: %w", err)
	}
	g.AddEntry(glossary.Entry{
		Term:          sg.Term,
		Pronunciation: sg.Pronunciation,
		Definition:    sg.Definition,
		Category:      sg.Category,
	})
	gs.logger.Info("suggestion approved", "term", sg.Term, "id", id)
	return nil
}

// RejectSuggestion marks a suggestion as rejected.
func (gs *GlossaryService) RejectSuggestion(ctx context.Context, id int) error {
	if err := gs.store.UpdateGlossarySuggestionStatus(id, domain.SuggestionRejected); err != nil {
		return fmt.Errorf("glossary reject: %w", err)
	}
	return nil
}

// ListPendingSuggestions returns all pending suggestions for a project.
func (gs *GlossaryService) ListPendingSuggestions(ctx context.Context, projectID string) ([]*domain.GlossarySuggestion, error) {
	return gs.store.ListGlossarySuggestionsByProject(projectID, domain.SuggestionPending)
}

func buildGlossaryPrompt(scenarioText string, existingTerms []string) string {
	existingContext := "None"
	if len(existingTerms) > 0 {
		existingContext = strings.Join(existingTerms, ", ")
	}

	return fmt.Sprintf(`Extract SCP-related specialized terms from the following Korean narration text.
For each term, provide a Korean pronunciation guide and a brief definition.

EXISTING glossary terms (DO NOT include these): %s

Narration text:
%s

Return a JSON array of objects with these fields:
- "term": the SCP term (e.g., "SCP-173", "Euclid", "격리 등급")
- "pronunciation": Korean pronunciation guide (e.g., "에스씨피 일칠삼")
- "definition": brief definition in Korean
- "category": one of "entity", "containment_class", "organization", "location", "procedure", "anomaly", "other"

Return ONLY a valid JSON array. No other text.`, existingContext, scenarioText)
}

// stripCodeBlock removes markdown code fences from LLM output.
func stripCodeBlock(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
