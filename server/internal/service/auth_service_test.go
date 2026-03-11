package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAuthService_UsesSecretKeyFilePath(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")
	t.Setenv("LUMILIO_SECRET_KEY", keyFile)

	svc := NewAuthService(nil, nil)
	require.Len(t, svc.jwtSecret, 32)

	content, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.NotEmpty(t, content)
}

func TestNewAuthService_RejectsRawSecretText(t *testing.T) {
	t.Setenv("LUMILIO_SECRET_KEY", "raw-secret-value")

	require.PanicsWithValue(
		t,
		"failed to initialize JWT secret from LUMILIO_SECRET_KEY: LUMILIO_SECRET_KEY must be a key file path (absolute path, ./relative, or ../relative)",
		func() {
			NewAuthService(nil, nil)
		},
	)
}

func TestNewAuthService_DefaultSecretPathUsesStoragePath(t *testing.T) {
	storageRoot := filepath.Join(t.TempDir(), "storage-root")
	t.Setenv("LUMILIO_SECRET_KEY", "")
	t.Setenv("STORAGE_PATH", storageRoot)

	svc := NewAuthService(nil, nil)
	require.Len(t, svc.jwtSecret, 32)

	keyFile := filepath.Join(storageRoot, ".secrets", "lumilio_secret_key")
	_, err := os.Stat(keyFile)
	require.NoError(t, err)
}
