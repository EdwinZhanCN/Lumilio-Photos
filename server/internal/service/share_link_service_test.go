package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewShareLinkService_ReturnsSecretKeyError(t *testing.T) {
	cases := map[string]string{
		"relative path": "raw-secret-value",
		"empty path":    "",
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			var svc *shareLinkService
			var err error
			require.NotPanics(t, func() {
				svc, err = NewShareLinkService(nil, nil, nil, path)
			})
			require.Error(t, err)
			require.Nil(t, svc)
			require.ErrorContains(t, err, "initialize share link root secret key")
		})
	}
}

func TestNewShareLinkService_ReturnsEmptySecretFileError(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")
	require.NoError(t, os.WriteFile(keyFile, []byte("  \n"), 0o600))

	svc, err := NewShareLinkService(nil, nil, nil, keyFile)
	require.Nil(t, svc)
	require.ErrorContains(t, err, "secret key file is empty")
}

func TestNewShareLinkService_DerivesScopedKeyFromSecretFile(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")

	svc, err := NewShareLinkService(nil, nil, nil, keyFile)
	require.NoError(t, err)
	require.Len(t, svc.hmacKey, 32)

	// The same root secret must derive the same hashing key, or every
	// existing share token would stop resolving after a restart.
	again, err := NewShareLinkService(nil, nil, nil, keyFile)
	require.NoError(t, err)
	require.Equal(t, svc.hmacKey, again.hmacKey)
	require.Equal(t, svc.hashToken("token"), again.hashToken("token"))
}
