package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/config"
	"server/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newAuthSessionTestHandler(t *testing.T) *AuthHandler {
	t.Helper()
	authService, err := service.NewAuthService(nil, nil, config.AuthConfig{
		SecretKeyFile: filepath.Join(t.TempDir(), "lumilio-secret"),
	})
	require.NoError(t, err)
	return NewAuthHandler(authService, nil, time.Hour)
}

func TestWriteAuthResponseUsesHttpOnlyRefreshCookieAndOmitsCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newAuthSessionTestHandler(t)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "http://192.168.1.20:6680/api/v1/auth/login", nil)
	context.Request.Header.Set("Origin", "http://192.168.1.20:6680")

	handler.writeAuthResponse(context, &service.AuthResponse{
		AccessToken:  "short-lived-access",
		RefreshToken: "long-lived-refresh",
	})

	require.Equal(t, http.StatusOK, recorder.Code)
	setCookie := recorder.Header().Get("Set-Cookie")
	require.Contains(t, setCookie, "lumilio_refresh=long-lived-refresh")
	require.Contains(t, setCookie, "Path=/api/v1/auth")
	require.Contains(t, setCookie, "HttpOnly")
	require.Contains(t, setCookie, "SameSite=Lax")
	require.NotContains(t, setCookie, "Secure")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, "short-lived-access", payload["token"])
	require.NotEmpty(t, payload["csrfToken"])
	require.NotContains(t, recorder.Body.String(), "long-lived-refresh")
	require.NotContains(t, recorder.Body.String(), "refreshToken")
	require.True(t, handler.authService.ValidateCSRFToken(
		"long-lived-refresh",
		payload["csrfToken"].(string),
	))
}

func TestWriteAuthResponseUsesSecureNoneCookieForHTTPSCrossSiteSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newAuthSessionTestHandler(t)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "http://server:6680/api/v1/auth/login", nil)
	context.Request.Host = "api.photos.example"
	context.Request.Header.Set("X-Forwarded-Proto", "https")
	context.Request.Header.Set("Origin", "https://ui.example.test")

	handler.writeAuthResponse(context, &service.AuthResponse{
		AccessToken:  "short-lived-access",
		RefreshToken: "long-lived-refresh",
	})

	setCookie := recorder.Header().Get("Set-Cookie")
	require.Contains(t, setCookie, "Secure")
	require.Contains(t, setCookie, "SameSite=None")
}

func TestGetCSRFTokenRequiresRefreshCookieAndBindsResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newAuthSessionTestHandler(t)

	t.Run("missing cookie", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/auth/csrf", nil)

		handler.GetCSRFToken(context)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
	})

	t.Run("bound token", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/auth/csrf", nil)
		context.Request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "refresh-a"})

		handler.GetCSRFToken(context)

		require.Equal(t, http.StatusOK, recorder.Code)
		var payload map[string]string
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
		require.True(t, handler.authService.ValidateCSRFToken("refresh-a", payload["csrfToken"]))
		require.False(t, handler.authService.ValidateCSRFToken("refresh-b", payload["csrfToken"]))
	})
}

func TestRequireCSRFTokenRejectsMissingAndTamperedValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := newAuthSessionTestHandler(t)
	valid := handler.authService.CSRFTokenForRefresh("refresh-a")

	for name, value := range map[string]string{
		"missing":  "",
		"tampered": valid + "x",
		"other":    handler.authService.CSRFTokenForRefresh("refresh-b"),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodPost, "http://localhost/api/v1/auth/refresh", nil)
			if strings.TrimSpace(value) != "" {
				context.Request.Header.Set(csrfHeaderName, value)
			}

			require.False(t, handler.requireCSRFToken(context, "refresh-a"))
			require.Equal(t, http.StatusForbidden, recorder.Code)
		})
	}
}
