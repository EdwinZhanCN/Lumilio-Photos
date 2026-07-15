package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/config"
	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// TestBreakGlassPostgresIntegration is deliberately opt-in because it exercises
// committed transactions against a real, already-migrated PostgreSQL database.
// Point the environment variable at an isolated disposable database.
func TestBreakGlassPostgresIntegration(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("LUMILIO_BREAK_GLASS_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("set LUMILIO_BREAK_GLASS_TEST_DATABASE_URL to an isolated migrated PostgreSQL database")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	oldAdmin := "bg_old_" + suffix
	newAdmin := "bg_new_" + suffix
	ordinary := "bg_user_" + suffix
	inactiveAdmin := "bg_off_" + suffix
	usernames := []string{oldAdmin, newAdmin, ordinary, inactiveAdmin}
	oldPassword := "OriginalPass123"
	oldPasswordHash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.MinCost)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id IN (SELECT user_id FROM users WHERE username = ANY($1))`, usernames)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE username = ANY($1)`, usernames)
	})

	insertUser := func(username, role string, active bool, createdAt time.Time) int32 {
		var userID int32
		err := pool.QueryRow(ctx, `
			INSERT INTO users (username, password, role, is_active, created_at, webauthn_user_handle)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING user_id`, username, string(oldPasswordHash), role, active, createdAt, []byte(username)).Scan(&userID)
		require.NoError(t, err)
		return userID
	}

	tieTime := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	oldID := insertUser(oldAdmin, "admin", true, tieTime)
	_ = insertUser(newAdmin, "admin", true, tieTime)
	ordinaryID := insertUser(ordinary, "user", true, tieTime.Add(time.Hour))
	inactiveID := insertUser(inactiveAdmin, "admin", false, tieTime.Add(time.Hour))

	for _, ineligible := range []struct {
		username string
		userID   int32
	}{
		{ordinary, ordinaryID},
		{inactiveAdmin, inactiveID},
	} {
		_, _, err := NewUserService(repo.New(pool), pool).BreakGlassReset(ctx, ineligible.username)
		require.ErrorIs(t, err, ErrBreakGlassTargetInvalid)
		var authVersion int64
		require.NoError(t, pool.QueryRow(ctx, `SELECT auth_version FROM users WHERE user_id=$1`, ineligible.userID).Scan(&authVersion))
		require.Zero(t, authVersion, "rejected targets must not be modified")
	}
	_, _, err = NewUserService(repo.New(pool), pool).BreakGlassReset(ctx, "missing_"+suffix)
	require.ErrorIs(t, err, ErrUserNotFound)

	queries := repo.New(pool)
	auth, err := NewAuthService(queries, pool, config.AuthConfig{
		SecretKeyFile:   filepath.Join(t.TempDir(), "secret"),
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
		MediaTokenTTL:   time.Hour,
	})
	require.NoError(t, err)
	oldUser, err := queries.GetUserByID(ctx, oldID)
	require.NoError(t, err)
	oldSession, err := auth.generateAuthResponse(oldUser)
	require.NoError(t, err)
	oldMediaToken, _, err := auth.GenerateMediaToken(ctx, int(oldID))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `INSERT INTO user_mfa_totp_credentials (user_id, secret_ciphertext) VALUES ($1, $2)`, oldID, []byte("secret"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO user_mfa_recovery_codes (user_id, code_hash) VALUES ($1, $2)`, oldID, fmt.Sprintf("%064d", 1))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO user_webauthn_credentials (credential_id, user_id, public_key) VALUES ($1, $2, $3)`, []byte("credential-"+suffix), oldID, []byte("public-key"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token, expires_at) VALUES ($1, $2, now() + interval '1 day')`, oldID, "refresh-"+suffix)
	require.NoError(t, err)

	result, selected, err := NewUserService(queries, pool).BreakGlassReset(ctx, "")
	require.NoError(t, err)
	require.Equal(t, oldID, selected.UserID, "created_at ties must be broken by user_id")
	require.NotEmpty(t, result.TemporaryPassword)

	var passwordHash string
	var authVersion int64
	var passwordChangeRequired bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT password, auth_version, password_change_required FROM users WHERE user_id=$1`, oldID).Scan(&passwordHash, &authVersion, &passwordChangeRequired))
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(result.TemporaryPassword)))
	require.Equal(t, int64(1), authVersion)
	require.True(t, passwordChangeRequired)

	for table := range map[string]struct{}{
		"user_mfa_totp_credentials": {},
		"user_mfa_recovery_codes":   {},
		"user_webauthn_credentials": {},
	} {
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE user_id=$1`, oldID).Scan(&count))
		require.Zero(t, count)
	}
	var revoked bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT is_revoked FROM refresh_tokens WHERE token=$1`, "refresh-"+suffix).Scan(&revoked))
	require.True(t, revoked)

	_, err = auth.AuthenticateAccessToken(ctx, oldSession.AccessToken)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = auth.ValidateMediaToken(ctx, oldMediaToken)
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = auth.RefreshToken(oldSession.RefreshToken)
	require.ErrorIs(t, err, ErrInvalidToken)

	temporaryLogin, err := auth.Login(LoginRequest{Username: oldAdmin, Password: result.TemporaryPassword})
	require.NoError(t, err)
	require.True(t, temporaryLogin.RequiresPasswordChange)
	require.NotEmpty(t, temporaryLogin.PasswordChangeToken)
	require.Empty(t, temporaryLogin.AccessToken)
	require.Empty(t, temporaryLogin.RefreshToken)

	completed, err := auth.CompleteRequiredPasswordChange(ctx, temporaryLogin.PasswordChangeToken, "PermanentPass456")
	require.NoError(t, err)
	require.NotEmpty(t, completed.AccessToken)
	require.NotEmpty(t, completed.RefreshToken)
	_, err = auth.CompleteRequiredPasswordChange(ctx, temporaryLogin.PasswordChangeToken, "AnotherPass789")
	require.ErrorIs(t, err, ErrInvalidPasswordChangeToken)
}
