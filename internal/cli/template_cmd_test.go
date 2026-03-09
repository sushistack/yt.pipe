package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sushistack/yt.pipe/internal/domain"
	"github.com/sushistack/yt.pipe/internal/service"
	"github.com/sushistack/yt.pipe/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTemplateServiceForCLI(t *testing.T) *service.TemplateService {
	t.Helper()
	db, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	svc := service.NewTemplateService(db)
	_, err = svc.InstallDefaults()
	require.NoError(t, err)
	return svc
}

func TestTemplateCLI_ListAll(t *testing.T) {
	svc := setupTemplateServiceForCLI(t)

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

func TestTemplateCLI_ListByCategory(t *testing.T) {
	svc := setupTemplateServiceForCLI(t)

	templates, err := svc.ListTemplates("scenario")
	require.NoError(t, err)
	assert.Len(t, templates, 1)
	assert.Equal(t, domain.CategoryScenario, templates[0].Category)
}

func TestTemplateCLI_CreateAndShow(t *testing.T) {
	svc := setupTemplateServiceForCLI(t)

	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Custom Scenario", "Custom prompt content", false)
	require.NoError(t, err)
	assert.NotEmpty(t, tmpl.ID)
	assert.Equal(t, 1, tmpl.Version)

	got, err := svc.GetTemplate(tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "Custom prompt content", got.Content)
	assert.Equal(t, "Custom Scenario", got.Name)
}

func TestTemplateCLI_UpdateAndRollback(t *testing.T) {
	svc := setupTemplateServiceForCLI(t)

	tmpl, err := svc.CreateTemplate(domain.CategoryImage, "Test", "v1 content", false)
	require.NoError(t, err)

	updated, err := svc.UpdateTemplate(tmpl.ID, "v2 content")
	require.NoError(t, err)
	assert.Equal(t, 2, updated.Version)
	assert.Equal(t, "v2 content", updated.Content)

	rolled, err := svc.RollbackTemplate(tmpl.ID, 1)
	require.NoError(t, err)
	assert.Equal(t, 3, rolled.Version)
	assert.Equal(t, "v1 content", rolled.Content)
}

func TestTemplateCLI_DeleteProtectsDefault(t *testing.T) {
	svc := setupTemplateServiceForCLI(t)

	templates, err := svc.ListTemplates("")
	require.NoError(t, err)

	err = svc.DeleteTemplate(templates[0].ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot delete default template")
}

func TestTemplateCLI_DeleteNonDefault(t *testing.T) {
	svc := setupTemplateServiceForCLI(t)

	tmpl, err := svc.CreateTemplate(domain.CategoryCaption, "Deletable", "content", false)
	require.NoError(t, err)

	err = svc.DeleteTemplate(tmpl.ID)
	require.NoError(t, err)

	_, err = svc.GetTemplate(tmpl.ID)
	assert.IsType(t, &domain.NotFoundError{}, err)
}

func TestTemplateCLI_Override(t *testing.T) {
	svc := setupTemplateServiceForCLI(t)

	tmpl, err := svc.CreateTemplate(domain.CategoryScenario, "Test", "global", false)
	require.NoError(t, err)

	// Set override
	err = svc.SetOverride("proj-1", tmpl.ID, "project override")
	require.NoError(t, err)

	// Resolve returns override
	content, err := svc.ResolveTemplate("proj-1", tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "project override", content)

	// Delete override
	err = svc.DeleteOverride("proj-1", tmpl.ID)
	require.NoError(t, err)

	// Resolve returns global
	content, err = svc.ResolveTemplate("proj-1", tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, "global", content)
}

func TestTemplateCategoryFromString(t *testing.T) {
	assert.Equal(t, domain.CategoryScenario, templateCategoryFromString("scenario"))
	assert.Equal(t, domain.CategoryImage, templateCategoryFromString("image"))
	assert.Equal(t, domain.CategoryTTS, templateCategoryFromString("tts"))
	assert.Equal(t, domain.CategoryCaption, templateCategoryFromString("caption"))
}

func TestInstallDefaultTemplates(t *testing.T) {
	tmpDir := t.TempDir()
	installed, err := installDefaultTemplates(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, 4, installed)

	// Second call should be idempotent
	installed, err = installDefaultTemplates(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, 0, installed)

	// Verify DB exists
	dbPath := filepath.Join(tmpDir, "yt-pipe.db")
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}
