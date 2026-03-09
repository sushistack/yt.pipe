package store

import (
	"fmt"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestTemplate(t *testing.T, s *Store, id string, category domain.TemplateCategory) *domain.PromptTemplate {
	t.Helper()
	tmpl := &domain.PromptTemplate{
		ID:       id,
		Category: category,
		Name:     "Test " + id,
		Content:  "content for " + id,
	}
	require.NoError(t, s.CreateTemplate(tmpl))
	return tmpl
}

func TestCreateTemplate_Success(t *testing.T) {
	s := setupTestStore(t)
	tmpl := &domain.PromptTemplate{
		ID:       "t-1",
		Category: domain.CategoryScenario,
		Name:     "Scenario Template",
		Content:  "Write a scenario for {{.SCPID}}",
	}
	err := s.CreateTemplate(tmpl)
	require.NoError(t, err)
	assert.Equal(t, 1, tmpl.Version)
	assert.False(t, tmpl.CreatedAt.IsZero())
	assert.False(t, tmpl.UpdatedAt.IsZero())
}

func TestCreateTemplate_CreatesVersion1(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)

	versions, err := s.ListTemplateVersions("t-1")
	require.NoError(t, err)
	require.Len(t, versions, 1)
	assert.Equal(t, 1, versions[0].Version)
	assert.Equal(t, "content for t-1", versions[0].Content)
}

func TestCreateTemplate_DefaultFlag(t *testing.T) {
	s := setupTestStore(t)
	tmpl := &domain.PromptTemplate{
		ID:        "t-def",
		Category:  domain.CategoryImage,
		Name:      "Default Image",
		Content:   "default content",
		IsDefault: true,
	}
	require.NoError(t, s.CreateTemplate(tmpl))

	got, err := s.GetTemplate("t-def")
	require.NoError(t, err)
	assert.True(t, got.IsDefault)
}

func TestGetTemplate_Success(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryTTS)

	got, err := s.GetTemplate("t-1")
	require.NoError(t, err)
	assert.Equal(t, "t-1", got.ID)
	assert.Equal(t, domain.CategoryTTS, got.Category)
	assert.Equal(t, "Test t-1", got.Name)
}

func TestGetTemplate_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetTemplate("nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestListTemplates_All(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)
	createTestTemplate(t, s, "t-2", domain.CategoryImage)
	createTestTemplate(t, s, "t-3", domain.CategoryCaption)

	templates, err := s.ListTemplates("")
	require.NoError(t, err)
	assert.Len(t, templates, 3)
}

func TestListTemplates_FilterByCategory(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)
	createTestTemplate(t, s, "t-2", domain.CategoryImage)
	createTestTemplate(t, s, "t-3", domain.CategoryScenario)

	templates, err := s.ListTemplates("scenario")
	require.NoError(t, err)
	assert.Len(t, templates, 2)
}

func TestListTemplates_Empty(t *testing.T) {
	s := setupTestStore(t)
	templates, err := s.ListTemplates("")
	require.NoError(t, err)
	assert.Empty(t, templates)
}

func TestUpdateTemplate_IncrementsVersion(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)

	err := s.UpdateTemplate("t-1", "updated content", "t-1-v2")
	require.NoError(t, err)

	got, err := s.GetTemplate("t-1")
	require.NoError(t, err)
	assert.Equal(t, 2, got.Version)
	assert.Equal(t, "updated content", got.Content)
}

func TestUpdateTemplate_CreatesVersionRecord(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)
	require.NoError(t, s.UpdateTemplate("t-1", "v2 content", "t-1-v2"))

	versions, err := s.ListTemplateVersions("t-1")
	require.NoError(t, err)
	assert.Len(t, versions, 2)
	assert.Equal(t, 2, versions[0].Version) // DESC order
	assert.Equal(t, 1, versions[1].Version)
}

func TestUpdateTemplate_NotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.UpdateTemplate("nonexistent", "content", "v-id")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestUpdateTemplate_PrunesBeyond10Versions(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)

	// Create 11 more versions (total 12 including v1)
	for i := 2; i <= 12; i++ {
		vID := fmt.Sprintf("t-1-v%d", i)
		require.NoError(t, s.UpdateTemplate("t-1", fmt.Sprintf("content v%d", i), vID))
	}

	versions, err := s.ListTemplateVersions("t-1")
	require.NoError(t, err)
	assert.Len(t, versions, 10)
	assert.Equal(t, 12, versions[0].Version) // newest
	assert.Equal(t, 3, versions[9].Version)  // oldest kept (v1, v2 pruned)
}

func TestDeleteTemplate_Success(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)

	err := s.DeleteTemplate("t-1")
	require.NoError(t, err)

	_, err = s.GetTemplate("t-1")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteTemplate_RemovesVersions(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)
	require.NoError(t, s.UpdateTemplate("t-1", "v2", "t-1-v2"))

	require.NoError(t, s.DeleteTemplate("t-1"))

	versions, err := s.ListTemplateVersions("t-1")
	require.NoError(t, err)
	assert.Empty(t, versions)
}

func TestDeleteTemplate_RemovesOverrides(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)
	require.NoError(t, s.SetOverride("proj-1", "t-1", "override"))

	require.NoError(t, s.DeleteTemplate("t-1"))

	_, err := s.GetOverride("proj-1", "t-1")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteTemplate_NotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.DeleteTemplate("nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestGetTemplateVersion_Success(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)

	v, err := s.GetTemplateVersion("t-1", 1)
	require.NoError(t, err)
	assert.Equal(t, 1, v.Version)
	assert.Equal(t, "content for t-1", v.Content)
}

func TestGetTemplateVersion_NotFound(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)

	_, err := s.GetTemplateVersion("t-1", 99)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestRollbackTemplate_Success(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)
	require.NoError(t, s.UpdateTemplate("t-1", "v2 content", "t-1-v2"))
	require.NoError(t, s.UpdateTemplate("t-1", "v3 content", "t-1-v3"))

	// Rollback to v1
	err := s.RollbackTemplate("t-1", 1, "t-1-v4")
	require.NoError(t, err)

	got, err := s.GetTemplate("t-1")
	require.NoError(t, err)
	assert.Equal(t, 4, got.Version) // version increments
	assert.Equal(t, "content for t-1", got.Content)
}

func TestRollbackTemplate_VersionNotFound(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)

	err := s.RollbackTemplate("t-1", 99, "v-id")
	assert.Error(t, err)
}

func TestSetOverride_Success(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)

	err := s.SetOverride("proj-1", "t-1", "project override content")
	require.NoError(t, err)

	o, err := s.GetOverride("proj-1", "t-1")
	require.NoError(t, err)
	assert.Equal(t, "project override content", o.Content)
}

func TestSetOverride_Upsert(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)

	require.NoError(t, s.SetOverride("proj-1", "t-1", "first"))
	require.NoError(t, s.SetOverride("proj-1", "t-1", "second"))

	o, err := s.GetOverride("proj-1", "t-1")
	require.NoError(t, err)
	assert.Equal(t, "second", o.Content)
}

func TestGetOverride_NotFound(t *testing.T) {
	s := setupTestStore(t)
	_, err := s.GetOverride("proj-1", "t-1")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteOverride_Success(t *testing.T) {
	s := setupTestStore(t)
	createTestTemplate(t, s, "t-1", domain.CategoryScenario)
	require.NoError(t, s.SetOverride("proj-1", "t-1", "override"))

	err := s.DeleteOverride("proj-1", "t-1")
	require.NoError(t, err)

	_, err = s.GetOverride("proj-1", "t-1")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteOverride_NotFound(t *testing.T) {
	s := setupTestStore(t)
	err := s.DeleteOverride("proj-1", "t-1")
	assert.IsType(t, &domain.NotFoundError{}, err)
}
