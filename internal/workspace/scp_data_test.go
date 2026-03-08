package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jay/youtube-pipeline/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSCPData(t *testing.T, scpID string) string {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, scpID)
	require.NoError(t, os.MkdirAll(dir, 0o755))

	facts := `{"schema_version":"1.0","facts":{"containment":"Euclid","origin":"Site-19"}}`
	meta := `{"schema_version":"1.0","title":"The Sculpture","object_class":"Euclid","series":"I","url":"https://scp-wiki.net/scp-173"}`
	mainText := "SCP-173 is a concrete sculpture..."

	require.NoError(t, os.WriteFile(filepath.Join(dir, "facts.json"), []byte(facts), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "meta.json"), []byte(meta), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.txt"), []byte(mainText), 0o644))

	return base
}

func TestLoadSCPData_Success(t *testing.T) {
	base := setupSCPData(t, "SCP-173")

	data, err := LoadSCPData(base, "SCP-173")
	require.NoError(t, err)

	assert.Equal(t, "SCP-173", data.SCPID)
	assert.Equal(t, "1.0", data.Facts.SchemaVersion)
	assert.Equal(t, "Euclid", data.Facts.Facts["containment"])
	assert.Equal(t, "Site-19", data.Facts.Facts["origin"])
	assert.Equal(t, "The Sculpture", data.Meta.Title)
	assert.Equal(t, "Euclid", data.Meta.ObjectClass)
	assert.Contains(t, data.MainText, "SCP-173")
}

func TestLoadSCPData_NotFound(t *testing.T) {
	base := t.TempDir()

	_, err := LoadSCPData(base, "SCP-999")
	require.Error(t, err)
	var nfe *domain.NotFoundError
	assert.ErrorAs(t, err, &nfe)
}

func TestLoadSCPData_MissingFactsFile(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "SCP-173")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "meta.json"), []byte(`{"schema_version":"1.0"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.txt"), []byte("text"), 0o644))

	_, err := LoadSCPData(base, "SCP-173")
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Equal(t, "facts.json", ve.Field)
}

func TestLoadSCPData_SchemaVersionMismatch(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "SCP-173")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	facts := `{"schema_version":"2.0","facts":{}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "facts.json"), []byte(facts), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "meta.json"), []byte(`{"schema_version":"1.0"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.txt"), []byte("text"), 0o644))

	_, err := LoadSCPData(base, "SCP-173")
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, "schema version mismatch")
}

func TestLoadSCPData_InvalidJSON(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "SCP-173")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "facts.json"), []byte("{bad json"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "meta.json"), []byte(`{"schema_version":"1.0"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.txt"), []byte("text"), 0o644))

	_, err := LoadSCPData(base, "SCP-173")
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Contains(t, ve.Message, "invalid JSON")
}

func TestLoadSCPData_MetaSchemaVersionMismatch(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "SCP-173")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "facts.json"), []byte(`{"schema_version":"1.0","facts":{}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "meta.json"), []byte(`{"schema_version":"3.0"}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.txt"), []byte("text"), 0o644))

	_, err := LoadSCPData(base, "SCP-173")
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Equal(t, "meta.json", ve.Field)
}

func TestLoadSCPData_MissingMainText(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "SCP-173")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "facts.json"), []byte(`{"schema_version":"1.0","facts":{}}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "meta.json"), []byte(`{"schema_version":"1.0"}`), 0o644))

	_, err := LoadSCPData(base, "SCP-173")
	require.Error(t, err)
	var ve *domain.ValidationError
	assert.ErrorAs(t, err, &ve)
	assert.Equal(t, "main.txt", ve.Field)
}
