package handler

import (
	"errors"
	"strconv"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

// VerifySecurity creates a short-lived, purpose-bound proof used by account
// security mutations. Password-only accounts need the current password;
// MFA-enabled accounts additionally prove possession of TOTP or a recovery
// code.
// @Summary Verify recent account security
// @Description Create a one-time security proof for a TOTP, recovery-code, or passkey mutation.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.SecurityVerificationRequestDTO true "Security verification payload"
// @Success 200 {object} dto.SecurityVerificationResponseDTO "Security proof created"
// @Failure 400 {object} api.ProblemResponse "Invalid request data"
// @Failure 401 {object} api.ProblemResponse "Invalid security verification"
// @Router /api/v1/auth/security/verify [post]
func (h *AuthHandler) VerifySecurity(c *gin.Context) {
	if !h.allowAuthNetwork(c, authRateScopeSecurityVerify) {
		return
	}
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	securitySubject := strconv.FormatInt(int64(user.UserID), 10)
	if !h.allowAuthOpaqueSubject(c, authRateScopeSecurityVerify, securitySubject) {
		return
	}
	var req dto.SecurityVerificationRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	response, err := h.authService.VerifySecurity(c.Request.Context(), user.UserID, service.SecurityVerificationInput{
		CurrentPassword: req.CurrentPassword,
		Code:            req.Code,
		Method:          req.Method,
		Purpose:         req.Purpose,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCurrentSecret),
			errors.Is(err, service.ErrInvalidSecurityProof),
			errors.Is(err, service.ErrInvalidMFACode):
			api.WriteProblem(c, api.KnownProblem(problem.MFAInvalid, err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}
	h.resetAuthOpaqueSubject(authRateScopeSecurityVerify, securitySubject)
	api.JSONOK(c, dto.ToSecurityVerificationResponseDTO(response))
}

// VerifyMFA completes a pending login after the password step.
// @Summary Verify MFA challenge
// @Description Verify a pending MFA login challenge with a TOTP code or recovery code.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.VerifyMFARequestDTO true "MFA verification payload"
// @Success 200 {object} dto.AuthResponseDTO "MFA verification successful"
// @Failure 400 {object} api.ProblemResponse "Invalid request data"
// @Failure 401 {object} api.ProblemResponse "Invalid or expired MFA challenge"
// @Failure 429 {object} api.ProblemResponse "Too many authentication attempts"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/mfa/verify [post]
func (h *AuthHandler) VerifyMFA(c *gin.Context) {
	if !h.allowAuthNetwork(c, authRateScopeMFAVerify) {
		return
	}

	var req dto.VerifyMFARequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if !h.allowAuthOpaqueSubject(c, authRateScopeMFAVerify, req.MFAToken) {
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
			api.WriteProblem(c, api.KnownProblem(problem.MFAInvalid, err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	h.resetAuthOpaqueSubject(authRateScopeMFAVerify, req.MFAToken)
	h.writeAuthResponse(c, authResponse)
}

// GetMFAStatus returns the current user's MFA status.
// @Summary Get MFA status
// @Description Get the authenticated user's MFA status, including TOTP enablement and remaining recovery codes.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.MFAStatusDTO "MFA status retrieved successfully"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/mfa [get]
func (h *AuthHandler) GetMFAStatus(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	status, err := h.authService.GetMFAStatus(c.Request.Context(), user.UserID)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.ToMFAStatusDTO(status))
}

// BeginTOTPSetup starts a TOTP setup flow.
// @Summary Begin TOTP setup
// @Description Generate a new TOTP secret and setup token for the authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.TOTPSetupRequestDTO true "Security verification payload"
// @Success 200 {object} dto.TOTPSetupResponseDTO "TOTP setup created successfully"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/mfa/totp/setup [post]
func (h *AuthHandler) BeginTOTPSetup(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}
	var req dto.TOTPSetupRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	response, err := h.authService.BeginTOTPSetup(c.Request.Context(), user.UserID, req.SecurityToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidSecurityProof):
			api.WriteProblem(c, api.Unauthorized(err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	api.JSONOK(c, dto.ToTOTPSetupResponseDTO(response))
}

// EnableTOTP completes TOTP enrollment.
// @Summary Enable TOTP
// @Description Verify a TOTP setup code and enable TOTP MFA for the authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.EnableTOTPRequestDTO true "TOTP enable payload"
// @Success 200 {object} dto.RecoveryCodesResponseDTO "TOTP enabled successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid setup token or verification code"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/mfa/totp/enable [post]
func (h *AuthHandler) EnableTOTP(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	var req dto.EnableTOTPRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
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
			api.WriteProblem(c, api.BadRequest(err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	h.writeRecoveryCodesResponse(c, response)
}

// DisableTOTP disables TOTP MFA for the authenticated user.
// @Summary Disable TOTP
// @Description Disable TOTP MFA and invalidate recovery codes for the authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.DisableTOTPRequestDTO true "Disable TOTP payload"
// @Success 200 {object} dto.MFAStatusDTO "TOTP disabled successfully"
// @Failure 400 {object} api.ProblemResponse "MFA is not enabled"
// @Failure 401 {object} api.ProblemResponse "Unauthorized or incorrect password"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/mfa/totp/disable [post]
func (h *AuthHandler) DisableTOTP(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	var req dto.DisableTOTPRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	status, err := h.authService.DisableTOTP(c.Request.Context(), user.UserID, req.SecurityToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidSecurityProof):
			api.WriteProblem(c, api.Unauthorized(err))
		case errors.Is(err, service.ErrMFANotEnabled):
			api.WriteProblem(c, api.BadRequest(err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	h.writeMFAStatusResponse(c, status)
}

// RegenerateRecoveryCodes replaces the current recovery codes.
// @Summary Regenerate recovery codes
// @Description Generate a fresh set of recovery codes for the authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.RegenerateRecoveryCodesRequestDTO true "Recovery code regeneration payload"
// @Success 200 {object} dto.RecoveryCodesResponseDTO "Recovery codes regenerated successfully"
// @Failure 400 {object} api.ProblemResponse "MFA is not enabled"
// @Failure 401 {object} api.ProblemResponse "Unauthorized or incorrect password"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/mfa/recovery-codes/regenerate [post]
func (h *AuthHandler) RegenerateRecoveryCodes(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	var req dto.RegenerateRecoveryCodesRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	response, err := h.authService.RegenerateRecoveryCodes(c.Request.Context(), user.UserID, req.SecurityToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidSecurityProof):
			api.WriteProblem(c, api.Unauthorized(err))
		case errors.Is(err, service.ErrMFANotEnabled):
			api.WriteProblem(c, api.BadRequest(err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	h.writeRecoveryCodesResponse(c, response)
}

func (h *AuthHandler) writeRecoveryCodesResponse(c *gin.Context, response service.RecoveryCodesResponse) {
	payload := dto.ToRecoveryCodesResponseDTO(response)
	if response.Session != nil {
		csrfToken, ok := h.installBrowserSession(c, response.Session)
		if !ok {
			return
		}
		if payload.Session != nil {
			payload.Session.CSRFToken = csrfToken
		}
	}
	api.JSONOK(c, payload)
}

func (h *AuthHandler) writeMFAStatusResponse(c *gin.Context, response service.MFAStatusResponse) {
	payload := dto.ToMFAStatusResponseDTO(response)
	if response.Session != nil {
		csrfToken, ok := h.installBrowserSession(c, response.Session)
		if !ok {
			return
		}
		if payload.Session != nil {
			payload.Session.CSRFToken = csrfToken
		}
	}
	api.JSONOK(c, payload)
}

func (h *AuthHandler) writePasskeyEnrollmentResponse(c *gin.Context, response service.PasskeyEnrollmentResponse) {
	payload := dto.ToPasskeyEnrollmentResponseDTO(response)
	if response.Session != nil {
		csrfToken, ok := h.installBrowserSession(c, response.Session)
		if !ok {
			return
		}
		if payload.Session != nil {
			payload.Session.CSRFToken = csrfToken
		}
	}
	api.JSONOK(c, payload)
}

func (h *AuthHandler) writePasskeyMutationResponse(c *gin.Context, response service.MFAStatusResponse) {
	payload := dto.ToPasskeyMutationResponseDTO(response)
	if response.Session != nil {
		csrfToken, ok := h.installBrowserSession(c, response.Session)
		if !ok {
			return
		}
		if payload.Session != nil {
			payload.Session.CSRFToken = csrfToken
		}
	}
	api.JSONOK(c, payload)
}
