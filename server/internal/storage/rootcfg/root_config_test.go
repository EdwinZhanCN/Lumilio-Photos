package rootcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootConfigSaveAndLoad(t *testing.T) {
	root := t.TempDir()
	config := New("External Archive")

	require.NoError(t, config.Save(root))
	assert.True(t, Exists(root))

	loaded, err := Load(root)
	require.NoError(t, err)
	assert.Equal(t, config.Version, loaded.Version)
	assert.Equal(t, config.ID, loaded.ID)
	assert.Equal(t, config.Name, loaded.Name)
	assert.True(t, config.CreatedAt.Equal(loaded.CreatedAt))
	assert.FileExists(t, filepath.Join(root, FileName))
	_, err = uuid.Parse(loaded.ID)
	require.NoError(t, err)
}

func TestRootConfigRejectsInvalidMarker(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, FileName), []byte("version: '1.0'\n"), 0o644))

	_, err := Load(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id must be a UUID")
}

func TestRootConfigRequiresCurrentVersion(t *testing.T) {
	config := New("Archive")
	config.Version = "2.0"

	err := config.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version must be 1.0")
}
