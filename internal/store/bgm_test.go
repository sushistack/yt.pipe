package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sushistack/yt.pipe/internal/domain"
)

func createTestBGM(t *testing.T, s *Store, id string, tags []string) *domain.BGM {
	t.Helper()
	b := &domain.BGM{
		ID:            id,
		Name:          "BGM " + id,
		FilePath:      "/music/" + id + ".mp3",
		MoodTags:      tags,
		DurationMs:    120000,
		LicenseType:   domain.LicenseRoyaltyFree,
		LicenseSource: "https://example.com",
		CreditText:    "Test Artist - " + id,
	}
	require.NoError(t, s.CreateBGM(b))
	return b
}

func TestCreateBGM_Success(t *testing.T) {
	s := setupTestStore(t)
	b := createTestBGM(t, s, "bgm-1", []string{"epic", "dark"})
	assert.False(t, b.CreatedAt.IsZero())
}

func TestGetBGM_Success(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-1", []string{"epic", "dark"})

	got, err := s.GetBGM("bgm-1")
	require.NoError(t, err)
	assert.Equal(t, "bgm-1", got.ID)
	assert.Equal(t, "BGM bgm-1", got.Name)
	assert.Equal(t, []string{"epic", "dark"}, got.MoodTags)
	assert.Equal(t, domain.LicenseRoyaltyFree, got.LicenseType)
	assert.Equal(t, int64(120000), got.DurationMs)
}

func TestGetBGM_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetBGM("nonexistent")
	assert.Error(t, err)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListBGMs(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-b", []string{"calm"})
	createTestBGM(t, s, "bgm-a", []string{"epic"})

	bgms, err := s.ListBGMs()
	require.NoError(t, err)
	assert.Len(t, bgms, 2)
	assert.Equal(t, "bgm-a", bgms[0].ID) // ordered by name
}

func TestUpdateBGM_Success(t *testing.T) {
	s := setupTestStore(t)
	b := createTestBGM(t, s, "bgm-1", []string{"epic"})

	b.Name = "Updated BGM"
	b.MoodTags = []string{"calm", "ambient"}
	require.NoError(t, s.UpdateBGM(b))

	got, err := s.GetBGM("bgm-1")
	require.NoError(t, err)
	assert.Equal(t, "Updated BGM", got.Name)
	assert.Equal(t, []string{"calm", "ambient"}, got.MoodTags)
}

func TestUpdateBGM_NotFound(t *testing.T) {
	s := setupTestStore(t)
	b := &domain.BGM{ID: "nonexistent", Name: "x", FilePath: "/x", LicenseType: domain.LicenseCustom}
	err := s.UpdateBGM(b)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteBGM_Success(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-1", []string{"epic"})

	err := s.DeleteBGM("bgm-1")
	require.NoError(t, err)

	_, err = s.GetBGM("bgm-1")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteBGM_NotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.DeleteBGM("nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteBGM_FailsWithAssignments(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-1", []string{"epic"})

	// Create an assignment
	a := &domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1"}
	require.NoError(t, s.AssignBGMToScene(a))

	err := s.DeleteBGM("bgm-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete bgm")
}

func TestSearchByMoodTags_SingleTag(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-1", []string{"epic", "dark"})
	createTestBGM(t, s, "bgm-2", []string{"calm", "ambient"})
	createTestBGM(t, s, "bgm-3", []string{"epic", "heroic"})

	results, err := s.SearchByMoodTags([]string{"epic"})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestSearchByMoodTags_MultipleTags_RankedByMatches(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-1", []string{"epic", "dark"})    // 2 matches
	createTestBGM(t, s, "bgm-2", []string{"calm"})             // 0 matches
	createTestBGM(t, s, "bgm-3", []string{"epic", "heroic"})   // 1 match

	results, err := s.SearchByMoodTags([]string{"epic", "dark"})
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "bgm-1", results[0].ID) // 2 matches first
}

func TestSearchByMoodTags_Empty(t *testing.T) {
	s := setupTestStore(t)
	results, err := s.SearchByMoodTags([]string{})
	require.NoError(t, err)
	assert.Nil(t, results)
}

func TestAssignBGMToScene_Success(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-1", []string{"epic"})

	a := &domain.SceneBGMAssignment{
		ProjectID:       "p1",
		SceneNum:        1,
		BGMID:           "bgm-1",
		VolumeDB:        -3.0,
		FadeInMs:        1000,
		FadeOutMs:       2000,
		DuckingDB:       -12.0,
		AutoRecommended: true,
		Confirmed:       false,
	}
	require.NoError(t, s.AssignBGMToScene(a))

	got, err := s.GetSceneBGMAssignment("p1", 1)
	require.NoError(t, err)
	assert.Equal(t, "bgm-1", got.BGMID)
	assert.Equal(t, -3.0, got.VolumeDB)
	assert.True(t, got.AutoRecommended)
	assert.False(t, got.Confirmed)
}

func TestAssignBGMToScene_Upsert(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-1", []string{"epic"})
	createTestBGM(t, s, "bgm-2", []string{"calm"})

	a := &domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1"}
	require.NoError(t, s.AssignBGMToScene(a))

	// Reassign to different BGM
	a.BGMID = "bgm-2"
	a.Confirmed = true
	require.NoError(t, s.AssignBGMToScene(a))

	got, err := s.GetSceneBGMAssignment("p1", 1)
	require.NoError(t, err)
	assert.Equal(t, "bgm-2", got.BGMID)
	assert.True(t, got.Confirmed)
}

func TestConfirmSceneBGM_Success(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-1", []string{"epic"})

	a := &domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1"}
	require.NoError(t, s.AssignBGMToScene(a))

	require.NoError(t, s.ConfirmSceneBGM("p1", 1))

	got, err := s.GetSceneBGMAssignment("p1", 1)
	require.NoError(t, err)
	assert.True(t, got.Confirmed)
}

func TestConfirmSceneBGM_NotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.ConfirmSceneBGM("p1", 99)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestGetSceneBGMAssignment_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetSceneBGMAssignment("p1", 99)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListSceneBGMAssignments(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-1", []string{"epic"})
	createTestBGM(t, s, "bgm-2", []string{"calm"})

	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 2, BGMID: "bgm-2"}))
	require.NoError(t, s.AssignBGMToScene(&domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1"}))

	assignments, err := s.ListSceneBGMAssignments("p1")
	require.NoError(t, err)
	assert.Len(t, assignments, 2)
	assert.Equal(t, 1, assignments[0].SceneNum) // ordered by scene_num
	assert.Equal(t, 2, assignments[1].SceneNum)
}

func TestListSceneBGMAssignments_Empty(t *testing.T) {
	s := setupTestStore(t)
	assignments, err := s.ListSceneBGMAssignments("no-project")
	require.NoError(t, err)
	assert.Empty(t, assignments)
}

func TestSceneBGMAssignment_DefaultValues(t *testing.T) {
	s := setupTestStore(t)
	createTestBGM(t, s, "bgm-1", []string{"epic"})

	// Assignment with zero-value params should get DB defaults
	a := &domain.SceneBGMAssignment{ProjectID: "p1", SceneNum: 1, BGMID: "bgm-1"}
	require.NoError(t, s.AssignBGMToScene(a))

	got, err := s.GetSceneBGMAssignment("p1", 1)
	require.NoError(t, err)
	assert.Equal(t, float64(0), got.VolumeDB)
	assert.Equal(t, 0, got.FadeInMs)
	assert.Equal(t, 0, got.FadeOutMs)
	assert.Equal(t, float64(0), got.DuckingDB)
}
