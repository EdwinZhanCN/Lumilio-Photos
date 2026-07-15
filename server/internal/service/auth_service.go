package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"server/config"
	"server/internal/db/repo"
	"server/internal/secretbox"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken      = errors.New("invalid token")
	ErrExpiredToken      = errors.New("token has expired")
	ErrTokenNotFound     = errors.New("token not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrUserAlreadyExists = errors.New("user already exists")
)

// Request/Response types
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginOptions is the public capability probe for identifier-first login.
// Password is always true for syntactically valid usernames so unknown and
// inactive accounts are not distinguishable from password-only accounts.
// Passkey is true only for active users with at least one enrolled passkey.
// TOTP is intentionally omitted — it is revealed only after password success.
type LoginOptions struct {
	Password bool `json:"password"`
	Passkey  bool `json:"passkey"`
}

func passwordOnlyLoginOptions() LoginOptions {
	return LoginOptions{Password: true, Passkey: false}
}

func loginOptionsFromPasskeyCount(passkeyCount int64) LoginOptions {
	return LoginOptions{Password: true, Passkey: passkeyCount > 0}
}

// resolveLoginOptions maps account lookup results to the public probe shape.
// Unknown and inactive users match password-only so existence is not leaked
// beyond the passkey bit already exposed by passkey login options.
func resolveLoginOptions(found bool, active bool, passkeyCount int64) LoginOptions {
	if !found || !active {
		return passwordOnlyLoginOptions()
	}
	return loginOptionsFromPasskeyCount(passkeyCount)
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type UserResponse struct {
	UserID        int        `json:"user_id"`
	Username      string     `json:"username"`
	DisplayName   string     `json:"display_name"`
	AvatarAssetID *string    `json:"avatar_asset_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	IsActive      bool       `json:"is_active"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
	Role          string     `json:"role"`
	Permissions   []string   `json:"permissions"`
}

type AuthResponse struct {
	User           *UserResponse `json:"user,omitempty"`
	AccessToken    string        `json:"token,omitempty"`
	RefreshToken   string        `json:"refreshToken,omitempty"`
	ExpiresAt      *time.Time    `json:"expiresAt,omitempty"`
	RequiresMFA    bool          `json:"requires_mfa"`
	MFAToken       string        `json:"mfa_token,omitempty"`
	MFAMethods     []string      `json:"mfa_methods,omitempty"`
	BootstrapAdmin bool          `json:"bootstrap_admin,omitempty"`
}

// AuthService handles JWT authentication operations
type AuthService struct {
	queries                *repo.Queries
	db                     *pgxpool.Pool
	jwtSecret              []byte
	mfaTokenSecret         []byte
	passkeyTokenSecret     []byte
	mediaTokenSecret       []byte
	mfaEncryptKey          []byte
	accessTokenTTL         time.Duration
	refreshTokenTTL        time.Duration
	mediaTokenTTL          time.Duration
	webauthnRPDisplayName  string
	webauthnRPID           string
	webauthnAllowedOrigins []string
	logger                 *zap.Logger
}

// JWTClaims represents the claims in the JWT token
type JWTClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type MediaTokenClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Scope    string `json:"scope"`
	jwt.RegisteredClaims
}

const mediaTokenScope = "media"

// NewAuthService creates a new authentication service. An optional zap logger
// can be supplied for structured auth/audit logging; when omitted, a no-op
// logger is used so callers (and tests) without logging stay valid.
func NewAuthService(queries *repo.Queries, db *pgxpool.Pool, cfg config.AuthConfig, loggers ...*zap.Logger) (*AuthService, error) {
	logger := zap.NewNop()
	if len(loggers) > 0 && loggers[0] != nil {
		logger = loggers[0]
	}
	rootSecret, err := secretbox.LoadOrCreateLumilioSecretKey(strings.TrimSpace(cfg.SecretKeyFile))
	if err != nil {
		return nil, fmt.Errorf("initialize root secret key: %w", err)
	}
	jwtSecret := secretbox.DeriveScopedSecret(rootSecret, "jwt.signing.v1")
	mfaTokenSecret := secretbox.DeriveScopedSecret(rootSecret, "mfa.signing.v1")
	passkeyTokenSecret := secretbox.DeriveScopedSecret(rootSecret, "passkey.signing.v1")
	mediaTokenSecret := secretbox.DeriveScopedSecret(rootSecret, "media.url.signing.v1")
	mfaEncryptKey := secretbox.DeriveScopedSecret(rootSecret, "mfa.encryption.v1")

	return &AuthService{
		queries:                queries,
		db:                     db,
		jwtSecret:              jwtSecret,
		mfaTokenSecret:         mfaTokenSecret,
		passkeyTokenSecret:     passkeyTokenSecret,
		mediaTokenSecret:       mediaTokenSecret,
		mfaEncryptKey:          mfaEncryptKey,
		accessTokenTTL:         cfg.AccessTokenTTL,
		refreshTokenTTL:        cfg.RefreshTokenTTL,
		mediaTokenTTL:          cfg.MediaTokenTTL,
		webauthnRPDisplayName:  cfg.WebAuthnRPName,
		webauthnRPID:           strings.TrimSpace(cfg.WebAuthnRPID),
		webauthnAllowedOrigins: normalizeConfiguredWebAuthnOrigins(cfg.WebAuthnRPOrigins),
		logger:                 logger,
	}, nil
}

func normalizeConfiguredWebAuthnOrigins(values []string) []string {
	origins := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		normalized, _, err := normalizeOriginString(value)
		if err != nil {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		origins = append(origins, normalized)
	}
	return origins
}

// GetLoginOptions returns which login methods the client should present after
// the user enters a username. It does not start a WebAuthn ceremony.
func (s *AuthService) GetLoginOptions(ctx context.Context, username string) (LoginOptions, error) {
	normalized, err := normalizeUsername(username)
	if err != nil {
		return LoginOptions{}, err
	}

	user, err := s.queries.GetUserByUsername(ctx, normalized)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return resolveLoginOptions(false, false, 0), nil
		}
		return LoginOptions{}, fmt.Errorf("get user by username: %w", err)
	}

	if user.IsActive == nil || !*user.IsActive {
		return resolveLoginOptions(true, false, 0), nil
	}

	status, err := s.queries.GetUserMFAStatus(ctx, user.UserID)
	if err != nil {
		return LoginOptions{}, fmt.Errorf("load mfa status: %w", err)
	}

	return resolveLoginOptions(true, true, status.PasskeyCount), nil
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(req LoginRequest) (*AuthResponse, error) {
	// Get user by username
	user, err := s.queries.GetUserByUsername(context.Background(), strings.ToLower(strings.TrimSpace(req.Username)))
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Check if user is active
	if user.IsActive == nil || !*user.IsActive {
		return nil, ErrUserNotFound // Don't reveal that user exists but is inactive
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidPassword
	}

	status, err := s.queries.GetUserMFAStatus(context.Background(), user.UserID)
	if err != nil {
		return nil, fmt.Errorf("error loading mfa status: %w", err)
	}

	if coerceBool(status.TotpEnabled) {
		challenge, err := s.issueLoginMFAChallenge(user, status)
		if err != nil {
			return nil, fmt.Errorf("error creating mfa challenge: %w", err)
		}
		return challenge, nil
	}

	lastLogin, err := s.updateUserLastLogin(context.Background(), user.UserID)
	if err != nil {
		// Log error but don't fail login
		s.logger.Warn("failed to update last login",
			zap.Int32("user_id", user.UserID), zap.Error(err))
	} else {
		user.LastLogin = lastLogin
	}

	// Generate tokens
	return s.generateAuthResponse(user)
}

// RefreshToken generates a new access token using a refresh token
func (s *AuthService) RefreshToken(refreshTokenString string) (*AuthResponse, error) {
	// Get refresh token from database
	refreshToken, err := s.queries.GetRefreshTokenByToken(context.Background(), refreshTokenString)
	if err != nil {
		return nil, ErrTokenNotFound
	}

	// Check if token is revoked. Presenting an already-revoked (but not yet
	// expired) refresh token is a strong signal of token theft/replay: the
	// legitimate client rotates to a fresh token, so the only party still
	// holding the old one is an attacker (or vice versa). Treat reuse as a
	// breach and revoke the user's entire token family to force every device
	// to re-authenticate.
	if refreshToken.IsRevoked != nil && *refreshToken.IsRevoked {
		if err := s.queries.RevokeUserRefreshTokens(context.Background(), refreshToken.UserID); err != nil {
			s.logger.Error("failed to revoke refresh token family after reuse",
				zap.Int32("user_id", refreshToken.UserID), zap.Error(err))
		} else {
			s.logger.Warn("refresh token reuse detected; revoked all sessions",
				zap.Int32("user_id", refreshToken.UserID))
		}
		return nil, ErrInvalidToken
	}

	// Check if token is expired
	if time.Now().After(refreshToken.ExpiresAt.Time) {
		// Revoke expired token
		s.queries.RevokeRefreshToken(context.Background(), refreshToken.TokenID)
		return nil, ErrExpiredToken
	}

	// Get user
	user, err := s.queries.GetUserByID(context.Background(), refreshToken.UserID)
	if err != nil {
		return nil, fmt.Errorf("error getting user: %w", err)
	}

	// Check if user is still active
	if user.IsActive == nil || !*user.IsActive {
		// Revoke token for inactive user
		s.queries.RevokeRefreshToken(context.Background(), refreshToken.TokenID)
		return nil, ErrUserNotFound
	}

	// Rotate fail-closed: revoke the presented refresh token *before* issuing a
	// new one. If revocation fails we abort instead of leaving two valid tokens
	// in circulation; the user simply re-authenticates.
	if err := s.queries.RevokeRefreshToken(context.Background(), refreshToken.TokenID); err != nil {
		return nil, fmt.Errorf("revoke refresh token during rotation: %w", err)
	}

	// Generate new tokens
	authResponse, err := s.generateAuthResponse(user)
	if err != nil {
		return nil, err
	}

	return authResponse, nil
}

// ValidateToken validates a JWT access token and returns the claims
func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("error parsing token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

func (s *AuthService) GenerateMediaToken(userID int, username, role string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(s.mediaTokenTTL)

	claims := &MediaTokenClaims{
		UserID:   userID,
		Username: username,
		Role:     string(normalizeUserRole(role)),
		Scope:    mediaTokenScope,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "lumilio-photos",
			Subject:   strconv.Itoa(userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.mediaTokenSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign media token: %w", err)
	}

	return tokenString, expiresAt, nil
}

func (s *AuthService) ValidateMediaToken(tokenString string) (*MediaTokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &MediaTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.mediaTokenSecret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error parsing media token: %w", err)
	}

	claims, ok := token.Claims.(*MediaTokenClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.Scope != mediaTokenScope {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// RevokeRefreshToken revokes a refresh token
func (s *AuthService) RevokeRefreshToken(refreshTokenString string) error {
	refreshToken, err := s.queries.GetRefreshTokenByToken(context.Background(), refreshTokenString)
	if err != nil {
		return ErrTokenNotFound
	}

	return s.queries.RevokeRefreshToken(context.Background(), refreshToken.TokenID)
}

func (s *AuthService) GetCurrentUser(userID int) (*UserResponse, error) {
	user, err := s.queries.GetUserByID(context.Background(), int32(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("error getting current user: %w", err)
	}

	if user.IsActive == nil || !*user.IsActive {
		return nil, ErrUserNotFound
	}

	response := ConvertUserToResponse(user)
	return &response, nil
}

// generateAuthResponse creates an authentication response with tokens
func (s *AuthService) generateAuthResponse(user repo.User) (*AuthResponse, error) {
	// Generate access token
	accessToken, expiresAt, err := s.generateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("error generating access token: %w", err)
	}

	// Generate refresh token
	refreshTokenString, err := s.generateRefreshToken(int(user.UserID))
	if err != nil {
		return nil, fmt.Errorf("error generating refresh token: %w", err)
	}

	return &AuthResponse{
		User:         ptr(ConvertUserToResponse(user)),
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
		ExpiresAt:    &expiresAt,
	}, nil
}

// generateAccessToken creates a new JWT access token
func (s *AuthService) generateAccessToken(user repo.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.accessTokenTTL)

	claims := &JWTClaims{
		UserID:   int(user.UserID),
		Username: user.Username,
		Role:     string(normalizeUserRole(user.Role)),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "lumilio-photos",
			Subject:   strconv.Itoa(int(user.UserID)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// generateRefreshToken creates a new refresh token
func (s *AuthService) generateRefreshToken(userID int) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("error generating random token: %w", err)
	}
	tokenString := hex.EncodeToString(tokenBytes)

	// Create refresh token record
	expiresAt := pgtype.Timestamptz{}
	expiresAt.Scan(time.Now().Add(s.refreshTokenTTL))

	params := repo.CreateRefreshTokenParams{
		UserID:    int32(userID),
		Token:     tokenString,
		ExpiresAt: expiresAt,
	}

	if _, err := s.queries.CreateRefreshToken(context.Background(), params); err != nil {
		return "", fmt.Errorf("error saving refresh token: %w", err)
	}

	return tokenString, nil
}

// ConvertUserToResponse converts SQLC User model to UserResponse
func ConvertUserToResponse(user repo.User) UserResponse {
	var lastLogin *time.Time
	if user.LastLogin.Valid {
		lastLogin = &user.LastLogin.Time
	}

	role := normalizeUserRole(user.Role)
	displayName := strings.TrimSpace(user.DisplayName)
	if displayName == "" {
		displayName = user.Username
	}

	return UserResponse{
		UserID:        int(user.UserID),
		Username:      user.Username,
		DisplayName:   displayName,
		AvatarAssetID: uuidToOptionalString(user.AvatarAssetID),
		CreatedAt:     user.CreatedAt.Time,
		UpdatedAt:     user.UpdatedAt.Time,
		IsActive:      user.IsActive != nil && *user.IsActive,
		LastLogin:     lastLogin,
		Role:          string(role),
		Permissions:   PermissionsForRole(role),
	}
}

func uuidToOptionalString(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}

	id := uuid.UUID(value.Bytes).String()
	return &id
}

func optionalStringToUUID(value *string) (pgtype.UUID, error) {
	if value == nil {
		return pgtype.UUID{}, nil
	}

	parsed, err := uuid.Parse(*value)
	if err != nil {
		return pgtype.UUID{}, err
	}

	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func (s *AuthService) updateUserLastLogin(ctx context.Context, userID int32) (pgtype.Timestamptz, error) {
	now := pgtype.Timestamptz{}
	if err := now.Scan(time.Now()); err != nil {
		return pgtype.Timestamptz{}, fmt.Errorf("scan current time: %w", err)
	}

	if err := s.queries.UpdateUserLastLogin(ctx, repo.UpdateUserLastLoginParams{
		UserID:    userID,
		LastLogin: now,
	}); err != nil {
		return pgtype.Timestamptz{}, err
	}

	return now, nil
}

func ptr[T any](value T) *T {
	return &value
}
