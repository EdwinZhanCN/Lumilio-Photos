package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"server/config"
	"server/internal/api"
	"server/internal/httporigin"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetBrowserCapabilitiesUsesResolvedOriginPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	policy, err := httporigin.New(config.ServerConfig{
		TLS: config.TLSConfig{Mode: config.TLSModeOff},
	}, config.PasskeyConfig{Enabled: true, Name: "Lumilio Photos"})
	require.NoError(t, err)
	handler := NewAuthHandler(nil, nil, time.Hour, policy)

	tests := map[string]struct {
		resolved httporigin.RequestContext
		want     dtoBrowserCapabilities
	}{
		"Desktop local": {
			resolved: httporigin.RequestContext{
				PeerIP:             netip.MustParseAddr("127.0.0.1"),
				ClientIP:           netip.MustParseAddr("127.0.0.1"),
				TargetOrigin:       "http://localhost:6680",
				BrowserOrigin:      "http://localhost:6680",
				IsSecureContext:    true,
				IsSecureForPasskey: true,
			},
			want: dtoBrowserCapabilities{
				CurrentOrigin:    "http://localhost:6680",
				PasskeyAvailable: true,
			},
		},
		"Desktop LAN HTTP remote browser": {
			resolved: httporigin.RequestContext{
				PeerIP:          netip.MustParseAddr("192.168.1.30"),
				ClientIP:        netip.MustParseAddr("192.168.1.30"),
				TargetOrigin:    "http://192.168.1.20:6680",
				BrowserOrigin:   "http://192.168.1.20:6680",
				IsSecureContext: false,
			},
			want: dtoBrowserCapabilities{
				CurrentOrigin:            "http://192.168.1.20:6680",
				PasskeyUnavailableReason: "secure_origin_required",
				InsecureTransport:        true,
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/browser-capabilities", nil)
			api.SetRequestOriginContext(context, test.resolved)

			handler.GetBrowserCapabilities(context)

			require.Equal(t, http.StatusOK, recorder.Code)
			var got dtoBrowserCapabilities
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &got))
			require.Equal(t, test.want.CurrentOrigin, got.CurrentOrigin)
			require.Equal(t, test.want.PasskeyAvailable, got.PasskeyAvailable)
			require.Equal(t, test.want.PasskeyUnavailableReason, got.PasskeyUnavailableReason)
			require.Equal(t, test.want.InsecureTransport, got.InsecureTransport)
		})
	}
}

type dtoBrowserCapabilities struct {
	CurrentOrigin            string `json:"current_origin"`
	PasskeyAvailable         bool   `json:"passkey_available"`
	PasskeyUnavailableReason string `json:"passkey_unavailable_reason"`
	InsecureTransport        bool   `json:"insecure_transport"`
}
