package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"server/internal/db/repo"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type UserResponse struct {
	UserID    int        `json:"user_id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"created_at"`
	IsActive  bool       `json:"is_active"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"token"`
	RefreshToken string       `json:"refreshToken"`
	ExpiresAt    time.Time    `json:"expiresAt"`
}

// AuthService handles JWT authentication operations
type AuthService struct {
	queries         *repo.Queries
	jwtSecret       []byte
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

// JWTClaims represents the claims in the JWT token
type JWTClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// NewAuthService creates a new authentication service
func NewAuthService(queries *repo.Queries) *AuthService {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-secret-key-change-in-production" // Default for development
	}

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

	return &AuthService{
		queries:         queries,
		jwtSecret:       []byte(jwtSecret),
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

// Register creates a new user account
func (s *AuthService) Register(req RegisterRequest) (*AuthResponse, error) {
	// Check if user already exists
	_, err := s.queries.GetUserByUsername(context.Background(), req.Username)
	if err == nil {
		return nil, ErrUserAlreadyExists
	}

	// Check if email already exists
	_, err = s.queries.GetUserByEmail(context.Background(), req.Email)
	if err == nil {
		return nil, ErrUserAlreadyExists
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("error hashing password: %w", err)
	}

	// Create new user
	params := repo.CreateUserParams{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashedPassword),
	}

	user, err := s.queries.CreateUser(context.Background(), params)
	if err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	// Generate tokens
	return s.generateAuthResponse(user)
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(req LoginRequest) (*AuthResponse, error) {
	// Get user by username
	user, err := s.queries.GetUserByUsername(context.Background(), req.Username)
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

	// Update last login
	now := pgtype.Timestamptz{}
	now.Scan(time.Now())

	_, err = s.queries.UpdateUser(context.Background(), repo.UpdateUserParams{
		UserID:    user.UserID,
		Username:  user.Username,
		Email:     user.Email,
		LastLogin: now,
	})
	if err != nil {
		// Log error but don't fail login
		fmt.Printf("Warning: failed to update last login for user %d: %v\n", user.UserID, err)
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

// RevokeRefreshToken revokes a refresh token
func (s *AuthService) RevokeRefreshToken(refreshTokenString string) error {
	refreshToken, err := s.queries.GetRefreshTokenByToken(context.Background(), refreshTokenString)
	if err != nil {
		return ErrTokenNotFound
	}

	return s.queries.RevokeRefreshToken(context.Background(), refreshToken.TokenID)
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
		User:         ConvertUserToResponse(user),
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
		ExpiresAt:    expiresAt,
	}, nil
}

// generateAccessToken creates a new JWT access token
func (s *AuthService) generateAccessToken(user repo.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.accessTokenTTL)

	claims := &JWTClaims{
		UserID:   int(user.UserID),
		Username: user.Username,
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

	return UserResponse{
		UserID:    int(user.UserID),
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt.Time,
		IsActive:  user.IsActive != nil && *user.IsActive,
		LastLogin: lastLogin,
	}
}
