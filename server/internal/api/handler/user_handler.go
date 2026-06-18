package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// UpdateMyProfile updates the authenticated user's profile.
// @Summary Update my profile
// @Description Update the current user's profile fields such as display name and avatar photo.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateOwnProfileRequestDTO true "Profile update payload"
// @Success 200 {object} dto.UserDTO "Profile updated successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request data"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/users/me/profile [patch]
func (h *UserHandler) UpdateMyProfile(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	var req dto.UpdateOwnProfileRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	updated, err := h.userService.UpdateOwnProfile(c.Request.Context(), user.UserID, service.UpdateOwnProfileInput{
		DisplayName:   req.DisplayName,
		AvatarAssetID: req.AvatarAssetID,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidDisplayName) ||
			errors.Is(err, service.ErrInvalidAvatarAsset) ||
			errors.Is(err, service.ErrAvatarAssetMustBePhoto) {
			api.GinBadRequest(c, err, err.Error())
			return
		}
		api.GinInternalError(c, err, "Failed to update profile")
		return
	}

	c.Set("current_user", &updated)
	c.Set("user_id", updated.UserID)
	c.Set("username", updated.Username)
	c.Set("user_role", updated.Role)
	c.Set("user_permissions", updated.Permissions)

	api.JSONOK(c, dto.ToUserDTO(updated))
}

// ListUsers returns paginated users for administrators.
// @Summary List users
// @Description List users with ownership statistics for administrator management views.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Maximum number of results" default(20)
// @Param offset query int false "Number of results to skip" default(0)
// @Success 200 {object} dto.ListUsersResponseDTO "Users retrieved successfully"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 403 {object} api.ErrorResponse "Forbidden"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/users [get]
func (h *UserHandler) ListUsers(c *gin.Context) {
	if _, ok := requireAdminUser(c); !ok {
		return
	}

	limit := 20
	offset := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	result, err := h.userService.ListUsers(c.Request.Context(), limit, offset)
	if err != nil {
		api.GinInternalError(c, err, "Failed to load users")
		return
	}

	api.JSONOK(c, dto.ToListUsersResponseDTO(result))
}

// UpdateUser updates a user as an administrator.
// @Summary Update user
// @Description Update user identity, role, status, and avatar fields as an administrator.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Param request body dto.AdminUpdateUserRequestDTO true "User update payload"
// @Success 200 {object} dto.UserDTO "User updated successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request data"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 403 {object} api.ErrorResponse "Forbidden"
// @Failure 404 {object} api.ErrorResponse "User not found"
// @Failure 409 {object} api.ErrorResponse "User already exists"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/users/{id} [patch]
func (h *UserHandler) UpdateUser(c *gin.Context) {
	admin, ok := requireAdminUser(c)
	if !ok {
		return
	}

	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid user ID")
		return
	}

	var req dto.AdminUpdateUserRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	var role *service.UserRole
	if req.Role != nil {
		normalized := service.UserRole(strings.ToLower(strings.TrimSpace(*req.Role)))
		if normalized != service.UserRoleAdmin && normalized != service.UserRoleUser {
			api.GinBadRequest(c, errors.New("invalid role"), "Role must be admin or user")
			return
		}
		role = &normalized
	}

	updated, err := h.userService.AdminUpdateUser(c.Request.Context(), admin.UserID, userID, service.AdminUpdateUserInput{
		Username:      req.Username,
		DisplayName:   req.DisplayName,
		AvatarAssetID: req.AvatarAssetID,
		Role:          role,
		IsActive:      req.IsActive,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidUsernameFormat),
			errors.Is(err, service.ErrInvalidDisplayName):
			api.GinBadRequest(c, err, err.Error())
			return
		case errors.Is(err, service.ErrUserNotFound):
			api.GinNotFound(c, err, "User not found")
		case errors.Is(err, service.ErrUserAlreadyExists):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "User already exists")
		case errors.Is(err, service.ErrInsufficientPermissions):
			api.GinForbidden(c, err, "Admin access required")
		case errors.Is(err, service.ErrCannotDisableLastAdmin):
			api.GinBadRequest(c, err, "At least one active admin must remain")
		default:
			api.GinInternalError(c, err, "Failed to update user")
		}
		return
	}

	api.JSONOK(c, dto.ToUserDTO(updated))
}

// ChangeMyPassword rotates the authenticated user's password and revokes all refresh tokens.
// @Summary Change my password
// @Description Verify the current password, set a new password, and revoke all refresh tokens for the current user.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.ChangePasswordRequestDTO true "Password change payload"
// @Success 200 {object} api.SuccessResponse "Password updated successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request data"
// @Failure 401 {object} api.ErrorResponse "Current password is incorrect"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/users/me/password [patch]
func (h *UserHandler) ChangeMyPassword(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	var req dto.ChangePasswordRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	if err := h.userService.ChangePassword(c.Request.Context(), user.UserID, service.ChangePasswordInput{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}); err != nil {
		switch {
		case errors.Is(err, service.ErrWeakPassword):
			api.GinBadRequest(c, err, err.Error())
			return
		case errors.Is(err, service.ErrInvalidCurrentSecret):
			api.GinUnauthorized(c, err, "Current password is incorrect")
		default:
			api.GinInternalError(c, err, "Failed to change password")
		}
		return
	}

	api.JSONOK(c, gin.H{"password_changed": true})
}

// ResetUserAccess resets password and clears MFA factors for a target user.
// @Summary Reset user access
// @Description Generate a temporary password and clear passkeys, TOTP, recovery codes, and refresh tokens for a user.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "User ID"
// @Success 200 {object} dto.ResetAccessResponseDTO "User access reset successfully"
// @Failure 400 {object} api.ErrorResponse "Invalid request"
// @Failure 401 {object} api.ErrorResponse "Unauthorized"
// @Failure 403 {object} api.ErrorResponse "Forbidden"
// @Failure 404 {object} api.ErrorResponse "User not found"
// @Failure 500 {object} api.ErrorResponse "Internal server error"
// @Router /api/v1/users/{id}/reset-access [post]
func (h *UserHandler) ResetUserAccess(c *gin.Context) {
	admin, ok := requireAdminUser(c)
	if !ok {
		return
	}

	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid user ID")
		return
	}

	result, err := h.userService.AdminResetAccess(c.Request.Context(), admin.UserID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			api.GinNotFound(c, err, "User not found")
		case errors.Is(err, service.ErrInsufficientPermissions):
			api.GinForbidden(c, err, "Admin access required")
		default:
			api.GinInternalError(c, err, "Failed to reset access")
		}
		return
	}

	api.JSONOK(c, dto.ToResetAccessResponseDTO(result))
}
