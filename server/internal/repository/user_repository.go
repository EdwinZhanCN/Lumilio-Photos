package repository

import "server/internal/models"

// UserRepository defines the interface for user data operations
type UserRepository interface {
	Create(user *models.User) error
	GetByID(id int) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	Update(user *models.User) error
	Delete(id int) error
	List(limit, offset int) ([]*models.User, error)
}

// RefreshTokenRepository defines the interface for refresh token operations
type RefreshTokenRepository interface {
	Create(token *models.RefreshToken) error
	GetByToken(token string) (*models.RefreshToken, error)
	GetByUserID(userID int) ([]*models.RefreshToken, error)
	Update(token *models.RefreshToken) error
	Delete(id int) error
	RevokeAllUserTokens(userID int) error
	CleanupExpiredTokens() error
}
