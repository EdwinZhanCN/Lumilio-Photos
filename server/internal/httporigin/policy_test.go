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

func TestDisabledProxyIgnoresForwardedHeaders(t *testing.T) {
	policy := newTestPolicy(t, config.ProxyModeDisabled, nil)
	request := httptest.NewRequest(http.MethodGet, "http://localhost:6680/probe", nil)
	request.RemoteAddr = "192.0.2.10:54321"
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "evil.example")
	request.Header.Set("X-Forwarded-For", "203.0.113.7")

	ctx, err := policy.Resolve(request)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:6680", ctx.TargetOrigin)
	require.Equal(t, netip.MustParseAddr("192.0.2.10"), ctx.ClientIP)
	require.False(t, ctx.ViaTrustedProxy)
	require.True(t, ctx.IsSecureForPasskey)
}

func TestRequiredProxyUsesImmediatePeerTrust(t *testing.T) {
	policy := newTestPolicy(t, config.ProxyModeRequired, []netip.Prefix{
		netip.MustParsePrefix("192.168.1.10/32"),
	})

	t.Run("untrusted direct peer cannot forge HTTPS", func(t *testing.T) {
		request := proxiedRequest("192.168.1.20:54321")
		_, err := policy.Resolve(request)
		require.ErrorIs(t, err, ErrTrustedProxyRequired)
	})

	t.Run("trusted peer reconstructs the canonical target", func(t *testing.T) {
		request := proxiedRequest("192.168.1.10:54321")
		request.Header.Set("X-Forwarded-For", "203.0.113.42, 192.168.1.10")
		ctx, err := policy.Resolve(request)
		require.NoError(t, err)
		require.True(t, ctx.ViaTrustedProxy)
		require.Equal(t, "https://photos.example.com", ctx.TargetOrigin)
		require.Equal(t, ctx.TargetOrigin, ctx.BrowserOrigin)
		require.Equal(t, netip.MustParseAddr("203.0.113.42"), ctx.ClientIP)
		require.True(t, ctx.IsPrimaryOrigin)
		require.True(t, ctx.IsSecureForPasskey)
	})
}

func TestRequiredProxyFailsClosedOnAmbiguousTargets(t *testing.T) {
	policy := newTestPolicy(t, config.ProxyModeRequired, []netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"),
	})

	tests := map[string]func(*http.Request){
		"missing host": func(request *http.Request) {
			request.Header.Del("X-Forwarded-Host")
		},
		"multiple proto values": func(request *http.Request) {
			request.Header.Set("X-Forwarded-Proto", "https,http")
		},
		"header families disagree": func(request *http.Request) {
			request.Header.Set("Forwarded", "for=203.0.113.1;proto=http;host=photos.example.com")
		},
		"malformed client chain": func(request *http.Request) {
			request.Header.Set("X-Forwarded-For", "not-an-ip")
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			request := proxiedRequest("127.0.0.1:54321")
			mutate(request)
			_, err := policy.Resolve(request)
			require.ErrorIs(t, err, ErrInvalidForwarded)
		})
	}
}

func TestRequiredProxyRejectsNonPrimaryTarget(t *testing.T) {
	policy := newTestPolicy(t, config.ProxyModeRequired, []netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"),
	})
	request := proxiedRequest("127.0.0.1:54321")
	request.Header.Set("X-Forwarded-Host", "other.example.com")

	_, err := policy.Resolve(request)
	require.ErrorIs(t, err, ErrMisdirectedRequest)
}

func TestBrowserOriginIsIndependentFromRequestTarget(t *testing.T) {
	policy := newTestPolicy(t, config.ProxyModeRequired, []netip.Prefix{
		netip.MustParsePrefix("::1/128"),
	})
	request := proxiedRequest("[::1]:54321")
	request.Header.Set("Origin", "https://client.example.com")

	ctx, err := policy.Resolve(request)
	require.NoError(t, err)
	require.Equal(t, "https://photos.example.com", ctx.TargetOrigin)
	require.Equal(t, "https://client.example.com", ctx.BrowserOrigin)
	require.False(t, ctx.IsPrimaryOrigin)
	require.False(t, ctx.IsSecureForPasskey)
}

func TestDirectHTTPSUsesTLSAndHost(t *testing.T) {
	server := config.ServerConfig{
		PrimaryOrigin: "https://photos.example.com",
		TLS:           config.TLSConfig{Mode: config.TLSModeACME},
		Proxy:         config.ProxyConfig{Mode: config.ProxyModeDisabled},
	}
	policy, err := New(server, config.PasskeyConfig{Enabled: true, Name: "Lumilio Photos"})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodGet, "https://photos.example.com/probe", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	request.TLS = &tls.ConnectionState{}

	ctx, err := policy.Resolve(request)
	require.NoError(t, err)
	require.Equal(t, "https://photos.example.com", ctx.TargetOrigin)
	require.True(t, ctx.IsSecureForPasskey)
}

func TestInvalidBrowserOriginIsRejected(t *testing.T) {
	policy := newTestPolicy(t, config.ProxyModeDisabled, nil)
	request := httptest.NewRequest(http.MethodGet, "http://localhost:6680/probe", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("Origin", "null")

	_, err := policy.Resolve(request)
	require.True(t, errors.Is(err, ErrInvalidBrowserOrigin))
}

func TestDevelopmentViteOriginCanUseCanonicalPasskeys(t *testing.T) {
	policy, err := New(config.ServerConfig{
		PrimaryOrigin:      "http://localhost:6657",
		CORSAllowedOrigins: []string{"http://localhost:6657"},
		TLS:                config.TLSConfig{Mode: config.TLSModeOff},
		Proxy:              config.ProxyConfig{Mode: config.ProxyModeDisabled},
	}, config.PasskeyConfig{Enabled: true, Name: "Lumilio Photos"})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "http://localhost:6680/api/v1/auth/passkeys/login/options", nil)
	request.RemoteAddr = "127.0.0.1:54321"
	request.Header.Set("Origin", "http://localhost:6657")

	ctx, err := policy.Resolve(request)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:6680", ctx.TargetOrigin)
	require.Equal(t, "http://localhost:6657", ctx.BrowserOrigin)
	available, reason := policy.PasskeyAvailability(ctx)
	require.True(t, available)
	require.Empty(t, reason)
}

func newTestPolicy(t *testing.T, mode config.ProxyMode, trusted []netip.Prefix) *Policy {
	t.Helper()
	tlsMode := config.TLSModeExternal
	primaryOrigin := "https://photos.example.com"
	if mode == config.ProxyModeDisabled {
		tlsMode = config.TLSModeOff
		primaryOrigin = "http://localhost:6680"
	}
	policy, err := New(config.ServerConfig{
		PrimaryOrigin: primaryOrigin,
		TLS:           config.TLSConfig{Mode: tlsMode},
		Proxy: config.ProxyConfig{
			Mode:         mode,
			TrustedCIDRs: trusted,
		},
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
