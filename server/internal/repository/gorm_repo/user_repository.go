package gorm_repo

import (
	"server/internal/models"
	"server/internal/repository"

	"gorm.io/gorm"
)

// userRepository implements the UserRepository interface using GORM
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) repository.UserRepository {
	return &userRepository{db: db}
}

// Create creates a new user
func (r *userRepository) Create(user *models.User) error {
	return r.db.Create(user).Error
}

// GetByID retrieves a user by ID
func (r *userRepository) GetByID(id int) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername retrieves a user by username
func (r *userRepository) GetByUsername(username string) (*models.User, error) {
	var user models.User
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail retrieves a user by email
func (r *userRepository) GetByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update updates a user
func (r *userRepository) Update(user *models.User) error {
	return r.db.Save(user).Error
}

// Delete deletes a user by ID
func (r *userRepository) Delete(id int) error {
	return r.db.Delete(&models.User{}, id).Error
}

// List retrieves users with pagination
func (r *userRepository) List(limit, offset int) ([]*models.User, error) {
	var users []*models.User
	err := r.db.Limit(limit).Offset(offset).Find(&users).Error
	return users, err
}

// refreshTokenRepository implements the RefreshTokenRepository interface using GORM
type refreshTokenRepository struct {
	db *gorm.DB
}

// NewRefreshTokenRepository creates a new refresh token repository
func NewRefreshTokenRepository(db *gorm.DB) repository.RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

// Create creates a new refresh token
func (r *refreshTokenRepository) Create(token *models.RefreshToken) error {
	return r.db.Create(token).Error
}

// GetByToken retrieves a refresh token by token string
func (r *refreshTokenRepository) GetByToken(token string) (*models.RefreshToken, error) {
	var refreshToken models.RefreshToken
	err := r.db.Where("token = ? AND is_revoked = ?", token, false).First(&refreshToken).Error
	if err != nil {
		return nil, err
	}
	return &refreshToken, nil
}

// GetByUserID retrieves all refresh tokens for a user
func (r *refreshTokenRepository) GetByUserID(userID int) ([]*models.RefreshToken, error) {
	var tokens []*models.RefreshToken
	err := r.db.Where("user_id = ?", userID).Find(&tokens).Error
	return tokens, err
}

// Update updates a refresh token
func (r *refreshTokenRepository) Update(token *models.RefreshToken) error {
	return r.db.Save(token).Error
}

// Delete deletes a refresh token by ID
func (r *refreshTokenRepository) Delete(id int) error {
	return r.db.Delete(&models.RefreshToken{}, id).Error
}

// RevokeAllUserTokens revokes all refresh tokens for a user
func (r *refreshTokenRepository) RevokeAllUserTokens(userID int) error {
	return r.db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND is_revoked = ?", userID, false).
		Update("is_revoked", true).Error
}

// CleanupExpiredTokens removes expired refresh tokens
func (r *refreshTokenRepository) CleanupExpiredTokens() error {
	return r.db.Where("expires_at < NOW() OR is_revoked = ?", true).
		Delete(&models.RefreshToken{}).Error
}
