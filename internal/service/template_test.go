package service

import (
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTemplateService(t *testing.T) *TemplateService {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return NewTemplateService(s)
}

func TestCreateTemplate_Success(t *testing.T) {
	svc := setupTemplateService(t)
	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "content", false)
	require.NoError(t, err)
	assert.NotEmpty(t, tmpl.ID)
	assert.Equal(t, domain.CategoryScenario, tmpl.Category)
	assert.Equal(t, 1, tmpl.Version)
}

func TestCreateTemplate_InvalidCategory(t *testing.T) {
	svc := setupTemplateService(t)
	_, err := svc.CreateTemplate("invalid", "Test", "content", false)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestCreateTemplate_EmptyName(t *testing.T) {
	svc := setupTemplateService(t)
	_, err := svc.CreateTemplate(domain.CategoryScenario, "", "content", false)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestCreateTemplate_EmptyContent(t *testing.T) {
	svc := setupTemplateService(t)
	_, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "", false)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestListTemplates_InvalidCategory(t *testing.T) {
	svc := setupTemplateService(t)
	_, err := svc.ListTemplates("invalid")
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestListTemplates_ValidCategory(t *testing.T) {
	svc := setupTemplateService(t)
	_, err := svc.CreateTemplate(domain.CategoryScenario, "S1", "c1", false)
	require.NoError(t, err)
	_, err = svc.CreateTemplate(domain.CategoryImage, "I1", "c2", false)
	require.NoError(t, err)

	templates, err := svc.ListTemplates("scenario")
	require.NoError(t, err)
	assert.Len(t, templates, 1)
}

func TestUpdateTemplate_Success(t *testing.T) {
	svc := setupTemplateService(t)
	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "v1", false)
	require.NoError(t, err)

	updated, err := svc.UpdateTemplate(tmpl.ID, "v2")
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)
	assert.Equal(t, "v2", updated.Content)
}

func TestUpdateTemplate_EmptyContent(t *testing.T) {
	svc := setupTemplateService(t)
	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "v1", false)
	require.NoError(t, err)

	_, err = svc.UpdateTemplate(tmpl.ID, "")
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestUpdateTemplate_NotFound(t *testing.T) {
	svc := setupTemplateService(t)
	_, err := svc.UpdateTemplate("nonexistent", "content")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestRollbackTemplate_Success(t *testing.T) {
	svc := setupTemplateService(t)
	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "v1 content", false)
	require.NoError(t, err)

	_, err = svc.UpdateTemplate(tmpl.ID, "v2 content")
	require.NoError(t, err)

	rolled, err := svc.RollbackTemplate(tmpl.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, 3, rolled.Version)
	assert.Equal(t, "v1 content", rolled.Content)
}

func TestRollbackTemplate_VersionNotFound(t *testing.T) {
	svc := setupTemplateService(t)
	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "v1", false)
	require.NoError(t, err)

	_, err = svc.RollbackTemplate(tmpl.ID, 99)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteTemplate_Success(t *testing.T) {
	svc := setupTemplateService(t)
	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "content", false)
	require.NoError(t, err)

	err = svc.DeleteTemplate(tmpl.ID)
	require.NoError(t, err)

	_, err = svc.GetTemplate(tmpl.ID)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestDeleteTemplate_ProtectsDefault(t *testing.T) {
	svc := setupTemplateService(t)
	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Default", "content", true)
	require.NoError(t, err)

	err = svc.DeleteTemplate(tmpl.ID)
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestDeleteTemplate_NotFound(t *testing.T) {
	svc := setupTemplateService(t)
	err := svc.DeleteTemplate("nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestResolveTemplate_ReturnsOverride(t *testing.T) {
	svc := setupTemplateService(t)
	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "global content", false)
	require.NoError(t, err)
	require.NoError(t, svc.SetOverride("proj-1", tmpl.ID, "project content"))

	content, err := svc.ResolveTemplate("proj-1", tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "project content", content)
}

func TestResolveTemplate_FallsBackToGlobal(t *testing.T) {
	svc := setupTemplateService(t)
	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "global content", false)
	require.NoError(t, err)

	content, err := svc.ResolveTemplate("proj-1", tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "global content", content)
}

func TestResolveTemplate_TemplateNotFound(t *testing.T) {
	svc := setupTemplateService(t)
	_, err := svc.ResolveTemplate("proj-1", "nonexistent")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestSetOverride_EmptyContent(t *testing.T) {
	svc := setupTemplateService(t)
	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "content", false)
	require.NoError(t, err)

	err = svc.SetOverride("proj-1", tmpl.ID, "")
	assert.IsType(t, &domain.ValidationError{}, err)
}

func TestSetOverride_TemplateNotFound(t *testing.T) {
	svc := setupTemplateService(t)
	err := svc.SetOverride("proj-1", "nonexistent", "content")
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestInstallDefaults_Success(t *testing.T) {
	svc := setupTemplateService(t)
	count, err := svc.InstallDefaults()
	require.NoError(t, err)
	assert.Equal(t, 4, count)

	// Verify all 4 categories have default templates
	templates, err := svc.ListTemplates("")
	require.NoError(t, err)
	assert.Len(t, templates, 4)

	categories := make(map[domain.TemplateCategory]bool)
	for _, tmpl := range templates {
		assert.True(t, tmpl.IsDefault)
		categories[tmpl.Category] = true
	}
	assert.True(t, categories[domain.CategoryScenario])
	assert.True(t, categories[domain.CategoryImage])
	assert.True(t, categories[domain.CategoryTTS])
	assert.True(t, categories[domain.CategoryCaption])
}

func TestInstallDefaults_Idempotent(t *testing.T) {
	svc := setupTemplateService(t)

	count1, err := svc.InstallDefaults()
	require.NoError(t, err)
	assert.Equal(t, 4, count1)

	// Second call should skip
	count2, err := svc.InstallDefaults()
	require.NoError(t, err)
	assert.Equal(t, 0, count2)

	// Still only 4 templates
	templates, err := svc.ListTemplates("")
	require.NoError(t, err)
	assert.Len(t, templates, 4)
}
