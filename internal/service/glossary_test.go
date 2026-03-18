package service

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/glossary"
	"github.com/sushistack/yt.pipe/internal/mocks"
	"github.com/sushistack/yt.pipe/internal/plugin/llm"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupGlossaryService(t *testing.T) (*GlossaryService, *store.Store, *mocks.MockLLM) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w",
	}))

	mockLLM := mocks.NewMockLLM(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewGlossaryService(s, mockLLM, logger)
	return svc, s, mockLLM
}

func TestSuggestTerms_Success(t *testing.T) {
	svc, s, mockLLM := setupGlossaryService(t)
	ctx := context.Background()

	llmResponse := `[
		{"term": "SCP-173", "pronunciation": "에스씨피 일칠삼", "definition": "조각상", "category": "entity"},
		{"term": "Euclid", "pronunciation": "유클리드", "definition": "격리 등급", "category": "containment_class"}
	]`
	mockLLM.On("Complete", ctx, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: llmResponse}, nil)

	existing := glossary.New()
	suggestions, err := svc.SuggestTerms(ctx, "p1", "SCP-173은 Euclid 등급의 개체입니다.", existing)
	require.NoError(t, err)
	assert.Len(t, suggestions, 2)
	assert.Equal(t, "SCP-173", suggestions[0].Term)
	assert.Equal(t, "Euclid", suggestions[1].Term)
	assert.Equal(t, domain.SuggestionPending, suggestions[0].Status)

	// Verify stored in DB
	stored, err := s.ListGlossarySuggestionsByProject("p1", "")
	require.NoError(t, err)
	assert.Len(t, stored, 2)
}

func TestSuggestTerms_FilterExisting(t *testing.T) {
	svc, _, mockLLM := setupGlossaryService(t)
	ctx := context.Background()

	llmResponse := `[
		{"term": "SCP-173", "pronunciation": "에스씨피", "definition": "existing", "category": "entity"},
		{"term": "SCP-096", "pronunciation": "영구육", "definition": "new term", "category": "entity"}
	]`
	mockLLM.On("Complete", ctx, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: llmResponse}, nil)

	// Create glossary with SCP-173 already existing
	existing := glossary.New()
	// Manually write a glossary file and load it
	tmpFile := t.TempDir() + "/glossary.json"
	require.NoError(t, glossary.WriteToFile(tmpFile, []glossary.Entry{
		{Term: "SCP-173", Pronunciation: "existing", Definition: "existing", Category: "entity"},
	}))
	existing = glossary.LoadFromFile(tmpFile)

	suggestions, err := svc.SuggestTerms(ctx, "p1", "text", existing)
	require.NoError(t, err)
	// Only SCP-096 should be stored (SCP-173 is already in glossary)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "SCP-096", suggestions[0].Term)
}

func TestSuggestTerms_InvalidJSON(t *testing.T) {
	svc, _, mockLLM := setupGlossaryService(t)
	ctx := context.Background()

	mockLLM.On("Complete", ctx, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: "not valid json"}, nil)

	existing := glossary.New()
	_, err := svc.SuggestTerms(ctx, "p1", "text", existing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse llm response")
}

func TestSuggestTerms_NoNewTerms(t *testing.T) {
	svc, _, mockLLM := setupGlossaryService(t)
	ctx := context.Background()

	mockLLM.On("Complete", ctx, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: "[]"}, nil)

	existing := glossary.New()
	suggestions, err := svc.SuggestTerms(ctx, "p1", "text", existing)
	require.NoError(t, err)
	assert.Nil(t, suggestions)
}

func TestSuggestTerms_LLMError(t *testing.T) {
	svc, _, mockLLM := setupGlossaryService(t)
	ctx := context.Background()

	mockLLM.On("Complete", ctx, mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	existing := glossary.New()
	_, err := svc.SuggestTerms(ctx, "p1", "text", existing)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm complete")
}

func TestSuggestTerms_CodeBlockStripping(t *testing.T) {
	svc, _, mockLLM := setupGlossaryService(t)
	ctx := context.Background()

	llmResponse := "```json\n[{\"term\": \"SCP-999\", \"pronunciation\": \"구구구\", \"definition\": \"d\", \"category\": \"entity\"}]\n```"
	mockLLM.On("Complete", ctx, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: llmResponse}, nil)

	existing := glossary.New()
	suggestions, err := svc.SuggestTerms(ctx, "p1", "text", existing)
	require.NoError(t, err)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "SCP-999", suggestions[0].Term)
}

func TestSuggestTerms_DuplicateSkipped(t *testing.T) {
	svc, s, mockLLM := setupGlossaryService(t)
	ctx := context.Background()

	// Pre-insert a suggestion
	require.NoError(t, s.CreateGlossarySuggestion(&domain.GlossarySuggestion{
		ProjectID: "p1", Term: "SCP-173", Pronunciation: "p",
	}))

	llmResponse := `[
		{"term": "SCP-173", "pronunciation": "에스씨피", "definition": "dup", "category": "entity"},
		{"term": "SCP-682", "pronunciation": "육팔이", "definition": "new", "category": "entity"}
	]`
	mockLLM.On("Complete", ctx, mock.Anything, mock.Anything).
		Return(&llm.CompletionResult{Content: llmResponse}, nil)

	existing := glossary.New()
	suggestions, err := svc.SuggestTerms(ctx, "p1", "text", existing)
	require.NoError(t, err)
	// SCP-173 is already in store (UNIQUE constraint), so only SCP-682
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "SCP-682", suggestions[0].Term)
}

func TestApproveSuggestion_Success(t *testing.T) {
	svc, s, _ := setupGlossaryService(t)
	ctx := context.Background()

	sg := &domain.GlossarySuggestion{
		ProjectID: "p1", Term: "SCP-173", Pronunciation: "에스씨피", Definition: "조각상", Category: "entity",
	}
	require.NoError(t, s.CreateGlossarySuggestion(sg))

	g := glossary.New()
	err := svc.ApproveSuggestion(ctx, sg.ID, g)
	require.NoError(t, err)

	// Verify status changed
	got, err := s.GetGlossarySuggestion(sg.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.SuggestionApproved, got.Status)

	// Verify added to glossary
	entry, ok := g.Lookup("SCP-173")
	assert.True(t, ok)
	assert.Equal(t, "에스씨피", entry.Pronunciation)
}

func TestRejectSuggestion_Success(t *testing.T) {
	svc, s, _ := setupGlossaryService(t)
	ctx := context.Background()

	sg := &domain.GlossarySuggestion{
		ProjectID: "p1", Term: "SCP-173", Pronunciation: "p",
	}
	require.NoError(t, s.CreateGlossarySuggestion(sg))

	err := svc.RejectSuggestion(ctx, sg.ID)
	require.NoError(t, err)

	got, err := s.GetGlossarySuggestion(sg.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.SuggestionRejected, got.Status)
}

func TestListPendingSuggestions(t *testing.T) {
	svc, s, _ := setupGlossaryService(t)
	ctx := context.Background()

	require.NoError(t, s.CreateGlossarySuggestion(&domain.GlossarySuggestion{
		ProjectID: "p1", Term: "SCP-173", Pronunciation: "p1",
	}))
	sg2 := &domain.GlossarySuggestion{ProjectID: "p1", Term: "SCP-096", Pronunciation: "p2"}
	require.NoError(t, s.CreateGlossarySuggestion(sg2))
	require.NoError(t, s.UpdateGlossarySuggestionStatus(sg2.ID, domain.SuggestionApproved))

	pending, err := svc.ListPendingSuggestions(ctx, "p1")
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, "SCP-173", pending[0].Term)
}

func TestStripCodeBlock(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`[{"term":"a"}]`, `[{"term":"a"}]`},
		{"```json\n[{\"term\":\"a\"}]\n```", `[{"term":"a"}]`},
		{"```\n[{\"term\":\"a\"}]\n```", `[{"term":"a"}]`},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, stripCodeBlock(tt.input))
	}
}
