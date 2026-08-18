package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/api/problem"
	"server/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	refreshCookieName = "lumilio_refresh"
	csrfHeaderName    = "X-CSRF-Token"
)

func (h *AuthHandler) writeAuthResponse(c *gin.Context, response *service.AuthResponse) {
	payload := dto.ToAuthResponseDTO(response)
	if response == nil || response.RefreshToken == "" {
		api.JSONOK(c, payload)
		return
	}

	csrfToken, ok := h.installBrowserSession(c, response)
	if !ok {
		return
	}
	payload.CSRFToken = csrfToken
	api.JSONOK(c, payload)
}

// installBrowserSession is the single transport boundary for a newly issued
// browser session. Security-setting mutations use this same writer so cookie,
// CSRF and refresh-session replacement semantics cannot drift by endpoint.
func (h *AuthHandler) installBrowserSession(c *gin.Context, response *service.AuthResponse) (string, bool) {
	if response == nil || response.RefreshToken == "" {
		return "", true
	}

	sameSite, secure, err := sessionCookiePolicy(c)
	if err != nil {
		_ = h.authService.RevokeRefreshToken(response.RefreshToken)
		api.WriteProblem(c, api.BadRequest(err))
		return "", false
	}
	if previousRefreshToken, cookieErr := c.Cookie(refreshCookieName); cookieErr == nil &&
		previousRefreshToken != "" &&
		previousRefreshToken != response.RefreshToken {
		if revokeErr := h.authService.RevokeRefreshToken(previousRefreshToken); revokeErr != nil &&
			!errors.Is(revokeErr, service.ErrTokenNotFound) {
			_ = h.authService.RevokeRefreshToken(response.RefreshToken)
			api.WriteProblem(c, api.Internal(revokeErr))
			return "", false
		}
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    response.RefreshToken,
		Path:     "/api/v1/auth",
		MaxAge:   int(h.refreshTokenTTL.Seconds()),
		Expires:  time.Now().Add(h.refreshTokenTTL).UTC(),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
	return h.authService.CSRFTokenForRefresh(response.RefreshToken), true
}

func (h *AuthHandler) requireRefreshCookie(c *gin.Context) (string, bool) {
	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil || strings.TrimSpace(refreshToken) == "" {
		if err == nil {
			err = errors.New("refresh cookie is empty")
		}
		api.WriteProblem(c, api.KnownProblem(problem.SessionExpired, err))
		return "", false
	}
	return refreshToken, true
}

func (h *AuthHandler) requireCSRFToken(c *gin.Context, refreshToken string) bool {
	csrfToken := strings.TrimSpace(c.GetHeader(csrfHeaderName))
	if !h.authService.ValidateCSRFToken(refreshToken, csrfToken) {
		api.WriteProblem(c, api.Forbidden(errors.New("invalid CSRF token")))
		return false
	}
	return true
}

func (h *AuthHandler) clearAuthCookies(c *gin.Context) {
	sameSite, secure, _ := sessionCookiePolicy(c)
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/api/v1/auth",
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

// GetCSRFToken restores the browser-readable half of a valid cookie session.
// It deliberately does not rotate or query the refresh record; refresh remains
// the authoritative validation point and returns the generic 401 shape.
// @Summary Get refresh-session CSRF token
// @Description Return a CSRF token bound to the current HttpOnly refresh cookie
// @Tags auth
// @Produce json
// @Success 200 {object} dto.CSRFTokenDTO "CSRF token issued"
// @Failure 401 {object} api.ProblemResponse "No refresh-cookie session"
// @Router /api/v1/auth/csrf [get]
func (h *AuthHandler) GetCSRFToken(c *gin.Context) {
	refreshToken, ok := h.requireRefreshCookie(c)
	if !ok {
		return
	}
	api.JSONOK(c, dto.CSRFTokenDTO{
		CSRFToken: h.authService.CSRFTokenForRefresh(refreshToken),
	})
}

func sessionCookiePolicy(c *gin.Context) (http.SameSite, bool, error) {
	resolved, ok := api.RequestOriginContext(c)
	if !ok {
		return http.SameSiteLaxMode, false, errors.New("request origin policy was not resolved")
	}
	target, err := url.Parse(resolved.TargetOrigin)
	if err != nil {
		return http.SameSiteLaxMode, false, errors.New("invalid resolved target origin")
	}
	origin, err := url.Parse(resolved.BrowserOrigin)
	if err != nil {
		return http.SameSiteLaxMode, target.Scheme == "https", errors.New("invalid resolved browser origin")
	}

	if sameCookieSite(origin, target) {
		return http.SameSiteLaxMode, target.Scheme == "https", nil
	}
	if target.Scheme != "https" {
		return http.SameSiteNoneMode, true, errors.New("cross-site cookie session requires HTTPS")
	}
	return http.SameSiteNoneMode, true, nil
}

func sameCookieSite(origin, target *url.URL) bool {
	return strings.EqualFold(origin.Scheme, target.Scheme) &&
		strings.EqualFold(origin.Hostname(), target.Hostname())
}
