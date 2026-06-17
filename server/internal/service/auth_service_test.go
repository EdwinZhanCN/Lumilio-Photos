package service

import (
	"os"
	"path/filepath"
	"testing"

	"server/config"

	"github.com/stretchr/testify/require"
)

func TestNewAuthService_UsesSecretKeyFilePath(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")

	svc := NewAuthService(nil, nil, config.AuthConfig{SecretKeyPath: keyFile})
	require.Len(t, svc.jwtSecret, 32)

	content, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.NotEmpty(t, content)
}

func TestNewAuthService_RejectsRawSecretText(t *testing.T) {
	require.PanicsWithValue(
		t,
		"failed to initialize root secret key: LUMILIO_SECRET_KEY must be a key file path (absolute path, ./relative, or ../relative)",
		func() {
			NewAuthService(nil, nil, config.AuthConfig{SecretKeyPath: "raw-secret-value"})
		},
	)
}

func TestNewAuthService_UsesConfiguredDerivedSecretPath(t *testing.T) {
	storageRoot := filepath.Join(t.TempDir(), "storage-root")
	keyFile := filepath.Join(storageRoot, ".secrets", "lumilio_secret_key")

	svc := NewAuthService(nil, nil, config.AuthConfig{SecretKeyPath: keyFile})
	require.Len(t, svc.jwtSecret, 32)

	_, err := os.Stat(keyFile)
	require.NoError(t, err)
}
