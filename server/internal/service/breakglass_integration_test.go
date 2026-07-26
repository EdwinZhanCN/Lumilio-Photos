package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/db/dbtypes"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestBreakGlassSQLiteIntegration(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))
	database, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "breakglass.sqlite"),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, database.Close(context.Background()))
	})
	require.NoError(t, database.Migrate(ctx))

	oldAdmin := "bg_old"
	newAdmin := "bg_new"
	ordinary := "bg_user"
	inactiveAdmin := "bg_off"
	oldPassword := "OriginalPass123"
	oldPasswordHash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.MinCost)
	require.NoError(t, err)

	insertUser := func(username, role string, active bool, createdAt time.Time) int32 {
		var userID int32
		created := dbtypes.NewTimestamp(createdAt)
		err := database.SQL.QueryRowContext(ctx, `
			INSERT INTO users (
				username,
				password,
				role,
				is_active,
				created_at,
				updated_at,
				webauthn_user_handle
			)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			RETURNING user_id`,
			username,
			string(oldPasswordHash),
			role,
			active,
			created,
			created,
			[]byte(username),
		).Scan(&userID)
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
		_, _, err := NewUserService(database.Queries, database.SQL).BreakGlassReset(ctx, ineligible.username)
		require.ErrorIs(t, err, ErrBreakGlassTargetInvalid)
		var authVersion int64
		require.NoError(t, database.SQL.QueryRowContext(ctx, `SELECT auth_version FROM users WHERE user_id = ?`, ineligible.userID).Scan(&authVersion))
		require.Zero(t, authVersion, "rejected targets must not be modified")
	}
	_, _, err = NewUserService(database.Queries, database.SQL).BreakGlassReset(ctx, "missing")
	require.ErrorIs(t, err, ErrUserNotFound)

	queries := database.Queries
	auth, err := NewAuthService(queries, database.SQL, config.AuthConfig{
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

	now := dbtypes.NewTimestamp(time.Now())
	_, err = database.SQL.ExecContext(ctx, `
		INSERT INTO user_mfa_totp_credentials (
			user_id,
			secret_ciphertext,
			created_at,
			updated_at,
			enabled_at
		)
		VALUES (?, ?, ?, ?, ?)`,
		oldID,
		[]byte("secret"),
		now,
		now,
		now,
	)
	require.NoError(t, err)
	_, err = database.SQL.ExecContext(ctx, `
		INSERT INTO user_mfa_recovery_codes (user_id, code_hash, created_at)
		VALUES (?, ?, ?)`,
		oldID,
		fmt.Sprintf("%064d", 1),
		now,
	)
	require.NoError(t, err)
	_, err = database.SQL.ExecContext(ctx, `
		INSERT INTO user_webauthn_credentials (
			credential_id,
			user_id,
			public_key,
			created_at
		)
		VALUES (?, ?, ?, ?)`,
		[]byte("credential"),
		oldID,
		[]byte("public-key"),
		now,
	)
	require.NoError(t, err)
	_, err = database.SQL.ExecContext(ctx, `
		INSERT INTO refresh_tokens (user_id, token, expires_at, created_at)
		VALUES (?, ?, ?, ?)`,
		oldID,
		"refresh-test",
		dbtypes.NewTimestamp(time.Now().Add(24*time.Hour)),
		now,
	)
	require.NoError(t, err)

	result, selected, err := NewUserService(queries, database.SQL).BreakGlassReset(ctx, "")
	require.NoError(t, err)
	require.Equal(t, oldID, selected.UserID, "created_at ties must be broken by user_id")
	require.NotEmpty(t, result.TemporaryPassword)

	var passwordHash string
	var authVersion int64
	var passwordChangeRequired bool
	require.NoError(t, database.SQL.QueryRowContext(ctx, `
		SELECT password, auth_version, password_change_required
		FROM users
		WHERE user_id = ?`,
		oldID,
	).Scan(&passwordHash, &authVersion, &passwordChangeRequired))
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(result.TemporaryPassword)))
	require.Equal(t, int64(1), authVersion)
	require.True(t, passwordChangeRequired)

	for table := range map[string]struct{}{
		"user_mfa_totp_credentials": {},
		"user_mfa_recovery_codes":   {},
		"user_webauthn_credentials": {},
	} {
		var count int
		require.NoError(t, database.SQL.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE user_id = ?`, oldID).Scan(&count))
		require.Zero(t, count)
	}
	var revoked bool
	require.NoError(t, database.SQL.QueryRowContext(ctx, `
		SELECT is_revoked
		FROM refresh_tokens
		WHERE token = ?`,
		"refresh-test",
	).Scan(&revoked))
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
