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
// @Description Update the current user's profile fields such as display name and avatar URL.
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.UpdateOwnProfileRequestDTO true "Profile update payload"
// @Success 200 {object} api.Result{data=dto.UserDTO} "Profile updated successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
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
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		api.GinInternalError(c, err, "Failed to update profile")
		return
	}

	c.Set("current_user", &updated)
	c.Set("user_id", updated.UserID)
	c.Set("username", updated.Username)
	c.Set("user_role", updated.Role)
	c.Set("user_permissions", updated.Permissions)

	api.GinSuccess(c, dto.ToUserDTO(updated))
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
// @Success 200 {object} api.Result{data=dto.ListUsersResponseDTO} "Users retrieved successfully"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 403 {object} api.Result "Forbidden"
// @Failure 500 {object} api.Result "Internal server error"
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

	api.GinSuccess(c, dto.ToListUsersResponseDTO(result))
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
// @Success 200 {object} api.Result{data=dto.UserDTO} "User updated successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 403 {object} api.Result "Forbidden"
// @Failure 404 {object} api.Result "User not found"
// @Failure 409 {object} api.Result "User already exists"
// @Failure 500 {object} api.Result "Internal server error"
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
		Username:    req.Username,
		Email:       req.Email,
		DisplayName: req.DisplayName,
		AvatarURL:   req.AvatarURL,
		Role:        role,
		IsActive:    req.IsActive,
	})
	if err != nil {
		switch {
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

	api.GinSuccess(c, dto.ToUserDTO(updated))
}
