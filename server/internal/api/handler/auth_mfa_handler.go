package handler

import (
	"errors"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

// VerifyMFA completes a pending login after the password step.
// @Summary Verify MFA challenge
// @Description Verify a pending MFA login challenge with a TOTP code or recovery code.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.VerifyMFARequestDTO true "MFA verification payload"
// @Success 200 {object} api.Result{data=dto.AuthResponseDTO} "MFA verification successful"
// @Failure 400 {object} api.Result "Invalid request data"
// @Failure 401 {object} api.Result "Invalid or expired MFA challenge"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/mfa/verify [post]
func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	var req dto.VerifyMFARequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	authResponse, err := h.authService.VerifyLoginMFA(c.Request.Context(), service.VerifyMFARequest{
		MFAToken: req.MFAToken,
		Code:     req.Code,
		Method:   req.Method,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidMFAToken),
			errors.Is(err, service.ErrExpiredMFAToken),
			errors.Is(err, service.ErrInvalidMFACode),
			errors.Is(err, service.ErrUserNotFound),
			errors.Is(err, service.ErrMFANotEnabled):
			api.GinUnauthorized(c, err, "Invalid or expired MFA challenge")
		default:
			api.GinInternalError(c, err, "Failed to verify MFA challenge")
		}
		return
	}

	api.GinSuccess(c, dto.ToAuthResponseDTO(authResponse))
}

// GetMFAStatus returns the current user's MFA status.
// @Summary Get MFA status
// @Description Get the authenticated user's MFA status, including TOTP enablement and remaining recovery codes.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.Result{data=dto.MFAStatusDTO} "MFA status retrieved successfully"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/mfa [get]
func (h *AuthHandler) GetMFAStatus(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	status, err := h.authService.GetMFAStatus(c.Request.Context(), user.UserID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to load MFA status")
		return
	}

	api.GinSuccess(c, dto.ToMFAStatusDTO(status))
}

// BeginTOTPSetup starts a TOTP setup flow.
// @Summary Begin TOTP setup
// @Description Generate a new TOTP secret and setup token for the authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} api.Result{data=dto.TOTPSetupResponseDTO} "TOTP setup created successfully"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/mfa/totp/setup [post]
func (h *AuthHandler) BeginTOTPSetup(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	response, err := h.authService.BeginTOTPSetup(c.Request.Context(), user.UserID)
	if err != nil {
		api.GinInternalError(c, err, "Failed to start TOTP setup")
		return
	}

	api.GinSuccess(c, dto.ToTOTPSetupResponseDTO(response))
}

// EnableTOTP completes TOTP enrollment.
// @Summary Enable TOTP
// @Description Verify a TOTP setup code and enable TOTP MFA for the authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.EnableTOTPRequestDTO true "TOTP enable payload"
// @Success 200 {object} api.Result{data=dto.RecoveryCodesResponseDTO} "TOTP enabled successfully"
// @Failure 400 {object} api.Result "Invalid setup token or verification code"
// @Failure 401 {object} api.Result "Unauthorized"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/mfa/totp/enable [post]
func (h *AuthHandler) EnableTOTP(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	var req dto.EnableTOTPRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	response, err := h.authService.EnableTOTP(c.Request.Context(), user.UserID, service.EnableTOTPInput{
		SetupToken: req.SetupToken,
		Code:       req.Code,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidMFAToken),
			errors.Is(err, service.ErrExpiredMFAToken),
			errors.Is(err, service.ErrInvalidMFACode):
			api.GinBadRequest(c, err, "Invalid setup token or verification code")
		default:
			api.GinInternalError(c, err, "Failed to enable TOTP")
		}
		return
	}

	api.GinSuccess(c, dto.ToRecoveryCodesResponseDTO(response))
}

// DisableTOTP disables TOTP MFA for the authenticated user.
// @Summary Disable TOTP
// @Description Disable TOTP MFA and invalidate recovery codes for the authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.DisableTOTPRequestDTO true "Disable TOTP payload"
// @Success 200 {object} api.Result{data=dto.MFAStatusDTO} "TOTP disabled successfully"
// @Failure 400 {object} api.Result "MFA is not enabled"
// @Failure 401 {object} api.Result "Unauthorized or incorrect password"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/mfa/totp/disable [post]
func (h *AuthHandler) DisableTOTP(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	var req dto.DisableTOTPRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	status, err := h.authService.DisableTOTP(c.Request.Context(), user.UserID, req.CurrentPassword)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCurrentSecret):
			api.GinUnauthorized(c, err, "Current password is incorrect")
		case errors.Is(err, service.ErrMFANotEnabled):
			api.GinBadRequest(c, err, "TOTP is not enabled")
		default:
			api.GinInternalError(c, err, "Failed to disable TOTP")
		}
		return
	}

	api.GinSuccess(c, dto.ToMFAStatusDTO(status))
}

// RegenerateRecoveryCodes replaces the current recovery codes.
// @Summary Regenerate recovery codes
// @Description Generate a fresh set of recovery codes for the authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.RegenerateRecoveryCodesRequestDTO true "Recovery code regeneration payload"
// @Success 200 {object} api.Result{data=dto.RecoveryCodesResponseDTO} "Recovery codes regenerated successfully"
// @Failure 400 {object} api.Result "MFA is not enabled"
// @Failure 401 {object} api.Result "Unauthorized or incorrect password"
// @Failure 500 {object} api.Result "Internal server error"
// @Router /api/v1/auth/mfa/recovery-codes/regenerate [post]
func (h *AuthHandler) RegenerateRecoveryCodes(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	var req dto.RegenerateRecoveryCodesRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.GinBadRequest(c, err, "Invalid request data")
		return
	}

	response, err := h.authService.RegenerateRecoveryCodes(c.Request.Context(), user.UserID, req.CurrentPassword)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCurrentSecret):
			api.GinUnauthorized(c, err, "Current password is incorrect")
		case errors.Is(err, service.ErrMFANotEnabled):
			api.GinBadRequest(c, err, "TOTP is not enabled")
		default:
			api.GinInternalError(c, err, "Failed to regenerate recovery codes")
		}
		return
	}

	api.GinSuccess(c, dto.ToRecoveryCodesResponseDTO(response))
}
