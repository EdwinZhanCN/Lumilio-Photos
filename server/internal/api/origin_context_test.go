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

func TestOriginPolicyMiddlewareAcceptsDirectAndForwardedTargets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy, err := httporigin.New(config.ServerConfig{
		TLS: config.TLSConfig{Mode: config.TLSModeOff},
		Proxy: config.ProxyConfig{TrustedCIDRs: []netip.Prefix{
			netip.MustParsePrefix("192.168.1.10/32"),
		}},
	}, config.PasskeyConfig{Enabled: true, Name: "Lumilio Photos"})
	require.NoError(t, err)

	router := gin.New()
	router.Use(originPolicyMiddleware(policy))
	router.Any("/*path", func(c *gin.Context) {
		resolved, ok := RequestOriginContext(c)
		require.True(t, ok)
		c.Header("X-Test-Origin", resolved.TargetOrigin)
		c.Status(http.StatusNoContent)
	})

	for name, request := range map[string]*http.Request{
		"direct LAN": func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "http://192.168.1.20:6680/api/v1/assets", nil)
			r.RemoteAddr = "192.168.1.30:54321"
			return r
		}(),
		"forwarded HTTPS": func() *http.Request {
			r := httptest.NewRequest(http.MethodGet, "http://internal:6680/api/v1/assets", nil)
			r.RemoteAddr = "192.168.1.10:54321"
			r.Header.Set("X-Forwarded-Proto", "https")
			r.Header.Set("X-Forwarded-Host", "photos.example.com")
			return r
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusNoContent, recorder.Code)
			require.NotEmpty(t, recorder.Header().Get("X-Test-Origin"))
		})
	}
}
