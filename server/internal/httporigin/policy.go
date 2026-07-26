// Package httporigin owns Lumilio's canonical browser origin and trusted-proxy
// request interpretation. Security-sensitive consumers must use one resolved
// RequestContext instead of independently reading Host or forwarding headers.
package httporigin

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"

	"server/config"
)

var (
	ErrTrustedProxyRequired = errors.New("trusted proxy required")
	ErrInvalidPeerAddress   = errors.New("invalid immediate peer address")
	ErrInvalidTargetOrigin  = errors.New("invalid request target origin")
	ErrInvalidBrowserOrigin = errors.New("invalid browser origin")
	ErrInvalidForwarded     = errors.New("invalid forwarded headers")
	ErrMisdirectedRequest   = errors.New("request target is not the primary origin")
)

type Policy struct {
	primaryOrigin string
	primaryURL    *url.URL
	rpID          string
	passkey       config.PasskeyConfig
	tlsMode       config.TLSMode
	proxyMode     config.ProxyMode
	trusted       []netip.Prefix
	cors          map[string]struct{}
}

type RequestContext struct {
	PeerIP             netip.Addr
	ClientIP           netip.Addr
	ViaTrustedProxy    bool
	TargetOrigin       string
	BrowserOrigin      string
	IsPrimaryOrigin    bool
	IsSecureContext    bool
	IsSecureForPasskey bool
}

type PasskeyUnavailableReason string

const (
	PasskeyUnavailableDisabled             PasskeyUnavailableReason = "disabled"
	PasskeyUnavailableSecureOriginRequired PasskeyUnavailableReason = "secure_origin_required"
	PasskeyUnavailableNonPrimaryOrigin     PasskeyUnavailableReason = "non_primary_origin"
	PasskeyUnavailableTrustedProxyRequired PasskeyUnavailableReason = "trusted_proxy_required"
	PasskeyUnavailableInvalidOrigin        PasskeyUnavailableReason = "invalid_request_origin"
)

func New(server config.ServerConfig, passkey config.PasskeyConfig) (*Policy, error) {
	normalized, primaryURL, err := config.NormalizeOrigin(server.PrimaryOrigin)
	if err != nil {
		return nil, fmt.Errorf("primary origin: %w", err)
	}
	if normalized != server.PrimaryOrigin {
		return nil, errors.New("primary origin must already be normalized")
	}
	trusted := append([]netip.Prefix(nil), server.Proxy.TrustedCIDRs...)
	cors := make(map[string]struct{}, len(server.CORSAllowedOrigins))
	for _, origin := range server.CORSAllowedOrigins {
		normalizedOrigin, _, normalizeErr := config.NormalizeOrigin(origin)
		if normalizeErr != nil || normalizedOrigin != origin {
			return nil, fmt.Errorf("CORS origin %q is not normalized", origin)
		}
		cors[origin] = struct{}{}
	}
	return &Policy{
		primaryOrigin: normalized,
		primaryURL:    primaryURL,
		rpID:          primaryURL.Hostname(),
		passkey:       passkey,
		tlsMode:       server.TLS.Mode,
		proxyMode:     server.Proxy.Mode,
		trusted:       trusted,
		cors:          cors,
	}, nil
}

func (p *Policy) PrimaryOrigin() string { return p.primaryOrigin }
func (p *Policy) RPID() string          { return p.rpID }
func (p *Policy) PasskeyEnabled() bool  { return p.passkey.Enabled }
func (p *Policy) TLSMode() config.TLSMode {
	return p.tlsMode
}
func (p *Policy) ProxyMode() config.ProxyMode {
	return p.proxyMode
}

func (p *Policy) TrustedProxyCIDRs() []netip.Prefix {
	return append([]netip.Prefix(nil), p.trusted...)
}

func (p *Policy) IsCORSAllowed(origin string) bool {
	normalized, _, err := config.NormalizeOrigin(origin)
	if err != nil {
		return false
	}
	_, ok := p.cors[normalized]
	return ok
}

func (p *Policy) PasskeyAvailability(ctx RequestContext) (bool, PasskeyUnavailableReason) {
	if !p.passkey.Enabled {
		return false, PasskeyUnavailableDisabled
	}
	if p.proxyMode == config.ProxyModeRequired && !ctx.ViaTrustedProxy {
		return false, PasskeyUnavailableTrustedProxyRequired
	}
	_, browserURL, err := config.NormalizeOrigin(ctx.BrowserOrigin)
	if err != nil {
		return false, PasskeyUnavailableInvalidOrigin
	}
	secureContext := browserURL.Scheme == "https" ||
		(browserURL.Scheme == "http" && browserURL.Hostname() == "localhost")
	if !secureContext {
		return false, PasskeyUnavailableSecureOriginRequired
	}
	if ctx.BrowserOrigin != p.primaryOrigin {
		return false, PasskeyUnavailableNonPrimaryOrigin
	}
	return true, ""
}

func (p *Policy) Resolve(r *http.Request) (RequestContext, error) {
	if r == nil {
		return RequestContext{}, ErrInvalidTargetOrigin
	}
	peer, err := parseRemoteAddr(r.RemoteAddr)
	if err != nil {
		return RequestContext{}, fmt.Errorf("%w: %v", ErrInvalidPeerAddress, err)
	}
	ctx := RequestContext{
		PeerIP:   peer,
		ClientIP: peer,
	}

	if p.proxyMode == config.ProxyModeRequired {
		if !p.isTrusted(peer) {
			return ctx, ErrTrustedProxyRequired
		}
		ctx.ViaTrustedProxy = true
		ctx.TargetOrigin, err = forwardedTargetOrigin(r.Header)
		if err != nil {
			return ctx, err
		}
		ctx.ClientIP, err = p.forwardedClientIP(r.Header, peer)
		if err != nil {
			return ctx, err
		}
		if ctx.TargetOrigin != p.primaryOrigin {
			return ctx, ErrMisdirectedRequest
		}
	} else {
		ctx.TargetOrigin, err = directTargetOrigin(r)
		if err != nil {
			return ctx, err
		}
	}

	return p.finishContext(ctx, r)
}

// ResolveLoopbackHealth is the sole direct-request exception for a
// proxy-required deployment. Callers must additionally restrict it to the
// documented liveness/readiness routes.
func (p *Policy) ResolveLoopbackHealth(r *http.Request) (RequestContext, error) {
	if r == nil {
		return RequestContext{}, ErrInvalidTargetOrigin
	}
	peer, err := parseRemoteAddr(r.RemoteAddr)
	if err != nil {
		return RequestContext{}, fmt.Errorf("%w: %v", ErrInvalidPeerAddress, err)
	}
	if !peer.IsLoopback() {
		return RequestContext{PeerIP: peer, ClientIP: peer}, ErrTrustedProxyRequired
	}
	target, err := directTargetOrigin(r)
	if err != nil {
		return RequestContext{}, err
	}
	return p.finishContext(RequestContext{
		PeerIP:       peer,
		ClientIP:     peer,
		TargetOrigin: target,
	}, r)
}

func (p *Policy) finishContext(ctx RequestContext, r *http.Request) (RequestContext, error) {
	ctx.BrowserOrigin = ctx.TargetOrigin
	if rawOrigin := strings.TrimSpace(r.Header.Get("Origin")); rawOrigin != "" {
		normalized, _, err := config.NormalizeOrigin(rawOrigin)
		if err != nil {
			return ctx, fmt.Errorf("%w: %v", ErrInvalidBrowserOrigin, err)
		}
		ctx.BrowserOrigin = normalized
	}
	ctx.IsPrimaryOrigin = ctx.BrowserOrigin == p.primaryOrigin
	_, browserURL, _ := config.NormalizeOrigin(ctx.BrowserOrigin)
	ctx.IsSecureContext = browserURL != nil &&
		(browserURL.Scheme == "https" ||
			(browserURL.Scheme == "http" && browserURL.Hostname() == "localhost"))
	ctx.IsSecureForPasskey, _ = p.PasskeyAvailability(ctx)
	return ctx, nil
}

func (p *Policy) isTrusted(addr netip.Addr) bool {
	for _, prefix := range p.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func parseRemoteAddr(value string) (netip.Addr, error) {
	value = strings.TrimSpace(value)
	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Unmap(), nil
	}
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		return netip.Addr{}, err
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, err
	}
	return addr.Unmap(), nil
}

func directTargetOrigin(r *http.Request) (string, error) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	normalized, _, err := config.NormalizeOrigin(scheme + "://" + strings.TrimSpace(r.Host))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidTargetOrigin, err)
	}
	return normalized, nil
}

func forwardedTargetOrigin(header http.Header) (string, error) {
	forwardedProto, forwardedHost, forwardedPresent, err := parseForwardedTarget(header.Values("Forwarded"))
	if err != nil {
		return "", err
	}
	xProto, xProtoPresent, err := singleHeaderValue(header.Values("X-Forwarded-Proto"))
	if err != nil {
		return "", fmt.Errorf("%w: X-Forwarded-Proto must contain one value", ErrInvalidForwarded)
	}
	xHost, xHostPresent, err := singleHeaderValue(header.Values("X-Forwarded-Host"))
	if err != nil {
		return "", fmt.Errorf("%w: X-Forwarded-Host must contain one value", ErrInvalidForwarded)
	}
	if xProtoPresent != xHostPresent {
		return "", fmt.Errorf("%w: forwarded proto and host must be provided together", ErrInvalidForwarded)
	}
	xPresent := xProtoPresent && xHostPresent
	if !forwardedPresent && !xPresent {
		return "", fmt.Errorf("%w: trusted proxy did not provide a public proto and host", ErrInvalidForwarded)
	}

	var forwardedOrigin string
	if forwardedPresent {
		forwardedOrigin, _, err = config.NormalizeOrigin(strings.ToLower(forwardedProto) + "://" + forwardedHost)
		if err != nil {
			return "", fmt.Errorf("%w: invalid Forwarded target", ErrInvalidForwarded)
		}
	}
	var xOrigin string
	if xPresent {
		xOrigin, _, err = config.NormalizeOrigin(strings.ToLower(xProto) + "://" + xHost)
		if err != nil {
			return "", fmt.Errorf("%w: invalid X-Forwarded target", ErrInvalidForwarded)
		}
	}
	if forwardedPresent && xPresent && forwardedOrigin != xOrigin {
		return "", fmt.Errorf("%w: Forwarded and X-Forwarded targets disagree", ErrInvalidForwarded)
	}
	if forwardedPresent {
		return forwardedOrigin, nil
	}
	return xOrigin, nil
}

func parseForwardedTarget(values []string) (proto, host string, present bool, err error) {
	if len(values) == 0 {
		return "", "", false, nil
	}
	raw, ok, err := singleHeaderValue(values)
	if err != nil || !ok || strings.Contains(raw, ",") {
		return "", "", false, fmt.Errorf("%w: Forwarded must contain one element", ErrInvalidForwarded)
	}
	for _, parameter := range strings.Split(raw, ";") {
		name, value, found := strings.Cut(parameter, "=")
		if !found {
			return "", "", false, fmt.Errorf("%w: malformed Forwarded parameter", ErrInvalidForwarded)
		}
		name = strings.ToLower(strings.TrimSpace(name))
		value, err = unquoteForwardedValue(strings.TrimSpace(value))
		if err != nil {
			return "", "", false, fmt.Errorf("%w: malformed Forwarded value", ErrInvalidForwarded)
		}
		switch name {
		case "proto":
			if proto != "" {
				return "", "", false, fmt.Errorf("%w: duplicate Forwarded proto", ErrInvalidForwarded)
			}
			proto = value
		case "host":
			if host != "" {
				return "", "", false, fmt.Errorf("%w: duplicate Forwarded host", ErrInvalidForwarded)
			}
			host = value
		}
	}
	if proto == "" || host == "" {
		return "", "", false, fmt.Errorf("%w: Forwarded requires proto and host", ErrInvalidForwarded)
	}
	return proto, host, true, nil
}

func unquoteForwardedValue(value string) (string, error) {
	if !strings.HasPrefix(value, `"`) {
		if strings.ContainsAny(value, " \t\"") {
			return "", errors.New("invalid token")
		}
		return value, nil
	}
	unquoted, err := strconv.Unquote(value)
	if err != nil {
		return "", err
	}
	return unquoted, nil
}

func singleHeaderValue(values []string) (string, bool, error) {
	if len(values) == 0 {
		return "", false, nil
	}
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return "", false, errors.New("multiple values")
	}
	value := strings.TrimSpace(values[0])
	if value == "" {
		return "", false, errors.New("empty value")
	}
	return value, true, nil
}

func (p *Policy) forwardedClientIP(header http.Header, peer netip.Addr) (netip.Addr, error) {
	raw := strings.TrimSpace(header.Get("X-Forwarded-For"))
	if raw == "" {
		return peer, nil
	}
	parts := strings.Split(raw, ",")
	chain := make([]netip.Addr, 0, len(parts)+1)
	for _, part := range parts {
		addr, err := parseForwardedAddress(part)
		if err != nil {
			return netip.Addr{}, fmt.Errorf("%w: malformed X-Forwarded-For", ErrInvalidForwarded)
		}
		chain = append(chain, addr)
	}
	chain = append(chain, peer)
	for i := len(chain) - 1; i >= 0; i-- {
		if !p.isTrusted(chain[i]) {
			return chain[i], nil
		}
	}
	return chain[0], nil
}

func parseForwardedAddress(value string) (netip.Addr, error) {
	value = strings.TrimSpace(value)
	if addr, err := netip.ParseAddr(value); err == nil {
		return addr.Unmap(), nil
	}
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		return netip.Addr{}, err
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, err
	}
	return addr.Unmap(), nil
}
