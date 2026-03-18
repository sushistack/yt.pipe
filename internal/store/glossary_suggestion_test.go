package store

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGlossarySuggestionTestStore(t *testing.T) *Store {
	t.Helper()
	s := setupTestStore(t)
	require.NoError(t, s.CreateProject(&domain.Project{
		ID: "p1", SCPID: "SCP-173", Status: domain.StagePending, WorkspacePath: "/w",
	}))
	return s
}

func TestCreateGlossarySuggestion_Success(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	sg := &domain.GlossarySuggestion{
		ProjectID:     "p1",
		Term:          "SCP-173",
		Pronunciation: "에스씨피 일칠삼",
		Definition:    "The Sculpture",
		Category:      "entity",
	}
	err := s.CreateGlossarySuggestion(sg)
	require.NoError(t, err)
	assert.NotZero(t, sg.ID)
	assert.Equal(t, domain.SuggestionPending, sg.Status)
	assert.False(t, sg.CreatedAt.IsZero())

	// Verify read-back
	got, err := s.GetGlossarySuggestion(sg.ID)
	require.NoError(t, err)
	assert.Equal(t, "SCP-173", got.Term)
	assert.Equal(t, "에스씨피 일칠삼", got.Pronunciation)
	assert.Equal(t, "The Sculpture", got.Definition)
	assert.Equal(t, "entity", got.Category)
}

func TestCreateGlossarySuggestion_DuplicateConstraint(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	sg1 := &domain.GlossarySuggestion{
		ProjectID: "p1", Term: "SCP-173", Pronunciation: "에스씨피 일칠삼",
	}
	require.NoError(t, s.CreateGlossarySuggestion(sg1))

	sg2 := &domain.GlossarySuggestion{
		ProjectID: "p1", Term: "SCP-173", Pronunciation: "다른 발음",
	}
	err := s.CreateGlossarySuggestion(sg2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "UNIQUE constraint")
}

func TestGetGlossarySuggestion_NotFound(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	_, err := s.GetGlossarySuggestion(999)
	assert.Error(t, err)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListGlossarySuggestionsByProject_AllStatuses(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	for _, term := range []string{"SCP-173", "SCP-096", "SCP-682"} {
		require.NoError(t, s.CreateGlossarySuggestion(&domain.GlossarySuggestion{
			ProjectID: "p1", Term: term, Pronunciation: "pron",
		}))
	}

	all, err := s.ListGlossarySuggestionsByProject("p1", "")
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestListGlossarySuggestionsByProject_FilterByStatus(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	sg := &domain.GlossarySuggestion{ProjectID: "p1", Term: "SCP-173", Pronunciation: "p"}
	require.NoError(t, s.CreateGlossarySuggestion(sg))
	require.NoError(t, s.UpdateGlossarySuggestionStatus(sg.ID, domain.SuggestionApproved))

	sg2 := &domain.GlossarySuggestion{ProjectID: "p1", Term: "SCP-096", Pronunciation: "p"}
	require.NoError(t, s.CreateGlossarySuggestion(sg2))

	pending, err := s.ListGlossarySuggestionsByProject("p1", domain.SuggestionPending)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, "SCP-096", pending[0].Term)

	approved, err := s.ListGlossarySuggestionsByProject("p1", domain.SuggestionApproved)
	require.NoError(t, err)
	assert.Len(t, approved, 1)
	assert.Equal(t, "SCP-173", approved[0].Term)
}

func TestUpdateGlossarySuggestionStatus_ValidTransition(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	sg := &domain.GlossarySuggestion{ProjectID: "p1", Term: "SCP-173", Pronunciation: "p"}
	require.NoError(t, s.CreateGlossarySuggestion(sg))

	err := s.UpdateGlossarySuggestionStatus(sg.ID, domain.SuggestionApproved)
	require.NoError(t, err)

	got, err := s.GetGlossarySuggestion(sg.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.SuggestionApproved, got.Status)
}

func TestUpdateGlossarySuggestionStatus_InvalidTransition(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	sg := &domain.GlossarySuggestion{ProjectID: "p1", Term: "SCP-173", Pronunciation: "p"}
	require.NoError(t, s.CreateGlossarySuggestion(sg))
	require.NoError(t, s.UpdateGlossarySuggestionStatus(sg.ID, domain.SuggestionApproved))

	// approved → pending is not allowed
	err := s.UpdateGlossarySuggestionStatus(sg.ID, domain.SuggestionPending)
	assert.Error(t, err)
	assert.IsType(t, &domain.TransitionError{}, err)
}

func TestUpdateGlossarySuggestionStatus_NotFound(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	err := s.UpdateGlossarySuggestionStatus(999, domain.SuggestionApproved)
	assert.Error(t, err)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteGlossarySuggestion_Success(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	sg := &domain.GlossarySuggestion{ProjectID: "p1", Term: "SCP-173", Pronunciation: "p"}
	require.NoError(t, s.CreateGlossarySuggestion(sg))

	err := s.DeleteGlossarySuggestion(sg.ID)
	require.NoError(t, err)

	_, err = s.GetGlossarySuggestion(sg.ID)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteGlossarySuggestion_NotFound(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	err := s.DeleteGlossarySuggestion(999)
	assert.Error(t, err)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestCreateGlossarySuggestion_Validation(t *testing.T) {
	s := setupGlossarySuggestionTestStore(t)

	// Empty project_id
	err := s.CreateGlossarySuggestion(&domain.GlossarySuggestion{Term: "t", Pronunciation: "p"})
	assert.IsType(t, &domain.ValidationError{}, err)

	// Empty term
	err = s.CreateGlossarySuggestion(&domain.GlossarySuggestion{ProjectID: "p1", Pronunciation: "p"})
	assert.IsType(t, &domain.ValidationError{}, err)

	// Empty pronunciation
	err = s.CreateGlossarySuggestion(&domain.GlossarySuggestion{ProjectID: "p1", Term: "t"})
	assert.IsType(t, &domain.ValidationError{}, err)
}
