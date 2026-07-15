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

	svc, err := NewAuthService(nil, nil, config.AuthConfig{SecretKeyFile: keyFile})
	require.NoError(t, err)
	require.Len(t, svc.jwtSecret, 32)

	content, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.NotEmpty(t, content)
}

func TestNewAuthService_RejectsRawSecretText(t *testing.T) {
	_, err := NewAuthService(nil, nil, config.AuthConfig{SecretKeyFile: "raw-secret-value"})
	require.EqualError(t, err, "initialize root secret key: secret key file path must be absolute")
}

func TestNewAuthService_UsesConfiguredDerivedSecretPath(t *testing.T) {
	storageRoot := filepath.Join(t.TempDir(), "storage-root")
	keyFile := filepath.Join(storageRoot, ".secrets", "lumilio_secret_key")

	svc, err := NewAuthService(nil, nil, config.AuthConfig{SecretKeyFile: keyFile})
	require.NoError(t, err)
	require.Len(t, svc.jwtSecret, 32)

	_, err = os.Stat(keyFile)
	require.NoError(t, err)
}
