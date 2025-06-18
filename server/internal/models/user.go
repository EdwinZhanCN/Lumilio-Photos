package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User represents a user in the system
// @Description User account information for authentication and authorization
type User struct {
	UserID    int        `gorm:"primaryKey;autoIncrement" json:"user_id" example:"1"`
	Username  string     `gorm:"type:varchar(50);uniqueIndex;not null" json:"username" example:"john_doe"`
	Email     string     `gorm:"type:varchar(100);uniqueIndex;not null" json:"email" example:"john@example.com"`
	Password  string     `gorm:"type:varchar(255);not null" json:"-"` // Never include password in JSON responses
	CreatedAt time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at" example:"2024-01-16T10:30:00Z"`
	IsActive  bool       `gorm:"default:true" json:"is_active" example:"true"`
	LastLogin *time.Time `json:"last_login,omitempty" example:"2024-01-16T15:45:00Z"`
}

// RefreshToken represents a refresh token for JWT authentication
// @Description Refresh token for JWT token renewal
type RefreshToken struct {
	TokenID   int       `gorm:"primaryKey;autoIncrement" json:"token_id"`
	UserID    int       `gorm:"not null;index" json:"user_id" example:"1"`
	Token     string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"-"` // Never expose refresh token
	ExpiresAt time.Time `gorm:"not null" json:"expires_at" example:"2024-02-15T10:30:00Z"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at" example:"2024-01-15T10:30:00Z"`
	IsRevoked bool      `gorm:"default:false" json:"is_revoked" example:"false"`

	// Relationship
	User User `gorm:"foreignKey:UserID" json:"-"`
}

// BeforeCreate hook to hash password before saving to database
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		u.Password = string(hashedPassword)
	}
	return nil
}

// CheckPassword verifies if the provided password matches the user's password
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// UserResponse represents the user data returned in API responses
// @Description User information for API responses (excludes sensitive data)
type UserResponse struct {
	UserID    int        `json:"user_id" example:"1"`
	Username  string     `json:"username" example:"john_doe"`
	Email     string     `json:"email" example:"john@example.com"`
	CreatedAt time.Time  `json:"created_at" example:"2024-01-15T10:30:00Z"`
	IsActive  bool       `json:"is_active" example:"true"`
	LastLogin *time.Time `json:"last_login,omitempty" example:"2024-01-16T15:45:00Z"`
}

// ToResponse converts User model to UserResponse
func (u *User) ToResponse() UserResponse {
	return UserResponse{
		UserID:    u.UserID,
		Username:  u.Username,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		IsActive:  u.IsActive,
		LastLogin: u.LastLogin,
	}
}

// LoginRequest represents the login request payload
// @Description Login credentials
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"john_doe"`
	Password string `json:"password" binding:"required" example:"securepassword123"`
}

// RegisterRequest represents the registration request payload
// @Description User registration data
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50" example:"john_doe"`
	Email    string `json:"email" binding:"required,email" example:"john@example.com"`
	Password string `json:"password" binding:"required,min=6" example:"securepassword123"`
}

// RefreshTokenRequest represents the refresh token request payload
// @Description Refresh token for JWT renewal
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// AuthResponse represents the authentication response
// @Description Authentication response containing tokens and user info
type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	RefreshToken string       `json:"refreshToken" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	ExpiresAt    time.Time    `json:"expiresAt" example:"2024-01-16T10:30:00Z"`
}
