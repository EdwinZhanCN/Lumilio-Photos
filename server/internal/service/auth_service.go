package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"server/internal/db/repo"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken                = errors.New("invalid token")
	ErrExpiredToken                = errors.New("token has expired")
	ErrTokenNotFound               = errors.New("token not found")
	ErrUserNotFound                = errors.New("user not found")
	ErrInvalidPassword             = errors.New("invalid password")
	ErrUserAlreadyExists           = errors.New("user already exists")
	ErrRegistrationSessionNotFound = errors.New("registration session not found")
	ErrRegistrationSessionExpired  = errors.New("registration session expired")
)

// Request/Response types
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type UserResponse struct {
	UserID      int        `json:"user_id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"display_name"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	IsActive    bool       `json:"is_active"`
	LastLogin   *time.Time `json:"last_login,omitempty"`
	Role        string     `json:"role"`
	Permissions []string   `json:"permissions"`
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

type BootstrapStatus struct {
	HasUsers             bool   `json:"has_users"`
	IsBootstrapMode      bool   `json:"is_bootstrap_mode"`
	NextRegistrationRole string `json:"next_registration_role"`
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

// NewAuthService creates a new authentication service
func NewAuthService(queries *repo.Queries, db *pgxpool.Pool) *AuthService {
	rootSecret, err := loadOrCreateLumilioSecretKey(strings.TrimSpace(os.Getenv("LUMILIO_SECRET_KEY")))
	if err != nil {
		panic(fmt.Sprintf("failed to initialize JWT secret from LUMILIO_SECRET_KEY: %v", err))
	}
	jwtSecret := deriveScopedSecret(rootSecret, "jwt.signing.v1")
	mfaTokenSecret := deriveScopedSecret(rootSecret, "mfa.signing.v1")
	passkeyTokenSecret := deriveScopedSecret(rootSecret, "passkey.signing.v1")
	mediaTokenSecret := deriveScopedSecret(rootSecret, "media.url.signing.v1")
	mfaEncryptKey := deriveScopedSecret(rootSecret, "mfa.encryption.v1")

	// Access token TTL (default: 15 minutes)
	accessTokenTTL := 15 * time.Minute
	if ttlStr := os.Getenv("ACCESS_TOKEN_TTL"); ttlStr != "" {
		if ttl, err := time.ParseDuration(ttlStr); err == nil {
			accessTokenTTL = ttl
		}
	}

	// Refresh token TTL (default: 7 days)
	refreshTokenTTL := 7 * 24 * time.Hour
	if ttlStr := os.Getenv("REFRESH_TOKEN_TTL"); ttlStr != "" {
		if ttl, err := time.ParseDuration(ttlStr); err == nil {
			refreshTokenTTL = ttl
		}
	}

	// Media token TTL (default: 10 minutes)
	mediaTokenTTL := 10 * time.Minute
	if ttlStr := os.Getenv("MEDIA_TOKEN_TTL"); ttlStr != "" {
		if ttl, err := time.ParseDuration(ttlStr); err == nil {
			mediaTokenTTL = ttl
		}
	}

	return &AuthService{
		queries:                queries,
		db:                     db,
		jwtSecret:              jwtSecret,
		mfaTokenSecret:         mfaTokenSecret,
		passkeyTokenSecret:     passkeyTokenSecret,
		mediaTokenSecret:       mediaTokenSecret,
		mfaEncryptKey:          mfaEncryptKey,
		accessTokenTTL:         accessTokenTTL,
		refreshTokenTTL:        refreshTokenTTL,
		mediaTokenTTL:          mediaTokenTTL,
		webauthnRPDisplayName:  loadWebAuthnRPDisplayName(),
		webauthnRPID:           loadWebAuthnRPID(),
		webauthnAllowedOrigins: loadWebAuthnAllowedOrigins(),
	}
}

func deriveScopedSecret(rootSecret string, scope string) []byte {
	sum := sha256.Sum256([]byte(scope + "\x00" + rootSecret))
	derived := make([]byte, len(sum))
	copy(derived, sum[:])
	return derived
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
		fmt.Printf("Warning: failed to update last login for user %d: %v\n", user.UserID, err)
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

	// Check if token is revoked
	if refreshToken.IsRevoked != nil && *refreshToken.IsRevoked {
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

	// Generate new tokens
	authResponse, err := s.generateAuthResponse(user)
	if err != nil {
		return nil, err
	}

	// Revoke old refresh token
	if err := s.queries.RevokeRefreshToken(context.Background(), refreshToken.TokenID); err != nil {
		// Log error but don't fail the refresh
		fmt.Printf("Warning: failed to revoke old refresh token: %v\n", err)
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

func (s *AuthService) GetBootstrapStatus(ctx context.Context) (BootstrapStatus, error) {
	userCount, err := s.queries.CountUsers(ctx)
	if err != nil {
		return BootstrapStatus{}, fmt.Errorf("count users: %w", err)
	}

	if userCount == 0 {
		return BootstrapStatus{
			HasUsers:             false,
			IsBootstrapMode:      true,
			NextRegistrationRole: string(UserRoleAdmin),
		}, nil
	}

	return BootstrapStatus{
		HasUsers:             true,
		IsBootstrapMode:      false,
		NextRegistrationRole: string(UserRoleUser),
	}, nil
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
		UserID:      int(user.UserID),
		Username:    user.Username,
		DisplayName: displayName,
		AvatarURL:   cloneOptionalString(user.AvatarUrl),
		CreatedAt:   user.CreatedAt.Time,
		UpdatedAt:   user.UpdatedAt.Time,
		IsActive:    user.IsActive != nil && *user.IsActive,
		LastLogin:   lastLogin,
		Role:        string(role),
		Permissions: PermissionsForRole(role),
	}
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
