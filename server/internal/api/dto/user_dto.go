package dto

import (
	"strings"

	"server/internal/service"
)

type UpdateOwnProfileRequestDTO struct {
	DisplayName *string `json:"display_name,omitempty" example:"Alex Chen"`
	AvatarURL   *string `json:"avatar_url,omitempty" example:"https://example.com/avatar.jpg"`
}

type AdminUpdateUserRequestDTO struct {
	Username    *string `json:"username,omitempty" example:"alex"`
	DisplayName *string `json:"display_name,omitempty" example:"Alex Chen"`
	AvatarURL   *string `json:"avatar_url,omitempty" example:"https://example.com/avatar.jpg"`
	Role        *string `json:"role,omitempty" enums:"admin,user" example:"admin"`
	IsActive    *bool   `json:"is_active,omitempty" example:"true"`
}

type ChangePasswordRequestDTO struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=6"`
}

type ResetAccessResponseDTO struct {
	TemporaryPassword string `json:"temporary_password"`
	ClearedPasskeys   bool   `json:"cleared_passkeys"`
	ClearedTOTP       bool   `json:"cleared_totp"`
}

type ManagedUserDTO struct {
	UserDTO
	AssetCount int64 `json:"asset_count"`
	AlbumCount int64 `json:"album_count"`
}

type ListUsersResponseDTO struct {
	Users  []ManagedUserDTO `json:"users"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

func ToUserDTO(user service.UserResponse) UserDTO {
	displayName := strings.TrimSpace(user.DisplayName)
	if displayName == "" {
		displayName = user.Username
	}

	return UserDTO{
		UserID:      user.UserID,
		Username:    user.Username,
		DisplayName: displayName,
		AvatarURL:   user.AvatarURL,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		IsActive:    user.IsActive,
		LastLogin:   user.LastLogin,
		Role:        user.Role,
		Permissions: append([]string(nil), user.Permissions...),
	}
}

func ToManagedUserDTO(user service.ManagedUser) ManagedUserDTO {
	return ManagedUserDTO{
		UserDTO:    ToUserDTO(user.UserResponse),
		AssetCount: user.AssetCount,
		AlbumCount: user.AlbumCount,
	}
}

func ToListUsersResponseDTO(result service.UserListResult) ListUsersResponseDTO {
	items := make([]ManagedUserDTO, 0, len(result.Users))
	for _, user := range result.Users {
		items = append(items, ToManagedUserDTO(user))
	}

	return ListUsersResponseDTO{
		Users:  items,
		Total:  int(result.Total),
		Limit:  result.Limit,
		Offset: result.Offset,
	}
}

func ToResetAccessResponseDTO(result service.ResetAccessResult) ResetAccessResponseDTO {
	return ResetAccessResponseDTO{
		TemporaryPassword: result.TemporaryPassword,
		ClearedPasskeys:   result.ClearedPasskeys,
		ClearedTOTP:       result.ClearedTOTP,
	}
}
