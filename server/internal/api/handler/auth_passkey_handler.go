package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

// BeginRegistrationPasskey creates WebAuthn creation options for a staged registration session.
// @Summary Begin passkey registration
// @Description Create WebAuthn registration options for a staged registration session.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegistrationSessionRequestDTO true "Registration session"
// @Success 200 {object} api.Result{data=dto.PasskeyOptionsResponseDTO} "Passkey registration options created successfully"
// @Failure 400 {object} api.Result "Invalid or expired registration session"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/passkeys/register/options [post]
func (h *AuthHandler) BeginRegistrationPasskey(c *gin.Context) {
	var req dto.RegistrationSessionRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	response, err := h.authService.BeginPasskeyRegistration(c.Request.Context(), req.RegistrationSessionID, requestOrigin(c))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRegistrationSessionNotFound),
			errors.Is(err, service.ErrRegistrationSessionExpired):
			api.GinBadRequest(c, err, "Registration session is invalid or expired")
		default:
			api.GinInternalError(c, err, "Failed to start passkey registration")
		}
		return
	}

	api.GinSuccess(c, dto.ToPasskeyOptionsResponseDTO(response))
}

// VerifyRegistrationPasskey finishes staged passkey registration and creates the user account.
// @Summary Verify passkey registration
// @Description Verify a staged registration passkey response, create the user, and issue session tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegistrationPasskeyVerifyRequestDTO true "Passkey registration verification payload"
// @Success 200 {object} api.Result{data=dto.AuthResponseDTO} "Passkey registration verified successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Invalid or expired passkey registration challenge"
// @Failure 409 {object} api.Result "User already exists"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/passkeys/register/verify [post]
func (h *AuthHandler) VerifyRegistrationPasskey(c *gin.Context) {
	var req dto.RegistrationPasskeyVerifyRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	credentialJSON, err := json.Marshal(req.Credential)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid passkey credential payload")
		return
	}

	response, err := h.authService.VerifyPasskeyRegistration(c.Request.Context(), req.RegistrationSessionID, req.ChallengeToken, credentialJSON)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserAlreadyExists):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "User already exists")
		case errors.Is(err, service.ErrRegistrationSessionNotFound),
			errors.Is(err, service.ErrRegistrationSessionExpired),
			errors.Is(err, service.ErrInvalidPasskeyChallenge),
			errors.Is(err, service.ErrExpiredPasskeyChallenge):
			api.GinUnauthorized(c, err, "Invalid or expired passkey registration challenge")
		default:
			api.GinInternalError(c, err, "Failed to verify passkey registration")
		}
		return
	}

	api.GinSuccess(c, dto.ToAuthResponseDTO(response))
}

// BeginRegistrationTOTPSetup creates TOTP setup data for a staged registration session.
// @Summary Begin staged TOTP setup
// @Description Generate TOTP setup data for a staged registration session.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegistrationSessionRequestDTO true "Registration session"
// @Success 200 {object} api.Result{data=dto.RegistrationTOTPSetupResponseDTO} "TOTP setup created successfully"
// @Failure 400 {object} api.Result "Invalid or expired registration session"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/register/totp/setup [post]
func (h *AuthHandler) BeginRegistrationTOTPSetup(c *gin.Context) {
	var req dto.RegistrationSessionRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	response, err := h.authService.BeginRegistrationTOTPSetup(c.Request.Context(), req.RegistrationSessionID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRegistrationSessionNotFound),
			errors.Is(err, service.ErrRegistrationSessionExpired):
			api.GinBadRequest(c, err, "Registration session is invalid or expired")
		default:
			api.GinInternalError(c, err, "Failed to start TOTP setup")
		}
		return
	}

	api.GinSuccess(c, dto.ToRegistrationTOTPSetupResponseDTO(response))
}

// CompleteRegistrationTOTP finishes staged TOTP registration and creates the user account.
// @Summary Complete staged TOTP registration
// @Description Verify the staged TOTP code, create the user account, and issue session tokens plus recovery codes.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.RegistrationTOTPCompleteRequestDTO true "TOTP verification payload"
// @Success 200 {object} api.Result{data=dto.RegistrationTOTPCompleteResponseDTO} "TOTP registration completed successfully"
// @Failure 400 {object} api.Result "Invalid or expired registration session"
// @Failure 409 {object} api.Result "User already exists"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/register/totp/complete [post]
func (h *AuthHandler) CompleteRegistrationTOTP(c *gin.Context) {
	var req dto.RegistrationTOTPCompleteRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	response, err := h.authService.CompleteRegistrationTOTP(c.Request.Context(), req.RegistrationSessionID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidMFACode),
			errors.Is(err, service.ErrRegistrationTOTPNotPrepared):
			api.GinBadRequest(c, err, "Invalid TOTP setup or verification code")
		case errors.Is(err, service.ErrRegistrationSessionNotFound),
			errors.Is(err, service.ErrRegistrationSessionExpired):
			api.GinBadRequest(c, err, "Registration session is invalid or expired")
		case errors.Is(err, service.ErrUserAlreadyExists):
			api.GinError(c, http.StatusConflict, err, http.StatusConflict, "User already exists")
		default:
			api.GinInternalError(c, err, "Failed to complete TOTP registration")
		}
		return
	}

	api.GinSuccess(c, dto.ToRegistrationTOTPCompleteResponseDTO(response))
}

// BeginPasskeyLogin creates WebAuthn request options for username-first passkey login.
// @Summary Begin passkey login
// @Description Create WebAuthn login options for a username-first passkey login flow.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.PasskeyOptionsRequestDTO true "Username for passkey login"
// @Success 200 {object} api.Result{data=dto.PasskeyOptionsResponseDTO} "Passkey login options created successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Invalid credentials"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/passkeys/login/options [post]
func (h *AuthHandler) BeginPasskeyLogin(c *gin.Context) {
	var req dto.PasskeyOptionsRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		api.GinBadRequest(c, errors.New("username is required"), "Invalid request data")
		return
	}

	response, err := h.authService.BeginPasskeyLogin(c.Request.Context(), req.Username, requestOrigin(c))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPasskeyNotConfigured),
			errors.Is(err, service.ErrUserNotFound):
			api.GinUnauthorized(c, errors.New("Username or passkey is incorrect"), "Invalid credentials")
		default:
			api.GinInternalError(c, err, "Failed to start passkey login")
		}
		return
	}

	api.GinSuccess(c, dto.ToPasskeyOptionsResponseDTO(response))
}

// VerifyPasskeyLogin completes username-first passkey login.
// @Summary Verify passkey login
// @Description Verify a passkey login assertion and issue session tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.PasskeyVerifyRequestDTO true "Passkey login verification payload"
// @Success 200 {object} api.Result{data=dto.AuthResponseDTO} "Passkey login verified successfully"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Invalid credentials"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/passkeys/login/verify [post]
func (h *AuthHandler) VerifyPasskeyLogin(c *gin.Context) {
	var req dto.PasskeyVerifyRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	credentialJSON, err := json.Marshal(req.Credential)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid passkey credential payload")
		return
	}

	response, err := h.authService.VerifyPasskeyLogin(c.Request.Context(), req.ChallengeToken, credentialJSON)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPasskeyChallenge),
			errors.Is(err, service.ErrExpiredPasskeyChallenge),
			errors.Is(err, service.ErrPasskeyNotConfigured),
			errors.Is(err, service.ErrUserNotFound):
			api.GinUnauthorized(c, errors.New("Passkey verification failed"), "Invalid credentials")
		default:
			api.GinInternalError(c, err, "Failed to verify passkey login")
		}
		return
	}

	api.GinSuccess(c, dto.ToAuthResponseDTO(response))
}

// ListPasskeys returns the authenticated user's passkeys.
// @Summary List passkeys
// @Description List the authenticated user's enrolled passkeys.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.Result{data=dto.PasskeyListResponseDTO} "Passkeys retrieved successfully"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/mfa/passkeys [get]
func (h *AuthHandler) ListPasskeys(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	response, err := h.authService.ListPasskeys(c.Request.Context(), user.UserID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to load passkeys")
		return
	}

	api.GinSuccess(c, dto.ToPasskeyListResponseDTO(response))
}

// BeginPasskeyEnrollment creates WebAuthn registration options for the authenticated user.
// @Summary Begin passkey enrollment
// @Description Create WebAuthn registration options to add a new passkey to the authenticated account.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.Result{data=dto.PasskeyOptionsResponseDTO} "Passkey enrollment options created successfully"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/mfa/passkeys/options [post]
func (h *AuthHandler) BeginPasskeyEnrollment(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	response, err := h.authService.BeginPasskeyEnrollment(c.Request.Context(), user.UserID, requestOrigin(c))
	if err != nil {
		api.GinInternalError(c, err, "Failed to start passkey enrollment")
		return
	}

	api.GinSuccess(c, dto.ToPasskeyOptionsResponseDTO(response))
}

// VerifyPasskeyEnrollment completes passkey enrollment for the authenticated user.
// @Summary Verify passkey enrollment
// @Description Verify a passkey enrollment response and attach the new passkey to the authenticated account.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.PasskeyVerifyRequestDTO true "Passkey enrollment verification payload"
// @Success 200 {object} api.Result{data=dto.PasskeyCredentialSummaryDTO} "Passkey enrolled successfully"
// @Failure 400 {object} api.Result "Invalid or expired challenge"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/mfa/passkeys/verify [post]
func (h *AuthHandler) VerifyPasskeyEnrollment(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	var req dto.PasskeyVerifyRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	credentialJSON, err := json.Marshal(req.Credential)
	if err != nil {
		api.GinBadRequest(c, err, "Invalid passkey credential payload")
		return
	}

	response, err := h.authService.VerifyPasskeyEnrollment(c.Request.Context(), user.UserID, req.ChallengeToken, credentialJSON)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPasskeyChallenge),
			errors.Is(err, service.ErrExpiredPasskeyChallenge):
			api.GinBadRequest(c, err, "Invalid or expired passkey challenge")
		default:
			api.GinInternalError(c, err, "Failed to enroll passkey")
		}
		return
	}

	api.GinSuccess(c, dto.ToPasskeyCredentialSummaryDTO(response))
}

// DeletePasskey removes an enrolled passkey from the authenticated user.
// @Summary Delete passkey
// @Description Delete one enrolled passkey for the authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Passkey ID"
// @Success 200 {object} api.Result "Passkey deleted successfully"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 404 {object} api.Result "Passkey not found"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/mfa/passkeys/{id} [delete]
func (h *AuthHandler) DeletePasskey(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	passkeyID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		api.GinBadRequest(c, err, "Invalid passkey ID")
		return
	}

	if err := h.authService.DeletePasskey(c.Request.Context(), user.UserID, passkeyID); err != nil {
		switch {
		case errors.Is(err, service.ErrPasskeyCredentialNotFound):
			api.GinNotFound(c, err, "Passkey not found")
		default:
			api.GinInternalError(c, err, "Failed to delete passkey")
		}
		return
	}

	api.GinSuccess(c, gin.H{"deleted": true})
}

func requestOrigin(c *gin.Context) string {
	if origin := strings.TrimSpace(c.GetHeader("Origin")); origin != "" {
		return origin
	}

	proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if proto == "" {
		if c.Request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}

	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" {
		host = c.Request.Host
	}

	return fmt.Sprintf("%s://%s", proto, host)
}
