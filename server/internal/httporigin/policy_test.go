package httporigin

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"server/config"

	"github.com/stretchr/testify/require"
)

func TestResolveDerivesTargetFromEachRequest(t *testing.T) {
	policy := newTestPolicy(t, nil)

	direct := httptest.NewRequest(http.MethodGet, "http://192.168.1.20:6680/probe", nil)
	direct.RemoteAddr = "192.168.1.30:54321"
	ctx, err := policy.Resolve(direct)
	require.NoError(t, err)
	require.Equal(t, "http://192.168.1.20:6680", ctx.TargetOrigin)
	require.Equal(t, ctx.TargetOrigin, ctx.BrowserOrigin)
	require.False(t, ctx.IsSecureForPasskey)

	https := httptest.NewRequest(http.MethodGet, "https://photos.example.com/probe", nil)
	https.RemoteAddr = "127.0.0.1:54321"
	https.TLS = &tls.ConnectionState{}
	ctx, err = policy.Resolve(https)
	require.NoError(t, err)
	require.Equal(t, "https://photos.example.com", ctx.TargetOrigin)
	require.True(t, ctx.IsSecureForPasskey)
}

func TestForwardedTargetPrecedenceAndMalformedFallback(t *testing.T) {
	policy := newTestPolicy(t, nil)
	request := httptest.NewRequest(http.MethodGet, "http://internal:6680/probe", nil)
	request.RemoteAddr = "192.0.2.10:54321"
	request.Header.Set("Forwarded", `for=203.0.113.1;proto="broken";host="bad host", proto=https;host=photos.example.com`)
	request.Header.Set("X-Forwarded-Proto", "http")
	request.Header.Set("X-Forwarded-Host", "fallback.example.com")

	ctx, err := policy.Resolve(request)
	require.NoError(t, err)
	require.Equal(t, "https://photos.example.com", ctx.TargetOrigin)

	request.Header.Set("Forwarded", `proto="broken";host="bad host"`)
	ctx, err = policy.Resolve(request)
	require.NoError(t, err)
	require.Equal(t, "http://fallback.example.com", ctx.TargetOrigin)
}

func TestTrustedProxyOnlyAffectsClientIPRecovery(t *testing.T) {
	policy := newTestPolicy(t, []netip.Prefix{netip.MustParsePrefix("192.168.1.10/32")})

	trusted := proxiedRequest("192.168.1.10:54321")
	trusted.Header.Set("X-Forwarded-For", "203.0.113.42, 192.168.1.10")
	ctx, err := policy.Resolve(trusted)
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("203.0.113.42"), ctx.ClientIP)
	require.Equal(t, "https://photos.example.com", ctx.TargetOrigin)

	untrusted := proxiedRequest("192.168.1.20:54321")
	untrusted.Header.Set("X-Forwarded-For", "203.0.113.42")
	ctx, err = policy.Resolve(untrusted)
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("192.168.1.20"), ctx.ClientIP)
	require.Equal(t, "https://photos.example.com", ctx.TargetOrigin)

	trusted.Header.Set("X-Forwarded-For", "not-an-ip")
	ctx, err = policy.Resolve(trusted)
	require.NoError(t, err)
	require.Equal(t, netip.MustParseAddr("192.168.1.10"), ctx.ClientIP)
}

func TestBrowserOriginIsNormalizedAndValidated(t *testing.T) {
	policy := newTestPolicy(t, nil)
	request := httptest.NewRequest(http.MethodGet, "http://localhost:6680/probe", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("Origin", "https://PHOTOS.example.com:443")

	ctx, err := policy.Resolve(request)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:6680", ctx.TargetOrigin)
	require.Equal(t, "https://photos.example.com", ctx.BrowserOrigin)
	require.True(t, ctx.IsSecureForPasskey)

	for _, origin := range []string{"null", "https://photos.example.com/path", "https://a.example, https://b.example"} {
		request.Header.Set("Origin", origin)
		_, err := policy.Resolve(request)
		require.True(t, errors.Is(err, ErrInvalidBrowserOrigin), origin)
	}
	request.Header.Del("Origin")
	request.Header.Add("Origin", "https://a.example")
	request.Header.Add("Origin", "https://b.example")
	_, err = policy.Resolve(request)
	require.ErrorIs(t, err, ErrInvalidBrowserOrigin)
}

func TestPasskeyAvailabilityUsesCurrentBrowserOrigin(t *testing.T) {
	policy := newTestPolicy(t, nil)
	tests := []struct {
		origin    string
		available bool
		reason    PasskeyUnavailableReason
	}{
		{"http://localhost:6680", true, ""},
		{"http://127.0.0.1:6680", false, PasskeyUnavailableSecureOriginRequired},
		{"http://192.168.1.20:6680", false, PasskeyUnavailableSecureOriginRequired},
		{"https://photos.example.com", true, ""},
		{"https://203.0.113.10", false, PasskeyUnavailableDomainRequired},
		{"not-an-origin", false, PasskeyUnavailableInvalidOrigin},
	}
	for _, test := range tests {
		t.Run(test.origin, func(t *testing.T) {
			available, reason := policy.PasskeyAvailability(RequestContext{BrowserOrigin: test.origin})
			require.Equal(t, test.available, available)
			require.Equal(t, test.reason, reason)
		})
	}
}

func newTestPolicy(t *testing.T, trusted []netip.Prefix) *Policy {
	t.Helper()
	policy, err := New(config.ServerConfig{
		TLS:   config.TLSConfig{Mode: config.TLSModeOff},
		Proxy: config.ProxyConfig{TrustedCIDRs: trusted},
	}, config.PasskeyConfig{Enabled: true, Name: "Lumilio Photos"})
	require.NoError(t, err)
	return policy
}

func proxiedRequest(remoteAddr string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "http://internal:6680/probe", nil)
	request.RemoteAddr = remoteAddr
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "photos.example.com")
	return request
}
