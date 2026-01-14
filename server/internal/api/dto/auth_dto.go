package dto

import "time"

// RegisterRequestDTO represents the request structure for user registration
type RegisterRequestDTO struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequestDTO represents the request structure for user login
type LoginRequestDTO struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// RefreshTokenRequestDTO represents the request structure for token refresh
type RefreshTokenRequestDTO struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// UserDTO represents user information
type UserDTO struct {
	UserID    int        `json:"user_id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	CreatedAt time.Time  `json:"created_at"`
	IsActive  bool       `json:"is_active"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

// AuthResponseDTO represents the response structure for authentication operations
type AuthResponseDTO struct {
	User         UserDTO   `json:"user"`
	AccessToken  string    `json:"token"`
	RefreshToken string    `json:"refreshToken"`
	ExpiresAt    time.Time `json:"expiresAt"`
}
