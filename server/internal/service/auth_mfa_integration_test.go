package service

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/repo"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func newMFAIntegrationService(t *testing.T) (context.Context, *db.DB, *AuthService, repo.User, string, string) {
	t.Helper()
	ctx := context.Background()
	catalog, err := db.Open(ctx, config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "app-state", "library.sqlite3")})
	require.NoError(t, err)
	require.NoError(t, catalog.Migrate(ctx))
	t.Cleanup(func() { _ = catalog.Close(context.Background()) })

	password := "CorrectHorseBatteryStaple123"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	require.NoError(t, err)
	user, err := catalog.Queries.CreateUser(ctx, repo.CreateUserParams{
		Username:           "mfa-integration",
		Password:           string(passwordHash),
		DisplayName:        "MFA Integration",
		Role:               string(UserRoleAdmin),
		WebauthnUserHandle: []byte("mfa-integration-handle"),
	})
	require.NoError(t, err)

	secretPath := filepath.Join(t.TempDir(), "secret")
	auth, err := NewAuthService(catalog.Queries, catalog.SQL, config.AuthConfig{
		SecretKeyFile:   secretPath,
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		MediaTokenTTL:   time.Hour,
	})
	require.NoError(t, err)
	return ctx, catalog, auth, user, password, secretPath
}

func TestTOTPEnrollmentAuthoritativelyEnablesLoginMFA(t *testing.T) {
	ctx, catalog, auth, user, password, _ := newMFAIntegrationService(t)
	oldSession, err := auth.generateAuthResponse(user)
	require.NoError(t, err)
	oldMediaToken, _, err := auth.GenerateMediaToken(ctx, int(user.UserID))
	require.NoError(t, err)

	security, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
		CurrentPassword: password,
		Purpose:         securityPurposeTOTPSetup,
	})
	require.NoError(t, err)
	setup, err := auth.BeginTOTPSetup(ctx, int(user.UserID), security.SecurityToken)
	require.NoError(t, err)
	_, err = auth.BeginTOTPSetup(ctx, int(user.UserID), security.SecurityToken)
	require.ErrorIs(t, err, ErrInvalidSecurityProof)
	statusBeforeEnable, err := auth.GetMFAStatus(ctx, int(user.UserID))
	require.NoError(t, err)
	require.False(t, statusBeforeEnable.TOTPEnabled)
	code, err := generateTOTPCode(setup.Secret, time.Now())
	require.NoError(t, err)
	enabled, err := auth.EnableTOTP(ctx, int(user.UserID), EnableTOTPInput{
		SetupToken: setup.SetupToken,
		Code:       code,
	})
	require.NoError(t, err)
	require.True(t, enabled.Status.TOTPEnabled)
	require.Len(t, enabled.RecoveryCodes, recoveryCodeCount)
	require.NotNil(t, enabled.Status.RecoveryCodesGeneratedAt)
	require.NotNil(t, enabled.Session)
	_, err = auth.AuthenticateAccessToken(ctx, oldSession.AccessToken)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = auth.ValidateMediaToken(ctx, oldMediaToken)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = auth.RefreshToken(oldSession.RefreshToken)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = auth.EnableTOTP(ctx, int(user.UserID), EnableTOTPInput{
		SetupToken: setup.SetupToken,
		Code:       code,
	})
	require.ErrorIs(t, err, ErrExpiredMFAToken)

	status, err := auth.GetMFAStatus(ctx, int(user.UserID))
	require.NoError(t, err)
	require.True(t, status.TOTPEnabled)
	require.Equal(t, recoveryCodeCount, status.RecoveryCodesRemaining)
	require.NotNil(t, status.RecoveryCodesGeneratedAt)

	login, err := auth.Login(LoginRequest{Username: user.Username, Password: password})
	require.NoError(t, err)
	require.True(t, login.RequiresMFA)
	require.NotEmpty(t, login.MFAToken)
	require.Empty(t, login.AccessToken)
	require.Empty(t, login.RefreshToken)
	_, err = auth.VerifyLoginMFA(ctx, VerifyMFARequest{
		MFAToken: login.MFAToken,
		Method:   "unknown",
		Code:     enabled.RecoveryCodes[0],
	})
	require.ErrorIs(t, err, ErrInvalidMFACode)
	_, err = auth.VerifyLoginMFA(ctx, VerifyMFARequest{
		MFAToken: login.MFAToken,
		Method:   MFAMethodTOTP,
		Code:     code,
	})
	require.ErrorIs(t, err, ErrInvalidMFACode)

	completed, err := auth.VerifyLoginMFA(ctx, VerifyMFARequest{
		MFAToken: login.MFAToken,
		Method:   MFAMethodRecoveryCode,
		Code:     enabled.RecoveryCodes[0],
	})
	require.NoError(t, err)
	require.False(t, completed.RequiresMFA)
	require.NotEmpty(t, completed.AccessToken)
	require.NotEmpty(t, completed.RefreshToken)

	// Reopen the catalog through a new DB handle to prove the typed status is
	// durable rather than a process-local enrollment result.
	path := catalog.Path
	require.NoError(t, catalog.Close(ctx))
	reopened, err := db.Open(ctx, config.DatabaseConfig{Path: path})
	require.NoError(t, err)
	require.NoError(t, reopened.Migrate(ctx))
	t.Cleanup(func() { require.NoError(t, reopened.Close(context.Background())) })
	reopenedAuth, err := NewAuthService(reopened.Queries, reopened.SQL, config.AuthConfig{
		SecretKeyFile: filepath.Join(t.TempDir(), "secret-reopened"),
	})
	require.NoError(t, err)
	reopenedStatus, err := reopenedAuth.GetMFAStatus(ctx, int(user.UserID))
	require.NoError(t, err)
	require.True(t, reopenedStatus.TOTPEnabled)
}

func TestDisableTOTPInvalidatesEveryPriorSession(t *testing.T) {
	ctx, _, auth, user, password, _ := newMFAIntegrationService(t)
	enabled := enableIntegrationTOTP(t, ctx, auth, user, password)

	currentUser, err := auth.queries.GetUserByID(ctx, user.UserID)
	require.NoError(t, err)
	oldSession, err := auth.generateAuthResponseWithAssurance(currentUser, "mfa")
	require.NoError(t, err)
	oldMediaToken, _, err := auth.GenerateMediaToken(ctx, int(user.UserID))
	require.NoError(t, err)

	security, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
		CurrentPassword: password,
		Code:            enabled.RecoveryCodes[0],
		Method:          MFAMethodRecoveryCode,
		Purpose:         securityPurposeTOTPDisable,
	})
	require.NoError(t, err)
	disabled, err := auth.DisableTOTP(ctx, int(user.UserID), security.SecurityToken)
	require.NoError(t, err)
	require.False(t, disabled.Status.TOTPEnabled)
	require.NotNil(t, disabled.Session)

	_, err = auth.AuthenticateAccessToken(ctx, oldSession.AccessToken)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = auth.ValidateMediaToken(ctx, oldMediaToken)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = auth.RefreshToken(oldSession.RefreshToken)
	require.ErrorIs(t, err, ErrInvalidToken)

	passwordLogin, err := auth.Login(LoginRequest{Username: user.Username, Password: password})
	require.NoError(t, err)
	require.False(t, passwordLogin.RequiresMFA)
}

func TestRecoveryRegenerationInvalidatesEveryPriorSession(t *testing.T) {
	ctx, _, auth, user, password, _ := newMFAIntegrationService(t)
	enabled := enableIntegrationTOTP(t, ctx, auth, user, password)

	currentUser, err := auth.queries.GetUserByID(ctx, user.UserID)
	require.NoError(t, err)
	oldSession, err := auth.generateAuthResponseWithAssurance(currentUser, "mfa")
	require.NoError(t, err)
	oldMediaToken, _, err := auth.GenerateMediaToken(ctx, int(user.UserID))
	require.NoError(t, err)

	security, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
		CurrentPassword: password,
		Code:            enabled.RecoveryCodes[0],
		Method:          MFAMethodRecoveryCode,
		Purpose:         securityPurposeRecoveryRegenerate,
	})
	require.NoError(t, err)
	regenerated, err := auth.RegenerateRecoveryCodes(ctx, int(user.UserID), security.SecurityToken)
	require.NoError(t, err)
	require.True(t, regenerated.Status.TOTPEnabled)
	require.Len(t, regenerated.RecoveryCodes, recoveryCodeCount)
	require.NotNil(t, regenerated.Session)

	_, err = auth.AuthenticateAccessToken(ctx, oldSession.AccessToken)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = auth.ValidateMediaToken(ctx, oldMediaToken)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = auth.RefreshToken(oldSession.RefreshToken)
	require.ErrorIs(t, err, ErrInvalidToken)
}

func enableIntegrationTOTP(t *testing.T, ctx context.Context, auth *AuthService, user repo.User, password string) RecoveryCodesResponse {
	t.Helper()
	security, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
		CurrentPassword: password,
		Purpose:         securityPurposeTOTPSetup,
	})
	require.NoError(t, err)
	setup, err := auth.BeginTOTPSetup(ctx, int(user.UserID), security.SecurityToken)
	require.NoError(t, err)
	code, err := generateTOTPCode(setup.Secret, time.Now())
	require.NoError(t, err)
	enabled, err := auth.EnableTOTP(ctx, int(user.UserID), EnableTOTPInput{
		SetupToken: setup.SetupToken,
		Code:       code,
	})
	require.NoError(t, err)
	return enabled
}

func TestPendingTOTPEnrollmentSurvivesRestartAndExpiryIsNonDestructive(t *testing.T) {
	t.Run("restart", func(t *testing.T) {
		ctx, catalog, auth, user, password, secretPath := newMFAIntegrationService(t)
		security, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
			CurrentPassword: password,
			Purpose:         securityPurposeTOTPSetup,
		})
		require.NoError(t, err)
		setup, err := auth.BeginTOTPSetup(ctx, int(user.UserID), security.SecurityToken)
		require.NoError(t, err)
		status, err := auth.GetMFAStatus(ctx, int(user.UserID))
		require.NoError(t, err)
		require.False(t, status.TOTPEnabled)

		path := catalog.Path
		require.NoError(t, catalog.Close(ctx))
		reopened, err := db.Open(ctx, config.DatabaseConfig{Path: path})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, reopened.Close(context.Background())) })
		reopenedAuth, err := NewAuthService(reopened.Queries, reopened.SQL, config.AuthConfig{
			SecretKeyFile: secretPath,
		})
		require.NoError(t, err)
		code, err := generateTOTPCode(setup.Secret, time.Now())
		require.NoError(t, err)
		enabled, err := reopenedAuth.EnableTOTP(ctx, int(user.UserID), EnableTOTPInput{
			SetupToken: setup.SetupToken,
			Code:       code,
		})
		require.NoError(t, err)
		require.True(t, enabled.Status.TOTPEnabled)
	})

	t.Run("expiry", func(t *testing.T) {
		ctx, catalog, auth, user, password, _ := newMFAIntegrationService(t)
		security, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
			CurrentPassword: password,
			Purpose:         securityPurposeTOTPSetup,
		})
		require.NoError(t, err)
		setup, err := auth.BeginTOTPSetup(ctx, int(user.UserID), security.SecurityToken)
		require.NoError(t, err)
		_, err = catalog.SQL.ExecContext(ctx, `
			UPDATE pending_totp_enrollments SET expires_at = created_at + 1 WHERE enrollment_id = ?
		`, setup.SetupToken)
		require.NoError(t, err)
		time.Sleep(10 * time.Millisecond)
		code, err := generateTOTPCode(setup.Secret, time.Now())
		require.NoError(t, err)
		_, err = auth.EnableTOTP(ctx, int(user.UserID), EnableTOTPInput{
			SetupToken: setup.SetupToken,
			Code:       code,
		})
		require.ErrorIs(t, err, ErrExpiredMFAToken)
		status, err := auth.GetMFAStatus(ctx, int(user.UserID))
		require.NoError(t, err)
		require.False(t, status.TOTPEnabled)
	})
}

func TestSecurityVersionInvalidatesPendingSetupSecurityAndLoginChallenges(t *testing.T) {
	ctx, _, auth, user, password, _ := newMFAIntegrationService(t)
	security, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
		CurrentPassword: password,
		Purpose:         securityPurposeTOTPSetup,
	})
	require.NoError(t, err)
	setup, err := auth.BeginTOTPSetup(ctx, int(user.UserID), security.SecurityToken)
	require.NoError(t, err)
	code, err := generateTOTPCode(setup.Secret, time.Now())
	require.NoError(t, err)
	enabled, err := auth.EnableTOTP(ctx, int(user.UserID), EnableTOTPInput{
		SetupToken: setup.SetupToken,
		Code:       code,
	})
	require.NoError(t, err)

	// A pending re-enrollment and an unused security proof both carry the old
	// auth_version. Disabling TOTP must invalidate both, even though the
	// pending secret was never active.
	pendingProof, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
		CurrentPassword: password,
		Code:            enabled.RecoveryCodes[1],
		Method:          MFAMethodRecoveryCode,
		Purpose:         securityPurposeTOTPSetup,
	})
	require.NoError(t, err)
	pendingSetup, err := auth.BeginTOTPSetup(ctx, int(user.UserID), pendingProof.SecurityToken)
	require.NoError(t, err)
	staleProof, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
		CurrentPassword: password,
		Code:            enabled.RecoveryCodes[2],
		Method:          MFAMethodRecoveryCode,
		Purpose:         securityPurposeTOTPSetup,
	})
	require.NoError(t, err)

	loginChallenge, err := auth.Login(LoginRequest{Username: user.Username, Password: password})
	require.NoError(t, err)
	require.True(t, loginChallenge.RequiresMFA)

	disableProof, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
		CurrentPassword: password,
		Code:            enabled.RecoveryCodes[3],
		Method:          MFAMethodRecoveryCode,
		Purpose:         securityPurposeTOTPDisable,
	})
	require.NoError(t, err)
	_, err = auth.DisableTOTP(ctx, int(user.UserID), disableProof.SecurityToken)
	require.NoError(t, err)

	_, err = auth.EnableTOTP(ctx, int(user.UserID), EnableTOTPInput{
		SetupToken: pendingSetup.SetupToken,
		Code:       code,
	})
	require.ErrorIs(t, err, ErrExpiredMFAToken)
	_, err = auth.BeginTOTPSetup(ctx, int(user.UserID), staleProof.SecurityToken)
	require.ErrorIs(t, err, ErrInvalidSecurityProof)
	_, err = auth.VerifyLoginMFA(ctx, VerifyMFARequest{
		MFAToken: loginChallenge.MFAToken,
		Method:   MFAMethodRecoveryCode,
		Code:     enabled.RecoveryCodes[4],
	})
	require.ErrorIs(t, err, ErrInvalidMFAToken)

	passwordLogin, err := auth.Login(LoginRequest{Username: user.Username, Password: password})
	require.NoError(t, err)
	require.False(t, passwordLogin.RequiresMFA)
	require.NotEmpty(t, passwordLogin.AccessToken)
}

func TestTOTPCodeCounterIsConsumedExactlyOnceConcurrently(t *testing.T) {
	ctx, _, auth, user, password, _ := newMFAIntegrationService(t)
	security, err := auth.VerifySecurity(ctx, int(user.UserID), SecurityVerificationInput{
		CurrentPassword: password,
		Purpose:         securityPurposeTOTPSetup,
	})
	require.NoError(t, err)
	setup, err := auth.BeginTOTPSetup(ctx, int(user.UserID), security.SecurityToken)
	require.NoError(t, err)
	code, err := generateTOTPCode(setup.Secret, time.Now())
	require.NoError(t, err)
	enabled, err := auth.EnableTOTP(ctx, int(user.UserID), EnableTOTPInput{SetupToken: setup.SetupToken, Code: code})
	require.NoError(t, err)

	// Enrollment records the counter used by the setup code. Use the next
	// counter for the concurrent replay fence so exactly one login attempt can
	// advance the active credential.
	nextCode, err := generateTOTPCode(setup.Secret, time.Now().Add(totpPeriod))
	require.NoError(t, err)
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- auth.verifyUserTOTP(ctx, user.UserID, nextCode)
		}()
	}
	wg.Wait()
	close(results)

	var success, invalid int
	for result := range results {
		switch {
		case result == nil:
			success++
		case result == ErrInvalidMFACode:
			invalid++
		default:
			t.Fatalf("unexpected concurrent TOTP result: %v", result)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, invalid)

	recoveryResults := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recoveryResults <- auth.consumeRecoveryCode(ctx, user.UserID, enabled.RecoveryCodes[0])
		}()
	}
	wg.Wait()
	close(recoveryResults)
	success, invalid = 0, 0
	for result := range recoveryResults {
		switch {
		case result == nil:
			success++
		case result == ErrInvalidMFACode:
			invalid++
		default:
			t.Fatalf("unexpected concurrent recovery result: %v", result)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 1, invalid)
}
