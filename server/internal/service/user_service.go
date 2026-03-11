package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5"
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
	Email       *string
	DisplayName *string
	AvatarURL   *string
	Role        *UserRole
	IsActive    *bool
}

type UserService interface {
	GetUserByID(ctx context.Context, userID int) (UserResponse, error)
	ListUsers(ctx context.Context, limit, offset int) (UserListResult, error)
	UpdateOwnProfile(ctx context.Context, userID int, input UpdateOwnProfileInput) (UserResponse, error)
	AdminUpdateUser(ctx context.Context, actorUserID, targetUserID int, input AdminUpdateUserInput) (UserResponse, error)
}

type userService struct {
	queries *repo.Queries
}

func NewUserService(queries *repo.Queries) UserService {
	return &userService{queries: queries}
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
				UserID:      row.UserID,
				Username:    row.Username,
				Email:       row.Email,
				Password:    row.Password,
				CreatedAt:   row.CreatedAt,
				UpdatedAt:   row.UpdatedAt,
				IsActive:    row.IsActive,
				LastLogin:   row.LastLogin,
				DisplayName: row.DisplayName,
				AvatarUrl:   row.AvatarUrl,
				Role:        row.Role,
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
		displayName = strings.TrimSpace(*input.DisplayName)
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
		username = strings.TrimSpace(*input.Username)
	}
	email := strings.TrimSpace(target.Email)
	if input.Email != nil {
		email = strings.TrimSpace(*input.Email)
	}
	displayName := strings.TrimSpace(target.DisplayName)
	if input.DisplayName != nil {
		displayName = strings.TrimSpace(*input.DisplayName)
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

	if username == "" || email == "" {
		return UserResponse{}, errors.New("username and email are required")
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

	if email != target.Email {
		existing, err := s.queries.GetUserByEmail(ctx, email)
		if err == nil && existing.UserID != target.UserID {
			return UserResponse{}, ErrUserAlreadyExists
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return UserResponse{}, fmt.Errorf("check email availability: %w", err)
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
		Email:       email,
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
