package handler

import (
	"errors"
	"net/http"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// StartRegistration creates a staged registration session.
// @Summary Start user registration
// @Description Create a staged registration session with username and password before passkey or TOTP enrollment.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegistrationStartRequestDTO true "Registration data"
// @Success 200 {object} api.Result{data=dto.RegistrationStartResponseDTO} "Registration session created successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 409 {object} api.Result "User already exists"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/register/start [post]
func (h *AuthHandler) StartRegistration(c *gin.Context) {
	var req dto.RegistrationStartRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	response, err := h.authService.StartRegistration(c.Request.Context(), service.RegistrationStartRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidUsernameFormat),
			errors.Is(err, service.ErrWeakPassword):
			api.GinBadRequest(c, err, err.Error())
			return
		case errors.Is(err, service.ErrUserAlreadyExists):
			api.GinError(c, 409, err, http.StatusConflict, "User already exists")
			return
		}
		api.GinInternalError(c, err, "Failed to register user")
		return
	}

	api.GinSuccess(c, dto.ToRegistrationStartResponseDTO(response))
}

// GetBootstrapStatus reports whether the system is still waiting for its first account.
// @Summary Get auth bootstrap status
// @Description Return whether Lumilio is still in first-user bootstrap mode and which role the next registration receives.
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {object} api.Result{data=dto.BootstrapStatusDTO} "Bootstrap status retrieved successfully"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/bootstrap-status [get]
func (h *AuthHandler) GetBootstrapStatus(c *gin.Context) {
	status, err := h.authService.GetBootstrapStatus(c.Request.Context())
	if err != nil {
		api.GinInternalError(c, err, "Failed to load bootstrap status")
		return
	}

	api.GinSuccess(c, dto.ToBootstrapStatusDTO(status))
}

// Login handles user authentication
// @Summary Login user
// @Description Authenticate user with username and password. Returns an MFA challenge instead of session tokens when TOTP is enabled.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequestDTO true "Login credentials"
// @Success 200 {object} api.Result{data=dto.AuthResponseDTO} "Login successful"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Invalid credentials"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	authResponse, err := h.authService.Login(service.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) || errors.Is(err, service.ErrInvalidPassword) {
			api.GinUnauthorized(c, errors.New("Username or password is incorrect"), "Invalid credentials")
			return
		}
		api.GinInternalError(c, err, "Failed to login")
		return
	}

	api.GinSuccess(c, dto.ToAuthResponseDTO(authResponse))
}

// RefreshToken handles JWT token refresh
// @Summary Refresh access token
// @Description Generate a new access token using a valid refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequestDTO true "Refresh token"
// @Success 200 {object} api.Result{data=dto.AuthResponseDTO} "Token refreshed successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Invalid or expired refresh token"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	authResponse, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrTokenNotFound) ||
			errors.Is(err, service.ErrInvalidToken) ||
			errors.Is(err, service.ErrExpiredToken) {
			api.GinUnauthorized(c, err, "Invalid or expired refresh token")
			return
		}
		api.GinInternalError(c, err, "Failed to refresh token")
		return
	}

	api.GinSuccess(c, dto.ToAuthResponseDTO(authResponse))
}

// Logout handles user logout
// @Summary Logout user
// @Description Revoke the user's refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequestDTO true "Refresh token to revoke"
// @Success 200 {object} api.Result "Logout successful"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Invalid refresh token"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.RefreshTokenRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	err := h.authService.RevokeRefreshToken(req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrTokenNotFound) {
			api.GinUnauthorized(c, err, "Invalid refresh token")
			return
		}
		api.GinInternalError(c, err, "Failed to logout")
		return
	}

	api.GinSuccess(c, nil)
}

// Me returns the current authenticated user's information
// @Summary Get current user
// @Description Get information about the currently authenticated user
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.Result{data=dto.UserDTO} "User information retrieved successfully"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	api.GinSuccess(c, dto.ToUserDTO(*user))
}

// AuthMiddleware validates JWT tokens and sets user context
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := h.authenticateRequest(c)
		if err != nil {
			api.GinUnauthorized(c, errors.New("Invalid or expired token"), "Unauthorized")
			c.Abort()
			return
		}

		h.setUserContext(c, user)
		c.Next()
	}
}

// OptionalAuthMiddleware validates JWT tokens if present but doesn't require them
func (h *AuthHandler) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		user, err := h.authenticateRequest(c)
		if err != nil {
			c.Next()
			return
		}

		h.setUserContext(c, user)

		c.Next()
	}
}

func (h *AuthHandler) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := requireAdminUser(c); !ok {
			c.Abort()
			return
		}
		c.Next()
	}
}

func (h *AuthHandler) authenticateRequest(c *gin.Context) (*service.UserResponse, error) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return nil, errors.New("authorization header is required")
	}

	tokenParts := strings.Split(authHeader, " ")
	if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
		return nil, errors.New("invalid authorization header format")
	}

	claims, err := h.authService.ValidateToken(tokenParts[1])
	if err != nil {
		return nil, err
	}

	return h.authService.GetCurrentUser(claims.UserID)
}

func (h *AuthHandler) setUserContext(c *gin.Context, user *service.UserResponse) {
	c.Set("current_user", user)
	c.Set("user_id", user.UserID)
	c.Set("username", user.Username)
	c.Set("user_role", user.Role)
	c.Set("user_permissions", user.Permissions)
}
