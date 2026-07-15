package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"server/config"

	"github.com/stretchr/testify/require"
)

func TestResolveLoginOptions_AccountShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		found        bool
		active       bool
		passkeyCount int64
		want         LoginOptions
	}{
		{
			name:   "unknown username",
			found:  false,
			active: false,
			want:   LoginOptions{Password: true, Passkey: false},
		},
		{
			name:   "inactive user",
			found:  true,
			active: false,
			want:   LoginOptions{Password: true, Passkey: false},
		},
		{
			name:         "active password-only",
			found:        true,
			active:       true,
			passkeyCount: 0,
			want:         LoginOptions{Password: true, Passkey: false},
		},
		{
			name:         "active with passkey",
			found:        true,
			active:       true,
			passkeyCount: 2,
			want:         LoginOptions{Password: true, Passkey: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := resolveLoginOptions(tc.found, tc.active, tc.passkeyCount)
			require.Equal(t, tc.want, got)

			payload, err := json.Marshal(got)
			require.NoError(t, err)
			require.NotContains(t, string(payload), "totp")
			require.JSONEq(t, mustJSON(tc.want), string(payload))
		})
	}
}

func TestGetLoginOptions_InvalidUsername(t *testing.T) {
	t.Parallel()

	keyFile := filepath.Join(t.TempDir(), "lumilio_secret_key")
	svc, err := NewAuthService(nil, nil, config.AuthConfig{SecretKeyFile: keyFile})
	require.NoError(t, err)

	_, err = svc.GetLoginOptions(context.Background(), "ab")
	require.ErrorIs(t, err, ErrInvalidUsernameFormat)

	_, err = svc.GetLoginOptions(context.Background(), "")
	require.ErrorIs(t, err, ErrInvalidUsernameFormat)

	_, err = os.Stat(keyFile)
	require.NoError(t, err)
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
