package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"server/internal/models"
	"server/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

var (
	ErrInvalidToken      = errors.New("invalid token")
	ErrExpiredToken      = errors.New("token has expired")
	ErrTokenNotFound     = errors.New("token not found")
	ErrUserNotFound      = errors.New("user not found")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrUserAlreadyExists = errors.New("user already exists")
)

// AuthService handles JWT authentication operations
type AuthService struct {
	userRepo         repository.UserRepository
	refreshTokenRepo repository.RefreshTokenRepository
	jwtSecret        []byte
	accessTokenTTL   time.Duration
	refreshTokenTTL  time.Duration
}

// JWTClaims represents the claims in the JWT token
type JWTClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// NewAuthService creates a new authentication service
func NewAuthService(userRepo repository.UserRepository, refreshTokenRepo repository.RefreshTokenRepository) *AuthService {
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
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		jwtSecret:        []byte(jwtSecret),
		accessTokenTTL:   accessTokenTTL,
		refreshTokenTTL:  refreshTokenTTL,
	}
}

// Register creates a new user account
func (s *AuthService) Register(req models.RegisterRequest) (*models.AuthResponse, error) {
	// Check if user already exists
	existingUser, err := s.userRepo.GetByUsername(req.Username)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("error checking existing user: %w", err)
	}
	if existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	// Check if email already exists
	existingUser, err = s.userRepo.GetByEmail(req.Email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("error checking existing email: %w", err)
	}
	if existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	// Create new user
	user := &models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password, // Will be hashed by BeforeCreate hook
		IsActive: true,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("error creating user: %w", err)
	}

	// Generate tokens
	return s.generateAuthResponse(user)
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(req models.LoginRequest) (*models.AuthResponse, error) {
	// Get user by username
	user, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("error getting user: %w", err)
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserNotFound // Don't reveal that user exists but is inactive
	}

	// Verify password
	if !user.CheckPassword(req.Password) {
		return nil, ErrInvalidPassword
	}

	// Update last login
	now := time.Now()
	user.LastLogin = &now
	if err := s.userRepo.Update(user); err != nil {
		// Log error but don't fail login
		fmt.Printf("Warning: failed to update last login for user %d: %v\n", user.UserID, err)
	}

	// Generate tokens
	return s.generateAuthResponse(user)
}

// RefreshToken generates a new access token using a refresh token
func (s *AuthService) RefreshToken(refreshTokenString string) (*models.AuthResponse, error) {
	// Get refresh token from database
	refreshToken, err := s.refreshTokenRepo.GetByToken(refreshTokenString)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenNotFound
		}
		return nil, fmt.Errorf("error getting refresh token: %w", err)
	}

	// Check if token is revoked
	if refreshToken.IsRevoked {
		return nil, ErrInvalidToken
	}

	// Check if token is expired
	if time.Now().After(refreshToken.ExpiresAt) {
		// Revoke expired token
		refreshToken.IsRevoked = true
		s.refreshTokenRepo.Update(refreshToken)
		return nil, ErrExpiredToken
	}

	// Get user
	user, err := s.userRepo.GetByID(refreshToken.UserID)
	if err != nil {
		return nil, fmt.Errorf("error getting user: %w", err)
	}

	// Check if user is still active
	if !user.IsActive {
		// Revoke token for inactive user
		refreshToken.IsRevoked = true
		s.refreshTokenRepo.Update(refreshToken)
		return nil, ErrUserNotFound
	}

	// Generate new tokens
	authResponse, err := s.generateAuthResponse(user)
	if err != nil {
		return nil, err
	}

	// Revoke old refresh token
	refreshToken.IsRevoked = true
	if err := s.refreshTokenRepo.Update(refreshToken); err != nil {
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
	refreshToken, err := s.refreshTokenRepo.GetByToken(refreshTokenString)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrTokenNotFound
		}
		return fmt.Errorf("error getting refresh token: %w", err)
	}

	refreshToken.IsRevoked = true
	return s.refreshTokenRepo.Update(refreshToken)
}

// RevokeAllUserTokens revokes all refresh tokens for a user
func (s *AuthService) RevokeAllUserTokens(userID int) error {
	return s.refreshTokenRepo.RevokeAllUserTokens(userID)
}

// generateAuthResponse creates an authentication response with tokens
func (s *AuthService) generateAuthResponse(user *models.User) (*models.AuthResponse, error) {
	// Generate access token
	accessToken, expiresAt, err := s.generateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("error generating access token: %w", err)
	}

	// Generate refresh token
	refreshTokenString, err := s.generateRefreshToken(user.UserID)
	if err != nil {
		return nil, fmt.Errorf("error generating refresh token: %w", err)
	}

	return &models.AuthResponse{
		User:         user.ToResponse(),
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
		ExpiresAt:    expiresAt,
	}, nil
}

// generateAccessToken creates a new JWT access token
func (s *AuthService) generateAccessToken(user *models.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.accessTokenTTL)

	claims := &JWTClaims{
		UserID:   user.UserID,
		Username: user.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "lumilio-photos",
			Subject:   strconv.Itoa(user.UserID),
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
	refreshToken := &models.RefreshToken{
		UserID:    userID,
		Token:     tokenString,
		ExpiresAt: time.Now().Add(s.refreshTokenTTL),
		IsRevoked: false,
	}

	if err := s.refreshTokenRepo.Create(refreshToken); err != nil {
		return "", fmt.Errorf("error saving refresh token: %w", err)
	}

	return tokenString, nil
}
