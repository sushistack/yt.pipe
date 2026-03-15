package store

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCharacter(id, scpID, name string, aliases []string) *domain.Character {
	return &domain.Character{
		ID:               id,
		SCPID:            scpID,
		CanonicalName:    name,
		Aliases:          aliases,
		VisualDescriptor: "visual desc",
		StyleGuide:       "style guide",
		ImagePromptBase:  "prompt base",
	}
}

func TestCreateCharacter_Success(t *testing.T) {
	s := setupTestStore(t)
	c := newTestCharacter("c1", "SCP-173", "SCP-173", []string{"The Sculpture", "조각상"})
	err := s.CreateCharacter(c)
	require.NoError(t, err)
	assert.False(t, c.CreatedAt.IsZero())
	assert.False(t, c.UpdatedAt.IsZero())
}

func TestCreateCharacter_EmptyAliases(t *testing.T) {
	s := setupTestStore(t)
	c := newTestCharacter("c1", "SCP-173", "SCP-173", []string{})
	err := s.CreateCharacter(c)
	require.NoError(t, err)
}

func TestCreateCharacter_NilAliases(t *testing.T) {
	s := setupTestStore(t)
	c := newTestCharacter("c1", "SCP-173", "SCP-173", nil)
	err := s.CreateCharacter(c)
	require.NoError(t, err)
}

func TestGetCharacter_Success(t *testing.T) {
	s := setupTestStore(t)
	c := newTestCharacter("c1", "SCP-173", "SCP-173", []string{"The Sculpture", "조각상"})
	require.NoError(t, s.CreateCharacter(c))

	got, err := s.GetCharacter("c1")
	require.NoError(t, err)
	assert.Equal(t, "c1", got.ID)
	assert.Equal(t, "SCP-173", got.SCPID)
	assert.Equal(t, "SCP-173", got.CanonicalName)
	assert.Equal(t, []string{"The Sculpture", "조각상"}, got.Aliases)
	assert.Equal(t, "visual desc", got.VisualDescriptor)
	assert.Equal(t, "style guide", got.StyleGuide)
	assert.Equal(t, "prompt base", got.ImagePromptBase)
}

func TestGetCharacter_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetCharacter("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListCharactersBySCPID_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCharacter(newTestCharacter("c1", "SCP-173", "SCP-173", []string{"조각상"})))
	require.NoError(t, s.CreateCharacter(newTestCharacter("c2", "SCP-173", "Dr. Bright", []string{})))
	require.NoError(t, s.CreateCharacter(newTestCharacter("c3", "SCP-682", "SCP-682", []string{})))

	chars, err := s.ListCharactersBySCPID("SCP-173")
	require.NoError(t, err)
	assert.Len(t, chars, 2)
}

func TestListCharactersBySCPID_Empty(t *testing.T) {
	s := setupTestStore(t)
	chars, err := s.ListCharactersBySCPID("SCP-999")
	require.NoError(t, err)
	assert.Empty(t, chars)
}

func TestListAllCharacters(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCharacter(newTestCharacter("c1", "SCP-173", "SCP-173", nil)))
	require.NoError(t, s.CreateCharacter(newTestCharacter("c2", "SCP-682", "SCP-682", nil)))

	chars, err := s.ListAllCharacters()
	require.NoError(t, err)
	assert.Len(t, chars, 2)
}

func TestUpdateCharacter_Success(t *testing.T) {
	s := setupTestStore(t)
	c := newTestCharacter("c1", "SCP-173", "SCP-173", []string{"조각상"})
	require.NoError(t, s.CreateCharacter(c))

	c.CanonicalName = "The Sculpture"
	c.Aliases = []string{"SCP-173", "조각상", "Peanut"}
	c.VisualDescriptor = "updated visual"
	err := s.UpdateCharacter(c)
	require.NoError(t, err)

	got, _ := s.GetCharacter("c1")
	assert.Equal(t, "The Sculpture", got.CanonicalName)
	assert.Len(t, got.Aliases, 3)
	assert.Equal(t, "updated visual", got.VisualDescriptor)
}

func TestUpdateCharacter_NotFound(t *testing.T) {
	s := setupTestStore(t)
	c := newTestCharacter("nonexistent", "SCP-173", "SCP-173", nil)
	err := s.UpdateCharacter(c)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteCharacter_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCharacter(newTestCharacter("c1", "SCP-173", "SCP-173", nil)))

	err := s.DeleteCharacter("c1")
	require.NoError(t, err)

	_, err = s.GetCharacter("c1")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteCharacter_NotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.DeleteCharacter("nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestSearchCharactersByName_CanonicalNameMatch(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCharacter(newTestCharacter("c1", "SCP-173", "SCP-173", []string{"조각상"})))
	require.NoError(t, s.CreateCharacter(newTestCharacter("c2", "SCP-682", "SCP-682", []string{"도마뱀"})))

	chars, err := s.SearchCharactersByName("SCP-173")
	require.NoError(t, err)
	assert.Len(t, chars, 1)
	assert.Equal(t, "SCP-173", chars[0].CanonicalName)
}

func TestSearchCharactersByName_AliasMatch(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCharacter(newTestCharacter("c1", "SCP-173", "SCP-173", []string{"The Sculpture", "조각상"})))

	chars, err := s.SearchCharactersByName("조각상")
	require.NoError(t, err)
	assert.Len(t, chars, 1)
	assert.Equal(t, "SCP-173", chars[0].CanonicalName)
}

func TestSearchCharactersByName_CaseInsensitive(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCharacter(newTestCharacter("c1", "SCP-173", "SCP-173", []string{"The Sculpture"})))

	chars, err := s.SearchCharactersByName("scp-173")
	require.NoError(t, err)
	assert.Len(t, chars, 1)
}

func TestSearchCharactersByName_NoMatch(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCharacter(newTestCharacter("c1", "SCP-173", "SCP-173", []string{"조각상"})))

	chars, err := s.SearchCharactersByName("nonexistent")
	require.NoError(t, err)
	assert.Empty(t, chars)
}

// --- Character Candidate CRUD Tests ---

func TestCreateCandidateBatch_Success(t *testing.T) {
	s := setupTestStore(t)
	err := s.CreateCandidateBatch("proj1", "SCP-173", 4)
	require.NoError(t, err)

	candidates, err := s.ListCandidatesByProject("proj1")
	require.NoError(t, err)
	assert.Len(t, candidates, 4)
	for i, c := range candidates {
		assert.NotEmpty(t, c.ID)
		assert.Equal(t, "proj1", c.ProjectID)
		assert.Equal(t, "SCP-173", c.SCPID)
		assert.Equal(t, i+1, c.CandidateNum)
		assert.Equal(t, "pending", c.Status)
	}
}

func TestCreateCandidateBatch_ZeroCount(t *testing.T) {
	s := setupTestStore(t)
	err := s.CreateCandidateBatch("proj1", "SCP-173", 0)
	require.NoError(t, err)

	candidates, err := s.ListCandidatesByProject("proj1")
	require.NoError(t, err)
	assert.Empty(t, candidates)
}

func TestListCandidatesByProject_OrderedByCandidateNum(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 3))

	candidates, err := s.ListCandidatesByProject("proj1")
	require.NoError(t, err)
	require.Len(t, candidates, 3)
	assert.Equal(t, 1, candidates[0].CandidateNum)
	assert.Equal(t, 2, candidates[1].CandidateNum)
	assert.Equal(t, 3, candidates[2].CandidateNum)
}

func TestListCandidatesByProject_IsolatesProjects(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 2))
	require.NoError(t, s.CreateCandidateBatch("proj2", "SCP-682", 3))

	c1, err := s.ListCandidatesByProject("proj1")
	require.NoError(t, err)
	assert.Len(t, c1, 2)

	c2, err := s.ListCandidatesByProject("proj2")
	require.NoError(t, err)
	assert.Len(t, c2, 3)
}

func TestUpdateCandidateStatus_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 1))

	candidates, _ := s.ListCandidatesByProject("proj1")
	require.Len(t, candidates, 1)

	err := s.UpdateCandidateStatus(candidates[0].ID, "ready", "/img/candidate_1.png", "description", "")
	require.NoError(t, err)

	updated, _ := s.ListCandidatesByProject("proj1")
	assert.Equal(t, "ready", updated[0].Status)
	assert.Equal(t, "/img/candidate_1.png", updated[0].ImagePath)
	assert.Equal(t, "description", updated[0].Description)
}

func TestUpdateCandidateStatus_Failed(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 1))

	candidates, _ := s.ListCandidatesByProject("proj1")
	err := s.UpdateCandidateStatus(candidates[0].ID, "failed", "", "", "LLM error")
	require.NoError(t, err)

	updated, _ := s.ListCandidatesByProject("proj1")
	assert.Equal(t, "failed", updated[0].Status)
	assert.Equal(t, "LLM error", updated[0].ErrorDetail)
}

func TestDeleteCandidatesByProject_Success(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 4))

	err := s.DeleteCandidatesByProject("proj1")
	require.NoError(t, err)

	candidates, _ := s.ListCandidatesByProject("proj1")
	assert.Empty(t, candidates)
}

func TestDeleteCandidatesByProject_DoesNotAffectOtherProjects(t *testing.T) {
	s := setupTestStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 2))
	require.NoError(t, s.CreateCandidateBatch("proj2", "SCP-682", 3))

	require.NoError(t, s.DeleteCandidatesByProject("proj1"))

	c1, _ := s.ListCandidatesByProject("proj1")
	assert.Empty(t, c1)

	c2, _ := s.ListCandidatesByProject("proj2")
	assert.Len(t, c2, 3)
}
