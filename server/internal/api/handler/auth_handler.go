package handler

import (
	"errors"
	"net/http"
	"strings"

	"server/internal/api"
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

// Register handles user registration
// @Summary Register a new user
// @Description Create a new user account with username, email, and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.RegisterRequest true "Registration data"
// @Success 201 {object} api.Result{data=models.AuthResponse} "User registered successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 409 {object} api.Result "User already exists"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /auth/register [post]
func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.Error(c.Writer, 400, err, http.StatusBadRequest, "Invalid request data")
		return
	}

	authResponse, err := h.authService.Register(req)
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			api.Error(c.Writer, 409, err, http.StatusConflict, "User already exists")
			return
		}
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to register user")
		return
	}

	api.Success(c.Writer, authResponse)
}

// Login handles user authentication
// @Summary Login user
// @Description Authenticate user with username and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Login credentials"
// @Success 200 {object} api.Result{data=models.AuthResponse} "Login successful"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Invalid credentials"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.Error(c.Writer, 400, err, http.StatusBadRequest, "Invalid request data")
		return
	}

	authResponse, err := h.authService.Login(req)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) || errors.Is(err, service.ErrInvalidPassword) {
			api.Error(c.Writer, 401, errors.New("Username or password is incorrect"), http.StatusUnauthorized, "Invalid credentials")
			return
		}
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to login")
		return
	}

	api.Success(c.Writer, authResponse)
}

// RefreshToken handles JWT token refresh
// @Summary Refresh access token
// @Description Generate a new access token using a valid refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} api.Result{data=models.AuthResponse} "Token refreshed successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Invalid or expired refresh token"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req service.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.Error(c.Writer, 400, err, http.StatusBadRequest, "Invalid request data")
		return
	}

	authResponse, err := h.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrTokenNotFound) ||
			errors.Is(err, service.ErrInvalidToken) ||
			errors.Is(err, service.ErrExpiredToken) {
			api.Error(c.Writer, 401, err, http.StatusUnauthorized, "Invalid or expired refresh token")
			return
		}
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to refresh token")
		return
	}

	api.Success(c.Writer, authResponse)
}

// Logout handles user logout
// @Summary Logout user
// @Description Revoke the user's refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body models.RefreshTokenRequest true "Refresh token to revoke"
// @Success 200 {object} api.Result "Logout successful"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Invalid refresh token"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req service.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.Error(c.Writer, 400, err, http.StatusBadRequest, "Invalid request data")
		return
	}

	err := h.authService.RevokeRefreshToken(req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrTokenNotFound) {
			api.Error(c.Writer, 401, err, http.StatusUnauthorized, "Invalid refresh token")
			return
		}
		api.Error(c.Writer, http.StatusInternalServerError, err, http.StatusInternalServerError, "Failed to logout")
		return
	}

	api.Success(c.Writer, nil)
}

// Me returns the current authenticated user's information
// @Summary Get current user
// @Description Get information about the currently authenticated user
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.Result{data=models.UserResponse} "User information retrieved successfully"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	// Get user ID from JWT claims (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		api.Error(c.Writer, 401, errors.New("User ID not found in token"), http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Get user information
	// This would typically come from a user service
	user := service.UserResponse{
		UserID:   userID.(int),
		Username: c.GetString("username"),
		// Other fields would be fetched from database
	}

	api.Success(c.Writer, user)
}

// AuthMiddleware validates JWT tokens and sets user context
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			api.Error(c.Writer, 401, errors.New("Authorization header is required"), http.StatusUnauthorized, "Unauthorized")
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>" format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			api.Error(c.Writer, 401, errors.New("Invalid authorization header format"), http.StatusUnauthorized, "Unauthorized")
			c.Abort()
			return
		}

		token := tokenParts[1]
		claims, err := h.authService.ValidateToken(token)
		if err != nil {
			api.Error(c.Writer, 401, errors.New("Invalid or expired token"), http.StatusUnauthorized, "Unauthorized")
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

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

		// Extract token from "Bearer <token>" format
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.Next()
			return
		}

		token := tokenParts[1]
		claims, err := h.authService.ValidateToken(token)
		if err != nil {
			c.Next()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		c.Next()
	}
}
