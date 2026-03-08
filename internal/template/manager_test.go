package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_LoadAll(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "image_prompt.tmpl"),
		[]byte(`Scene {{.SceneNum}}: {{.VisualDescription}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "narration.tmpl"),
		[]byte(`Narrate: {{.Narration}}`), 0o644))
	// Non-template file should be ignored
	require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.txt"),
		[]byte(`not a template`), 0o644))

	mgr := NewManager(dir)
	err := mgr.LoadAll()
	require.NoError(t, err)

	templates := mgr.List()
	assert.Len(t, templates, 2)

	tmpl := mgr.Get("image_prompt")
	assert.NotNil(t, tmpl)

	info, ok := mgr.GetInfo("image_prompt")
	assert.True(t, ok)
	assert.Equal(t, "image_prompt", info.Name)
	assert.NotEmpty(t, info.Version)
	assert.NotEmpty(t, info.ModTime)
}

func TestManager_LoadAll_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	err := mgr.LoadAll()
	require.NoError(t, err)
	assert.Empty(t, mgr.List())
}

func TestManager_LoadAll_NoDir(t *testing.T) {
	mgr := NewManager("/nonexistent/dir")
	err := mgr.LoadAll()
	require.NoError(t, err) // non-existent dir is silently ignored
}

func TestManager_LoadAll_NoDirConfigured(t *testing.T) {
	mgr := NewManager("")
	err := mgr.LoadAll()
	require.NoError(t, err)
}

func TestManager_LoadAll_SyntaxError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.tmpl"),
		[]byte(`{{.Broken`), 0o644))

	mgr := NewManager(dir)
	err := mgr.LoadAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")
}

func TestManager_LoadAll_EmptyTemplate(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "empty.tmpl"),
		[]byte("   "), 0o644))

	mgr := NewManager(dir)
	err := mgr.LoadAll()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestManager_Load_Single(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "custom.tmpl"),
		[]byte(`Custom: {{.Name}}`), 0o644))

	mgr := NewManager(dir)
	err := mgr.Load("custom")
	require.NoError(t, err)
	assert.NotNil(t, mgr.Get("custom"))
}

func TestManager_Load_NotFound(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager(dir)
	err := mgr.Load("missing")
	require.Error(t, err)
}

func TestManager_Get_NotLoaded(t *testing.T) {
	mgr := NewManager("")
	assert.Nil(t, mgr.Get("nonexistent"))
}

func TestManager_GetVersion(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.tmpl"),
		[]byte(`Hello {{.World}}`), 0o644))

	mgr := NewManager(dir)
	require.NoError(t, mgr.Load("test"))

	v := mgr.GetVersion("test")
	assert.NotEmpty(t, v)
	assert.Equal(t, "", mgr.GetVersion("nonexistent"))
}

func TestManager_VersionChangesOnContentChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "versioned.tmpl")

	require.NoError(t, os.WriteFile(path, []byte(`v1 {{.X}}`), 0o644))
	mgr1 := NewManager(dir)
	require.NoError(t, mgr1.Load("versioned"))
	v1 := mgr1.GetVersion("versioned")

	require.NoError(t, os.WriteFile(path, []byte(`v2 {{.X}}`), 0o644))
	mgr2 := NewManager(dir)
	require.NoError(t, mgr2.Load("versioned"))
	v2 := mgr2.GetVersion("versioned")

	assert.NotEqual(t, v1, v2)
}

func TestHashContent(t *testing.T) {
	h1 := HashContent("hello")
	h2 := HashContent("hello")
	h3 := HashContent("world")
	assert.Equal(t, h1, h2)
	assert.NotEqual(t, h1, h3)
	assert.Len(t, h1, 16) // 8 bytes hex
}

func TestValidateFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "valid.tmpl")
	require.NoError(t, os.WriteFile(path, []byte(`{{.Foo}} bar`), 0o644))
	assert.NoError(t, ValidateFile(path))
}

func TestValidateFile_SyntaxError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.tmpl")
	require.NoError(t, os.WriteFile(path, []byte(`{{.Unclosed`), 0o644))
	err := ValidateFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error")
}

func TestValidateFile_NotFound(t *testing.T) {
	err := ValidateFile("/nonexistent/file.tmpl")
	require.Error(t, err)
}

func TestValidateFile_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.tmpl")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o644))
	err := ValidateFile(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestManager_Execute(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "greet.tmpl"),
		[]byte(`Hello {{.Name}}, you are {{.Age}}`), 0o644))

	mgr := NewManager(dir)
	require.NoError(t, mgr.Load("greet"))

	tmpl := mgr.Get("greet")
	require.NotNil(t, tmpl)

	var buf strings.Builder
	err := tmpl.Execute(&buf, map[string]any{"Name": "Alice", "Age": 30})
	require.NoError(t, err)
	assert.Equal(t, "Hello Alice, you are 30", buf.String())
}
