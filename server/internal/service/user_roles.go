package service

import "strings"

type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
	UserRoleUser  UserRole = "user"
)

const (
	PermissionManageUsers      = "manage_users"
	PermissionManageSettings   = "manage_settings"
	PermissionViewAllAssets    = "view_all_assets"
	PermissionManageAllAssets  = "manage_all_assets"
	PermissionViewOwnAssets    = "view_own_assets"
	PermissionManageOwnAssets  = "manage_own_assets"
	PermissionManageOwnProfile = "manage_own_profile"
)

func normalizeUserRole(value string) UserRole {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(UserRoleAdmin):
		return UserRoleAdmin
	default:
		return UserRoleUser
	}
}

func IsAdminRole(value string) bool {
	return normalizeUserRole(value) == UserRoleAdmin
}

func PermissionsForRole(role UserRole) []string {
	switch role {
	case UserRoleAdmin:
		return []string{
			PermissionManageUsers,
			PermissionManageSettings,
			PermissionViewAllAssets,
			PermissionManageAllAssets,
			PermissionManageOwnProfile,
		}
	default:
		return []string{
			PermissionViewOwnAssets,
			PermissionManageOwnAssets,
			PermissionManageOwnProfile,
		}
	}
}
