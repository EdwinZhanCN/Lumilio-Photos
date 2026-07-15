package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db/repo"

	"github.com/golang-jwt/jwt/v5"
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

func TestRequiredPasswordChangeTokenIsPurposeBoundAndCarriesAuthVersion(t *testing.T) {
	svc, err := NewAuthService(nil, nil, config.AuthConfig{SecretKeyFile: filepath.Join(t.TempDir(), "secret")})
	require.NoError(t, err)

	response, err := svc.issueRequiredPasswordChange(repo.User{
		UserID:      42,
		Username:    "admin",
		AuthVersion: 7,
	})
	require.NoError(t, err)
	require.True(t, response.RequiresPasswordChange)
	require.Empty(t, response.AccessToken)
	require.Empty(t, response.RefreshToken)
	require.NotEmpty(t, response.PasswordChangeToken)

	claims, err := svc.parsePasswordChangeToken(response.PasswordChangeToken)
	require.NoError(t, err)
	require.Equal(t, 42, claims.UserID)
	require.Equal(t, int64(7), claims.AuthVersion)
	require.Equal(t, passwordChangeTokenPurpose, claims.Purpose)
	require.WithinDuration(t, time.Now().Add(passwordChangeTokenTTL), claims.ExpiresAt.Time, 2*time.Second)
}

func TestRequiredPasswordChangeTokenRejectsWrongPurposeAndSigningMethod(t *testing.T) {
	svc, err := NewAuthService(nil, nil, config.AuthConfig{SecretKeyFile: filepath.Join(t.TempDir(), "secret")})
	require.NoError(t, err)

	now := time.Now()
	wrongPurpose := passwordChangeClaims{
		UserID: 1, AuthVersion: 2, Purpose: "access",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, wrongPurpose).SignedString(svc.passwordChangeTokenSecret)
	require.NoError(t, err)
	_, err = svc.parsePasswordChangeToken(token)
	require.ErrorIs(t, err, ErrInvalidPasswordChangeToken)

	noneToken := jwt.NewWithClaims(jwt.SigningMethodNone, passwordChangeClaims{
		UserID: 1, AuthVersion: 2, Purpose: passwordChangeTokenPurpose,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute))},
	})
	token, err = noneToken.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	_, err = svc.parsePasswordChangeToken(token)
	require.ErrorIs(t, err, ErrInvalidPasswordChangeToken)

	expired := passwordChangeClaims{
		UserID: 1, AuthVersion: 2, Purpose: passwordChangeTokenPurpose,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(-time.Minute))},
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, expired).SignedString(svc.passwordChangeTokenSecret)
	require.NoError(t, err)
	_, err = svc.parsePasswordChangeToken(token)
	require.ErrorIs(t, err, ErrInvalidPasswordChangeToken)
}
