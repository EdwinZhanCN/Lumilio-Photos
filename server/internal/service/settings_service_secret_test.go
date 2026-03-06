package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadOrCreateLumilioSecretKey_AutoGenerateAndReuse(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")

	first, err := loadOrCreateLumilioSecretKey(keyFile)
	require.NoError(t, err)
	require.Len(t, first, 64)

	second, err := loadOrCreateLumilioSecretKey(keyFile)
	require.NoError(t, err)
	require.Equal(t, first, second)

	content, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.Equal(t, first, string(bytes.TrimSpace(content)))

	stat, err := os.Stat(keyFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
}

func TestLoadOrCreateLumilioSecretKey_RejectsRawText(t *testing.T) {
	_, err := loadOrCreateLumilioSecretKey("my-raw-secret-value")
	require.ErrorContains(t, err, "must be a key file path")
}

func TestSettingsService_LoadsSecretFromPathEnv(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")
	t.Setenv("LUMILIO_SECRET_KEY", keyFile)

	svc := &settingsService{
		secretPath: keyFile,
	}

	key, err := svc.encryptionKey()
	require.NoError(t, err)
	require.Len(t, key, 32)

	content, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.NotEmpty(t, bytes.TrimSpace(content))
}
