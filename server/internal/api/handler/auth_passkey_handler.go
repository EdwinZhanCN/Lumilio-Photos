package handler

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

// BeginPasskeyLogin creates WebAuthn request options for username-first passkey login.
// @Summary Begin passkey login
// @Description Create WebAuthn login options for a username-first passkey login flow.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.PasskeyOptionsRequestDTO true "Username for passkey login"
// @Success 200 {object} dto.PasskeyOptionsResponseDTO "Passkey login options created successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request data"
// @Failure 401 {object} api.ProblemResponse "Invalid credentials"
// @Failure 429 {object} api.ProblemResponse "Too many authentication attempts"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/passkeys/login/options [post]
func (h *AuthHandler) BeginPasskeyLogin(c *gin.Context) {
	origin, ok := requirePasskeyRequest(c)
	if !ok {
		return
	}
	if !h.allowAuthNetwork(c, authRateScopePasskeyOptions) {
		return
	}

	var req dto.PasskeyOptionsRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		api.WriteProblem(c, api.BadRequest(errors.New("username is required")))
		return
	}
	if !h.allowAuthUsername(c, authRateScopePasskeyOptions, req.Username) {
		return
	}

	response, err := h.authService.BeginPasskeyLogin(c.Request.Context(), req.Username, origin)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPasskeyNotConfigured),
			errors.Is(err, service.ErrUserNotFound):
			api.WriteProblem(c, api.KnownProblem(problem.InvalidCredentials, err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	h.resetAuthUsername(authRateScopePasskeyOptions, req.Username)
	api.JSONOK(c, dto.ToPasskeyOptionsResponseDTO(response))
}

// VerifyPasskeyLogin completes username-first passkey login.
// @Summary Verify passkey login
// @Description Verify a passkey login assertion and issue session tokens.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body dto.PasskeyVerifyRequestDTO true "Passkey login verification payload"
// @Success 200 {object} dto.AuthResponseDTO "Passkey login verified successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid request data"
// @Failure 401 {object} api.ProblemResponse "Invalid credentials"
// @Failure 429 {object} api.ProblemResponse "Too many authentication attempts"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/passkeys/login/verify [post]
func (h *AuthHandler) VerifyPasskeyLogin(c *gin.Context) {
	origin, ok := requirePasskeyRequest(c)
	if !ok {
		return
	}
	if !h.allowAuthNetwork(c, authRateScopePasskeyVerify) {
		return
	}

	var req dto.PasskeyVerifyRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}
	if !h.allowAuthOpaqueSubject(c, authRateScopePasskeyVerify, req.ChallengeToken) {
		return
	}

	credentialJSON, err := json.Marshal(req.Credential)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	response, err := h.authService.VerifyPasskeyLogin(c.Request.Context(), req.ChallengeToken, credentialJSON, origin)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPasskeyChallenge),
			errors.Is(err, service.ErrExpiredPasskeyChallenge),
			errors.Is(err, service.ErrPasskeyNotConfigured),
			errors.Is(err, service.ErrUserNotFound):
			api.WriteProblem(c, api.KnownProblem(problem.InvalidCredentials, err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	h.resetAuthOpaqueSubject(authRateScopePasskeyVerify, req.ChallengeToken)
	h.writeAuthResponse(c, response)
}

// ListPasskeys returns the authenticated user's passkeys.
// @Summary List passkeys
// @Description List the authenticated user's enrolled passkeys.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.PasskeyListResponseDTO "Passkeys retrieved successfully"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/mfa/passkeys [get]
func (h *AuthHandler) ListPasskeys(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	response, err := h.authService.ListPasskeys(c.Request.Context(), user.UserID)
	if err != nil {
		api.WriteProblem(c, api.Internal(err))
		return
	}

	api.JSONOK(c, dto.ToPasskeyListResponseDTO(response))
}

// BeginPasskeyEnrollment creates WebAuthn registration options for the authenticated user.
// @Summary Begin passkey enrollment
// @Description Create WebAuthn registration options to add a new passkey to the authenticated account.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} dto.PasskeyOptionsResponseDTO "Passkey enrollment options created successfully"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/mfa/passkeys/options [post]
func (h *AuthHandler) BeginPasskeyEnrollment(c *gin.Context) {
	origin, ok := requirePasskeyRequest(c)
	if !ok {
		return
	}
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	response, err := h.authService.BeginPasskeyEnrollment(c.Request.Context(), user.UserID, origin)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTOTPRequiredForPasskey):
			api.WriteProblem(c, api.BadRequest(err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	api.JSONOK(c, dto.ToPasskeyOptionsResponseDTO(response))
}

// VerifyPasskeyEnrollment completes passkey enrollment for the authenticated user.
// @Summary Verify passkey enrollment
// @Description Verify a passkey enrollment response and attach the new passkey to the authenticated account.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.PasskeyEnrollmentVerifyRequestDTO true "Passkey enrollment verification payload"
// @Success 200 {object} dto.PasskeyEnrollmentResponseDTO "Passkey enrolled successfully"
// @Failure 400 {object} api.ProblemResponse "Invalid or expired challenge"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/mfa/passkeys/verify [post]
func (h *AuthHandler) VerifyPasskeyEnrollment(c *gin.Context) {
	origin, ok := requirePasskeyRequest(c)
	if !ok {
		return
	}
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	var req dto.PasskeyEnrollmentVerifyRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	credentialJSON, err := json.Marshal(req.Credential)
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	response, err := h.authService.VerifyPasskeyEnrollment(c.Request.Context(), user.UserID, req.ChallengeToken, credentialJSON, req.SecurityToken, origin)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidPasskeyChallenge),
			errors.Is(err, service.ErrExpiredPasskeyChallenge):
			api.WriteProblem(c, api.BadRequest(err))
		case errors.Is(err, service.ErrTOTPRequiredForPasskey):
			api.WriteProblem(c, api.BadRequest(err))
		case errors.Is(err, service.ErrInvalidSecurityProof):
			api.WriteProblem(c, api.Unauthorized(err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	h.writePasskeyEnrollmentResponse(c, response)
}

// DeletePasskey removes an enrolled passkey from the authenticated user.
// @Summary Delete passkey
// @Description Delete one enrolled passkey for the authenticated user.
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "Passkey ID"
// @Param request body dto.PasskeyDeleteRequestDTO true "Recent security verification token"
// @Success 200 {object} dto.PasskeyMutationResponseDTO "Passkey deleted successfully"
// @Failure 401 {object} api.ProblemResponse "Unauthorized"
// @Failure 404 {object} api.ProblemResponse "Passkey not found"
// @Failure 500 {object} api.ProblemResponse "Internal server error"
// @Router /api/v1/auth/mfa/passkeys/{id} [delete]
func (h *AuthHandler) DeletePasskey(c *gin.Context) {
	user, ok := requireCurrentUser(c)
	if !ok {
		return
	}

	passkeyID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	var req dto.PasskeyDeleteRequestDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		api.WriteProblem(c, api.BadRequest(err))
		return
	}

	response, err := h.authService.DeletePasskey(c.Request.Context(), user.UserID, passkeyID, req.SecurityToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPasskeyCredentialNotFound):
			api.WriteProblem(c, api.NotFound(err))
		case errors.Is(err, service.ErrInvalidSecurityProof):
			api.WriteProblem(c, api.Unauthorized(err))
		default:
			api.WriteProblem(c, api.Internal(err))
		}
		return
	}

	h.writePasskeyMutationResponse(c, response)
}

func requirePasskeyRequest(c *gin.Context) (string, bool) {
	resolved, ok := api.RequestOriginContext(c)
	if !ok || !resolved.IsSecureForPasskey {
		api.WriteProblem(c, api.KnownProblem(problem.PasskeyUnavailable, errors.New("passkey is unavailable for this browser origin")))
		return "", false
	}
	return resolved.BrowserOrigin, true
}
