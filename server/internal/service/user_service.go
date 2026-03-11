package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInsufficientPermissions = errors.New("insufficient permissions")
	ErrCannotDisableLastAdmin  = errors.New("cannot disable the last active admin")
)

type ManagedUser struct {
	UserResponse
	AssetCount int64 `json:"asset_count"`
	AlbumCount int64 `json:"album_count"`
}

type UserListResult struct {
	Users  []ManagedUser `json:"users"`
	Total  int64         `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type UpdateOwnProfileInput struct {
	DisplayName *string
	AvatarURL   *string
}

type AdminUpdateUserInput struct {
	Username    *string
	DisplayName *string
	AvatarURL   *string
	Role        *UserRole
	IsActive    *bool
}

type ChangePasswordInput struct {
	CurrentPassword string
	NewPassword     string
}

type ResetAccessResult struct {
	TemporaryPassword string `json:"temporary_password"`
	ClearedPasskeys   bool   `json:"cleared_passkeys"`
	ClearedTOTP       bool   `json:"cleared_totp"`
}

type UserService interface {
	GetUserByID(ctx context.Context, userID int) (UserResponse, error)
	ListUsers(ctx context.Context, limit, offset int) (UserListResult, error)
	UpdateOwnProfile(ctx context.Context, userID int, input UpdateOwnProfileInput) (UserResponse, error)
	AdminUpdateUser(ctx context.Context, actorUserID, targetUserID int, input AdminUpdateUserInput) (UserResponse, error)
	ChangePassword(ctx context.Context, userID int, input ChangePasswordInput) error
	AdminResetAccess(ctx context.Context, actorUserID, targetUserID int) (ResetAccessResult, error)
}

type userService struct {
	queries *repo.Queries
	db      *pgxpool.Pool
}

func NewUserService(queries *repo.Queries, db *pgxpool.Pool) UserService {
	return &userService{
		queries: queries,
		db:      db,
	}
}

func (s *userService) GetUserByID(ctx context.Context, userID int) (UserResponse, error) {
	user, err := s.queries.GetUserByID(ctx, int32(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserResponse{}, ErrUserNotFound
		}
		return UserResponse{}, fmt.Errorf("get user by id: %w", err)
	}

	return ConvertUserToResponse(user), nil
}

func (s *userService) ListUsers(ctx context.Context, limit, offset int) (UserListResult, error) {
	total, err := s.queries.CountUsers(ctx)
	if err != nil {
		return UserListResult{}, fmt.Errorf("count users: %w", err)
	}

	rows, err := s.queries.ListUsersWithStats(ctx, repo.ListUsersWithStatsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return UserListResult{}, fmt.Errorf("list users: %w", err)
	}

	items := make([]ManagedUser, 0, len(rows))
	for _, row := range rows {
		items = append(items, ManagedUser{
			UserResponse: ConvertUserToResponse(repo.User{
				UserID:             row.UserID,
				Username:           row.Username,
				Password:           row.Password,
				CreatedAt:          row.CreatedAt,
				UpdatedAt:          row.UpdatedAt,
				IsActive:           row.IsActive,
				LastLogin:          row.LastLogin,
				DisplayName:        row.DisplayName,
				AvatarUrl:          row.AvatarUrl,
				Role:               row.Role,
				WebauthnUserHandle: row.WebauthnUserHandle,
			}),
			AssetCount: row.AssetCount,
			AlbumCount: row.AlbumCount,
		})
	}

	return UserListResult{
		Users:  items,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *userService) UpdateOwnProfile(ctx context.Context, userID int, input UpdateOwnProfileInput) (UserResponse, error) {
	user, err := s.queries.GetUserByID(ctx, int32(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserResponse{}, ErrUserNotFound
		}
		return UserResponse{}, fmt.Errorf("get user profile: %w", err)
	}

	displayName := user.DisplayName
	if input.DisplayName != nil {
		normalizedDisplayName, err := normalizeDisplayName(*input.DisplayName)
		if err != nil {
			return UserResponse{}, err
		}
		displayName = normalizedDisplayName
	}

	avatarURL := cloneOptionalString(user.AvatarUrl)
	if input.AvatarURL != nil {
		avatarURL = normalizeOptionalString(*input.AvatarURL)
	}

	updated, err := s.queries.UpdateUserProfile(ctx, repo.UpdateUserProfileParams{
		UserID:      user.UserID,
		DisplayName: displayName,
		AvatarUrl:   avatarURL,
	})
	if err != nil {
		return UserResponse{}, fmt.Errorf("update user profile: %w", err)
	}

	return ConvertUserToResponse(updated), nil
}

func (s *userService) AdminUpdateUser(ctx context.Context, actorUserID, targetUserID int, input AdminUpdateUserInput) (UserResponse, error) {
	actor, err := s.queries.GetUserByID(ctx, int32(actorUserID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserResponse{}, ErrUserNotFound
		}
		return UserResponse{}, fmt.Errorf("get actor user: %w", err)
	}
	if !IsAdminRole(actor.Role) {
		return UserResponse{}, ErrInsufficientPermissions
	}

	target, err := s.queries.GetUserByID(ctx, int32(targetUserID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return UserResponse{}, ErrUserNotFound
		}
		return UserResponse{}, fmt.Errorf("get target user: %w", err)
	}

	username := strings.TrimSpace(target.Username)
	if input.Username != nil {
		normalizedUsername, err := normalizeUsername(*input.Username)
		if err != nil {
			return UserResponse{}, err
		}
		username = normalizedUsername
	}
	displayName := strings.TrimSpace(target.DisplayName)
	if input.DisplayName != nil {
		normalizedDisplayName, err := normalizeDisplayName(*input.DisplayName)
		if err != nil {
			return UserResponse{}, err
		}
		displayName = normalizedDisplayName
	}
	avatarURL := cloneOptionalString(target.AvatarUrl)
	if input.AvatarURL != nil {
		avatarURL = normalizeOptionalString(*input.AvatarURL)
	}
	role := normalizeUserRole(target.Role)
	if input.Role != nil {
		role = *input.Role
	}
	isActive := target.IsActive != nil && *target.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	if username != target.Username {
		existing, err := s.queries.GetUserByUsername(ctx, username)
		if err == nil && existing.UserID != target.UserID {
			return UserResponse{}, ErrUserAlreadyExists
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return UserResponse{}, fmt.Errorf("check username availability: %w", err)
		}
	}

	if IsAdminRole(target.Role) && (!isActive || role != UserRoleAdmin) {
		activeAdmins, err := s.queries.CountActiveUsersByRole(ctx, string(UserRoleAdmin))
		if err != nil {
			return UserResponse{}, fmt.Errorf("count active admins: %w", err)
		}
		if activeAdmins <= 1 && target.IsActive != nil && *target.IsActive {
			return UserResponse{}, ErrCannotDisableLastAdmin
		}
	}

	updated, err := s.queries.AdminUpdateUser(ctx, repo.AdminUpdateUserParams{
		UserID:      target.UserID,
		Username:    username,
		DisplayName: displayName,
		AvatarUrl:   avatarURL,
		Role:        string(role),
		IsActive:    &isActive,
	})
	if err != nil {
		return UserResponse{}, fmt.Errorf("admin update user: %w", err)
	}

	return ConvertUserToResponse(updated), nil
}

func (s *userService) ChangePassword(ctx context.Context, userID int, input ChangePasswordInput) error {
	user, err := s.queries.GetUserByID(ctx, int32(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("get user for password change: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.CurrentPassword)); err != nil {
		return ErrInvalidCurrentSecret
	}

	passwordHash, err := bcryptPassword(input.NewPassword)
	if err != nil {
		return err
	}

	return s.withTx(ctx, func(q *repo.Queries) error {
		if err := q.UpdateUserPassword(ctx, repo.UpdateUserPasswordParams{
			UserID:   user.UserID,
			Password: passwordHash,
		}); err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		if err := q.RevokeUserRefreshTokens(ctx, user.UserID); err != nil {
			return fmt.Errorf("revoke refresh tokens: %w", err)
		}
		return nil
	})
}

func (s *userService) AdminResetAccess(ctx context.Context, actorUserID, targetUserID int) (ResetAccessResult, error) {
	actor, err := s.queries.GetUserByID(ctx, int32(actorUserID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResetAccessResult{}, ErrUserNotFound
		}
		return ResetAccessResult{}, fmt.Errorf("get actor: %w", err)
	}
	if !IsAdminRole(actor.Role) {
		return ResetAccessResult{}, ErrInsufficientPermissions
	}

	target, err := s.queries.GetUserByID(ctx, int32(targetUserID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ResetAccessResult{}, ErrUserNotFound
		}
		return ResetAccessResult{}, fmt.Errorf("get target: %w", err)
	}

	temporaryPassword, err := generateTemporaryPassword()
	if err != nil {
		return ResetAccessResult{}, fmt.Errorf("generate temporary password: %w", err)
	}
	passwordHash, err := bcryptPassword(temporaryPassword)
	if err != nil {
		return ResetAccessResult{}, err
	}

	if err := s.withTx(ctx, func(q *repo.Queries) error {
		if err := q.UpdateUserPassword(ctx, repo.UpdateUserPasswordParams{
			UserID:   target.UserID,
			Password: passwordHash,
		}); err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		if err := q.DeleteUserWebAuthnCredentials(ctx, target.UserID); err != nil {
			return fmt.Errorf("delete passkeys: %w", err)
		}
		if err := q.DeleteUserTOTPCredential(ctx, target.UserID); err != nil {
			return fmt.Errorf("delete totp credential: %w", err)
		}
		if err := q.DeleteUserRecoveryCodes(ctx, target.UserID); err != nil {
			return fmt.Errorf("delete recovery codes: %w", err)
		}
		if err := q.RevokeUserRefreshTokens(ctx, target.UserID); err != nil {
			return fmt.Errorf("revoke refresh tokens: %w", err)
		}
		return nil
	}); err != nil {
		return ResetAccessResult{}, err
	}

	return ResetAccessResult{
		TemporaryPassword: temporaryPassword,
		ClearedPasskeys:   true,
		ClearedTOTP:       true,
	}, nil
}

func normalizeOptionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func cloneOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func (s *userService) withTx(ctx context.Context, fn func(*repo.Queries) error) error {
	if s.db == nil {
		return fn(s.queries)
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(s.queries.WithTx(tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func generateTemporaryPassword() (string, error) {
	buffer := make([]byte, 18)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	token := "Lm9" + base64.RawURLEncoding.EncodeToString(buffer)
	if len(token) > 24 {
		token = token[:24]
	}
	return token, nil
}
