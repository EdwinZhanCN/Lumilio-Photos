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
	assert.Contains(t, err.Error(), "newer Lumilio Photos")
}

// TestRootConfigReadsPreReleaseMarker proves a marker written by a pre-release
// build (same shape as the rc.1 baseline) is read without rewriting the file.
func TestRootConfigReadsPreReleaseMarker(t *testing.T) {
	root := t.TempDir()
	marker := []byte("version: \"1.0\"\nid: 7f2b0c1e-4a52-4d2e-9a8f-3c7d1e5b9a10\nname: Archive\ncreated_at: 2026-08-01T10:00:00Z\n")
	path := filepath.Join(root, FileName)
	require.NoError(t, os.WriteFile(path, marker, 0o644))

	config, err := Load(root)
	require.NoError(t, err)
	assert.Equal(t, "7f2b0c1e-4a52-4d2e-9a8f-3c7d1e5b9a10", config.ID)
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, marker, after)
}
