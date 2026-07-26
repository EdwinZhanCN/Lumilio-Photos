package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"server/config"
	"server/internal/httporigin"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCORSMiddlewareSeparatesCredentialedAndOpenAPIRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const trustedOrigin = "https://ui.example.test"

	newRouter := func(t *testing.T, reached *bool) *gin.Engine {
		t.Helper()
		router := gin.New()
		policy := testOriginPolicy(t, "http://api.example.test", []string{trustedOrigin})
		router.Use(originPolicyMiddleware(policy))
		router.Use(corsMiddleware(policy))
		router.Any("/probe", func(c *gin.Context) {
			*reached = true
			c.Status(http.StatusNoContent)
		})
		return router
	}

	t.Run("credentialless arbitrary origin uses wildcard without credentials", func(t *testing.T) {
		reached := false
		router := newRouter(t, &reached)
		request := httptest.NewRequest(http.MethodGet, "http://api.example.test/probe", nil)
		request.Header.Set("Origin", "https://third-party.example")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		require.True(t, reached)
		require.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
		require.Empty(t, recorder.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("arbitrary origin with cookies receives no CORS grant", func(t *testing.T) {
		reached := false
		router := newRouter(t, &reached)
		request := httptest.NewRequest(http.MethodGet, "http://api.example.test/probe", nil)
		request.Header.Set("Origin", "https://third-party.example")
		request.AddCookie(&http.Cookie{Name: "lumilio_refresh", Value: "secret"})
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		require.True(t, reached)
		require.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
		require.Empty(t, recorder.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("trusted origin receives an exact credentialed grant", func(t *testing.T) {
		reached := false
		router := newRouter(t, &reached)
		request := httptest.NewRequest(http.MethodGet, "http://api.example.test/probe", nil)
		request.Header.Set("Origin", trustedOrigin)
		request.AddCookie(&http.Cookie{Name: "lumilio_refresh", Value: "secret"})
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		require.True(t, reached)
		require.Equal(t, trustedOrigin, recorder.Header().Get("Access-Control-Allow-Origin"))
		require.Equal(t, "true", recorder.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("credentialless preflight is open and terminates before the route", func(t *testing.T) {
		reached := false
		router := newRouter(t, &reached)
		request := httptest.NewRequest(http.MethodOptions, "http://api.example.test/probe", nil)
		request.Header.Set("Origin", "https://third-party.example")
		request.Header.Set("Access-Control-Request-Method", http.MethodPost)
		request.Header.Set("Access-Control-Request-Headers", "authorization")
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, request)

		require.False(t, reached)
		require.Equal(t, http.StatusNoContent, recorder.Code)
		require.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
		require.Contains(t, recorder.Header().Get("Access-Control-Allow-Headers"), "Authorization")
		require.Contains(t, recorder.Header().Get("Access-Control-Allow-Headers"), "X-CSRF-Token")
	})
}

func TestTrustedSessionOriginMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const trustedOrigin = "https://ui.example.test"

	testRequest := func(origin, referer, fetchSite string) int {
		router := gin.New()
		policy := testOriginPolicy(t, "http://photos.example.test", []string{trustedOrigin})
		router.Use(originPolicyMiddleware(policy))
		router.Use(trustedSessionOriginMiddleware(policy))
		router.POST("/session", func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})

		request := httptest.NewRequest(http.MethodPost, "http://photos.example.test/session", nil)
		request.Host = "photos.example.test"
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		if referer != "" {
			request.Header.Set("Referer", referer)
		}
		if fetchSite != "" {
			request.Header.Set("Sec-Fetch-Site", fetchSite)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder.Code
	}

	require.Equal(t, http.StatusNoContent, testRequest("http://photos.example.test", "", "same-origin"))
	require.Equal(t, http.StatusNoContent, testRequest(trustedOrigin, "", "cross-site"))
	require.Equal(t, http.StatusNoContent, testRequest("", "http://photos.example.test/login", ""))
	require.Equal(t, http.StatusNoContent, testRequest("", "", ""))

	require.Equal(t, http.StatusForbidden, testRequest("https://evil.example", "", "cross-site"))
	require.Equal(t, http.StatusBadRequest, testRequest("null", "", "cross-site"))
	require.Equal(t, http.StatusForbidden, testRequest("", "", "cross-site"))
	require.Equal(t, http.StatusForbidden, testRequest("", "not a URL", ""))
}

func testOriginPolicy(t *testing.T, primaryOrigin string, cors []string) *httporigin.Policy {
	t.Helper()
	policy, err := httporigin.New(config.ServerConfig{
		PrimaryOrigin:      primaryOrigin,
		CORSAllowedOrigins: cors,
		TLS:                config.TLSConfig{Mode: config.TLSModeOff},
		Proxy:              config.ProxyConfig{Mode: config.ProxyModeDisabled},
	}, config.PasskeyConfig{Enabled: true, Name: "Lumilio Photos"})
	require.NoError(t, err)
	return policy
}

func TestNormalizeOriginRequiresAnExactHTTPOrigin(t *testing.T) {
	normalized, _, err := config.NormalizeOrigin("https://PHOTOS.Example.Test:443")
	require.NoError(t, err)
	require.Equal(t, "https://photos.example.test", normalized)

	for _, invalid := range []string{
		"null",
		"file:///tmp/photo",
		"https://user@example.test",
		"https://example.test/",
		"https://example.test/path",
		"https://example.test?query=1",
	} {
		_, _, err := config.NormalizeOrigin(invalid)
		require.Error(t, err, invalid)
	}
}
