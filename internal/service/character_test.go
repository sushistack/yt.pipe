package service

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCharacterService(t *testing.T) *CharacterService {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return NewCharacterService(s)
}

func TestCreateCharacter_Success(t *testing.T) {
	cs := setupCharacterService(t)
	c, err := cs.CreateCharacter("SCP-173", "SCP-173", []string{"The Sculpture", "조각상"},
		"concrete humanoid", "dark institutional", "concrete sculpture in containment")
	require.NoError(t, err)
	assert.NotEmpty(t, c.ID)
	assert.Equal(t, "SCP-173", c.SCPID)
	assert.Equal(t, "SCP-173", c.CanonicalName)
	assert.Len(t, c.Aliases, 2)
}

func TestCreateCharacter_EmptySCPID(t *testing.T) {
	cs := setupCharacterService(t)
	_, err := cs.CreateCharacter("", "SCP-173", nil, "", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scp_id")
}

func TestCreateCharacter_EmptyName(t *testing.T) {
	cs := setupCharacterService(t)
	_, err := cs.CreateCharacter("SCP-173", "", nil, "", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "canonical_name")
}

func TestCreateCharacter_InvalidAliases(t *testing.T) {
	cs := setupCharacterService(t)
	_, err := cs.CreateCharacter("SCP-173", "SCP-173", []string{"valid", ""}, "", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aliases")
}

func TestGetCharacter_Success(t *testing.T) {
	cs := setupCharacterService(t)
	created, _ := cs.CreateCharacter("SCP-173", "SCP-173", nil, "visual", "", "")

	got, err := cs.GetCharacter(created.ID)
	require.NoError(t, err)
	assert.Equal(t, "SCP-173", got.CanonicalName)
}

func TestListCharacters_BySCPID(t *testing.T) {
	cs := setupCharacterService(t)
	cs.CreateCharacter("SCP-173", "SCP-173", nil, "", "", "")
	cs.CreateCharacter("SCP-173", "Dr. Bright", nil, "", "", "")
	cs.CreateCharacter("SCP-682", "SCP-682", nil, "", "", "")

	chars, err := cs.ListCharacters("SCP-173")
	require.NoError(t, err)
	assert.Len(t, chars, 2)
}

func TestListCharacters_All(t *testing.T) {
	cs := setupCharacterService(t)
	cs.CreateCharacter("SCP-173", "SCP-173", nil, "", "", "")
	cs.CreateCharacter("SCP-682", "SCP-682", nil, "", "", "")

	chars, err := cs.ListCharacters("")
	require.NoError(t, err)
	assert.Len(t, chars, 2)
}

func TestUpdateCharacter_Success(t *testing.T) {
	cs := setupCharacterService(t)
	c, _ := cs.CreateCharacter("SCP-173", "SCP-173", nil, "old visual", "", "")

	updated, err := cs.UpdateCharacter(c.ID, "The Sculpture", []string{"SCP-173"}, "new visual", "", "")
	require.NoError(t, err)
	assert.Equal(t, "The Sculpture", updated.CanonicalName)
	assert.Equal(t, "new visual", updated.VisualDescriptor)
}

func TestUpdateCharacter_PartialUpdate(t *testing.T) {
	cs := setupCharacterService(t)
	c, _ := cs.CreateCharacter("SCP-173", "SCP-173", []string{"조각상"}, "visual", "style", "prompt")

	// Update only name, keep everything else
	updated, err := cs.UpdateCharacter(c.ID, "New Name", nil, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.CanonicalName)
	assert.Equal(t, []string{"조각상"}, updated.Aliases) // unchanged
	assert.Equal(t, "visual", updated.VisualDescriptor)  // unchanged
}

func TestDeleteCharacter_Success(t *testing.T) {
	cs := setupCharacterService(t)
	c, _ := cs.CreateCharacter("SCP-173", "SCP-173", nil, "", "", "")

	err := cs.DeleteCharacter(c.ID)
	require.NoError(t, err)

	_, err = cs.GetCharacter(c.ID)
	assert.Error(t, err)
}

func TestMatchCharacters_CanonicalNameMatch(t *testing.T) {
	cs := setupCharacterService(t)
	cs.CreateCharacter("SCP-173", "SCP-173", []string{"The Sculpture"}, "concrete figure", "", "concrete in room")
	cs.CreateCharacter("SCP-173", "Dr. Bright", nil, "man in lab coat", "", "scientist")

	refs, err := cs.MatchCharacters("SCP-173", "SCP-173이 복도 끝에서 서 있었다")
	require.NoError(t, err)
	assert.Len(t, refs, 1)
	assert.Equal(t, "SCP-173", refs[0].Name)
	assert.Equal(t, "concrete figure", refs[0].VisualDescriptor)
}

func TestMatchCharacters_AliasMatch(t *testing.T) {
	cs := setupCharacterService(t)
	cs.CreateCharacter("SCP-173", "SCP-173", []string{"The Sculpture", "조각상"}, "concrete", "", "prompt")

	refs, err := cs.MatchCharacters("SCP-173", "조각상이 움직였다")
	require.NoError(t, err)
	assert.Len(t, refs, 1)
	assert.Equal(t, "SCP-173", refs[0].Name)
}

func TestMatchCharacters_MultipleMatches(t *testing.T) {
	cs := setupCharacterService(t)
	cs.CreateCharacter("SCP-173", "SCP-173", []string{"조각상"}, "concrete", "", "")
	cs.CreateCharacter("SCP-173", "Dr. Bright", []string{"브라이트 박사"}, "scientist", "", "")

	refs, err := cs.MatchCharacters("SCP-173", "SCP-173과 Dr. Bright가 대치했다")
	require.NoError(t, err)
	assert.Len(t, refs, 2)
}

func TestMatchCharacters_Deduplicated(t *testing.T) {
	cs := setupCharacterService(t)
	cs.CreateCharacter("SCP-173", "SCP-173", []string{"조각상"}, "concrete", "", "")

	// Both canonical name and alias match — should return 1, not 2
	refs, err := cs.MatchCharacters("SCP-173", "SCP-173은 조각상이다")
	require.NoError(t, err)
	assert.Len(t, refs, 1)
}

func TestMatchCharacters_NoMatch(t *testing.T) {
	cs := setupCharacterService(t)
	cs.CreateCharacter("SCP-173", "SCP-173", []string{"조각상"}, "concrete", "", "")

	refs, err := cs.MatchCharacters("SCP-173", "복도가 어두웠다")
	require.NoError(t, err)
	assert.Empty(t, refs)
}

func TestMatchCharacters_CaseInsensitive(t *testing.T) {
	cs := setupCharacterService(t)
	cs.CreateCharacter("SCP-173", "SCP-173", []string{"The Sculpture"}, "concrete", "", "")

	refs, err := cs.MatchCharacters("SCP-173", "the sculpture appeared")
	require.NoError(t, err)
	assert.Len(t, refs, 1)
}

func TestMatchCharacters_NoCharactersExist(t *testing.T) {
	cs := setupCharacterService(t)

	refs, err := cs.MatchCharacters("SCP-173", "SCP-173이 나타났다")
	require.NoError(t, err)
	assert.Empty(t, refs)
}

// --- GetCandidateGenerationStatus Tests ---

func setupCharacterServiceWithStore(t *testing.T) (*CharacterService, *store.Store) {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return NewCharacterService(s), s
}

func TestGetCandidateGenerationStatus_Empty(t *testing.T) {
	cs, _ := setupCharacterServiceWithStore(t)
	status, err := cs.GetCandidateGenerationStatus("proj1")
	require.NoError(t, err)
	assert.Equal(t, "empty", status)
}

func TestGetCandidateGenerationStatus_AllPending(t *testing.T) {
	cs, s := setupCharacterServiceWithStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 4))

	status, err := cs.GetCandidateGenerationStatus("proj1")
	require.NoError(t, err)
	assert.Equal(t, "generating", status)
}

func TestGetCandidateGenerationStatus_AllReady(t *testing.T) {
	cs, s := setupCharacterServiceWithStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 2))

	candidates, _ := s.ListCandidatesByProject("proj1")
	for _, c := range candidates {
		require.NoError(t, s.UpdateCandidateStatus(c.ID, "ready", "/img.png", "desc", ""))
	}

	status, err := cs.GetCandidateGenerationStatus("proj1")
	require.NoError(t, err)
	assert.Equal(t, "ready", status)
}

func TestGetCandidateGenerationStatus_MixedGeneratingAndReady(t *testing.T) {
	cs, s := setupCharacterServiceWithStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 3))

	candidates, _ := s.ListCandidatesByProject("proj1")
	require.NoError(t, s.UpdateCandidateStatus(candidates[0].ID, "ready", "/img.png", "desc", ""))
	// candidates[1] and [2] remain "pending"

	status, err := cs.GetCandidateGenerationStatus("proj1")
	require.NoError(t, err)
	assert.Equal(t, "generating", status)
}

func TestGetCandidateGenerationStatus_Failed(t *testing.T) {
	cs, s := setupCharacterServiceWithStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 2))

	candidates, _ := s.ListCandidatesByProject("proj1")
	require.NoError(t, s.UpdateCandidateStatus(candidates[0].ID, "ready", "/img.png", "desc", ""))
	require.NoError(t, s.UpdateCandidateStatus(candidates[1].ID, "failed", "", "", "LLM error"))

	status, err := cs.GetCandidateGenerationStatus("proj1")
	require.NoError(t, err)
	assert.Equal(t, "failed", status)
}

func TestGetCandidateGenerationStatus_FailedOverridesReady(t *testing.T) {
	cs, s := setupCharacterServiceWithStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 3))

	candidates, _ := s.ListCandidatesByProject("proj1")
	require.NoError(t, s.UpdateCandidateStatus(candidates[0].ID, "ready", "/img.png", "desc", ""))
	require.NoError(t, s.UpdateCandidateStatus(candidates[1].ID, "ready", "/img2.png", "desc2", ""))
	require.NoError(t, s.UpdateCandidateStatus(candidates[2].ID, "failed", "", "", "timeout"))

	status, err := cs.GetCandidateGenerationStatus("proj1")
	require.NoError(t, err)
	assert.Equal(t, "failed", status)
}

func TestListCandidates_DelegatesToStore(t *testing.T) {
	cs, s := setupCharacterServiceWithStore(t)
	require.NoError(t, s.CreateCandidateBatch("proj1", "SCP-173", 3))

	candidates, err := cs.ListCandidates("proj1")
	require.NoError(t, err)
	assert.Len(t, candidates, 3)
}
