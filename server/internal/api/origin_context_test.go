package api

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"server/config"
	"server/internal/httporigin"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOriginPolicyMiddlewareEnforcesRequiredProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy, err := httporigin.New(config.ServerConfig{
		PrimaryOrigin: "https://photos.example.com",
		TLS:           config.TLSConfig{Mode: config.TLSModeExternal},
		Proxy: config.ProxyConfig{
			Mode: config.ProxyModeRequired,
			TrustedCIDRs: []netip.Prefix{
				netip.MustParsePrefix("192.168.1.10/32"),
			},
		},
	}, config.PasskeyConfig{Enabled: true, Name: "Lumilio Photos"})
	require.NoError(t, err)

	router := gin.New()
	router.Use(originPolicyMiddleware(policy))
	router.Any("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	t.Run("ordinary loopback direct request is rejected", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "http://localhost:6680/api/v1/auth/login", nil)
		request.RemoteAddr = "127.0.0.1:54321"
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusForbidden, recorder.Code)
		require.Contains(t, recorder.Body.String(), "trusted_proxy_required")
	})

	t.Run("loopback liveness is the narrow exception", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "http://localhost:6680/api/v1/health/live", nil)
		request.RemoteAddr = "127.0.0.1:54321"
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusNoContent, recorder.Code)
	})

	t.Run("non-loopback cannot use the health exception", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "http://server:6680/api/v1/health/ready", nil)
		request.RemoteAddr = "192.168.1.20:54321"
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusForbidden, recorder.Code)
	})

	t.Run("trusted proxy canonical request is accepted", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "http://server:6680/api/v1/auth/login", nil)
		request.RemoteAddr = "192.168.1.10:54321"
		request.Header.Set("X-Forwarded-Proto", "https")
		request.Header.Set("X-Forwarded-Host", "photos.example.com")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusNoContent, recorder.Code)
	})

	t.Run("trusted proxy wrong public host is misdirected", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "http://server:6680/api/v1/assets", nil)
		request.RemoteAddr = "192.168.1.10:54321"
		request.Header.Set("X-Forwarded-Proto", "https")
		request.Header.Set("X-Forwarded-Host", "other.example.com")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusMisdirectedRequest, recorder.Code)
		require.Contains(t, recorder.Body.String(), "misdirected_request")
	})
}
