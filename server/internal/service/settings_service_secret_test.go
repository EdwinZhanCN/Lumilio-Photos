package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"server/internal/secretbox"
	"server/platform/fsprivacy"

	"github.com/stretchr/testify/require"
)

func TestLoadOrCreateLumilioSecretKey_AutoGenerateAndReuse(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")

	first, err := secretbox.LoadOrCreateLumilioSecretKey(keyFile)
	require.NoError(t, err)
	require.Len(t, first, 64)

	second, err := secretbox.LoadOrCreateLumilioSecretKey(keyFile)
	require.NoError(t, err)
	require.Equal(t, first, second)

	content, err := os.ReadFile(keyFile)
	require.NoError(t, err)
	require.Equal(t, first, string(bytes.TrimSpace(content)))

	private, err := fsprivacy.IsPrivate(keyFile)
	require.NoError(t, err)
	require.True(t, private)
}

func TestLoadOrCreateLumilioSecretKey_RejectsRawText(t *testing.T) {
	_, err := secretbox.LoadOrCreateLumilioSecretKey("my-raw-secret-value")
	require.ErrorContains(t, err, "must be absolute")
}

func TestSettingsService_LoadsSecretFromInjectedPath(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")

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

func TestSecretBox_ScopeIsolation(t *testing.T) {
	root := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	left := secretbox.NewFromRoot(root, "left.scope")
	right := secretbox.NewFromRoot(root, "right.scope")

	ciphertext, err := left.Encrypt("secret value")
	require.NoError(t, err)

	plaintext, err := left.Decrypt(ciphertext)
	require.NoError(t, err)
	require.Equal(t, "secret value", plaintext)

	_, err = right.Decrypt(ciphertext)
	require.Error(t, err)
}
