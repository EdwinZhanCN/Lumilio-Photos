// Package httporigin resolves the request-facing browser origin without
// requiring operators to configure one canonical public URL.
package httporigin

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"server/config"
)

var (
	ErrInvalidPeerAddress   = errors.New("invalid immediate peer address")
	ErrInvalidTargetOrigin  = errors.New("invalid request target origin")
	ErrInvalidBrowserOrigin = errors.New("invalid browser origin")
)

type Policy struct {
	passkey config.PasskeyConfig
	tlsMode config.TLSMode
	trusted []netip.Prefix
	cors    map[string]struct{}
}

type RequestContext struct {
	PeerIP             netip.Addr
	ClientIP           netip.Addr
	TargetOrigin       string
	BrowserOrigin      string
	IsSecureContext    bool
	IsSecureForPasskey bool
}

type PasskeyUnavailableReason string

const (
	PasskeyUnavailableDisabled             PasskeyUnavailableReason = "disabled"
	PasskeyUnavailableSecureOriginRequired PasskeyUnavailableReason = "secure_origin_required"
	PasskeyUnavailableDomainRequired       PasskeyUnavailableReason = "domain_required"
	PasskeyUnavailableInvalidOrigin        PasskeyUnavailableReason = "invalid_request_origin"
)

func New(server config.ServerConfig, passkey config.PasskeyConfig) (*Policy, error) {
	trusted := append([]netip.Prefix(nil), server.Proxy.TrustedCIDRs...)
	cors := make(map[string]struct{}, len(server.CORSAllowedOrigins))
	for _, origin := range server.CORSAllowedOrigins {
		normalizedOrigin, _, err := config.NormalizeOrigin(origin)
		if err != nil || normalizedOrigin != origin {
			return nil, fmt.Errorf("CORS origin %q is not normalized", origin)
		}
		cors[origin] = struct{}{}
	}
	return &Policy{
		passkey: passkey,
		tlsMode: server.TLS.Mode,
		trusted: trusted,
		cors:    cors,
	}, nil
}

func (p *Policy) PasskeyEnabled() bool { return p.passkey.Enabled }
func (p *Policy) TLSMode() config.TLSMode {
	return p.tlsMode
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
	_, browserURL, err := config.NormalizeOrigin(ctx.BrowserOrigin)
	if err != nil {
		return false, PasskeyUnavailableInvalidOrigin
	}
	host := browserURL.Hostname()
	if browserURL.Scheme == "http" {
		if host == "localhost" {
			return true, ""
		}
		return false, PasskeyUnavailableSecureOriginRequired
	}
	if browserURL.Scheme != "https" {
		return false, PasskeyUnavailableSecureOriginRequired
	}
	if net.ParseIP(host) != nil {
		return false, PasskeyUnavailableDomainRequired
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
	target, err := requestTargetOrigin(r)
	if err != nil {
		return RequestContext{PeerIP: peer, ClientIP: peer}, err
	}
	ctx := RequestContext{
		PeerIP:       peer,
		ClientIP:     peer,
		TargetOrigin: target,
	}
	if p.isTrusted(peer) {
		ctx.ClientIP = p.forwardedClientIP(r.Header, peer)
	}
	return p.finishContext(ctx, r)
}

// ResolveLoopbackHealth remains a separate entry point for router compatibility,
// but health requests now follow the same request-derived origin policy.
func (p *Policy) ResolveLoopbackHealth(r *http.Request) (RequestContext, error) {
	return p.Resolve(r)
}

func (p *Policy) finishContext(ctx RequestContext, r *http.Request) (RequestContext, error) {
	ctx.BrowserOrigin = ctx.TargetOrigin
	originValues := r.Header.Values("Origin")
	if len(originValues) != 0 {
		if len(originValues) != 1 {
			return ctx, ErrInvalidBrowserOrigin
		}
		rawOrigin := strings.TrimSpace(originValues[0])
		if rawOrigin == "" || strings.EqualFold(rawOrigin, "null") || strings.Contains(rawOrigin, ",") {
			return ctx, ErrInvalidBrowserOrigin
		}
		normalized, _, err := config.NormalizeOrigin(rawOrigin)
		if err != nil {
			return ctx, fmt.Errorf("%w: %v", ErrInvalidBrowserOrigin, err)
		}
		ctx.BrowserOrigin = normalized
	}
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

func requestTargetOrigin(r *http.Request) (string, error) {
	directScheme := "http"
	if r.TLS != nil {
		directScheme = "https"
	}
	scheme := firstForwardedParameter(r.Header.Values("Forwarded"), "proto", validScheme)
	if scheme == "" {
		scheme = firstHeaderCandidate(r.Header.Values("X-Forwarded-Proto"), validScheme)
	}
	if scheme == "" {
		scheme = directScheme
	}

	host := firstForwardedParameter(r.Header.Values("Forwarded"), "host", validOriginHost)
	if host == "" {
		host = firstHeaderCandidate(r.Header.Values("X-Forwarded-Host"), validOriginHost)
	}
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	normalized, _, err := config.NormalizeOrigin(strings.ToLower(scheme) + "://" + host)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidTargetOrigin, err)
	}
	return normalized, nil
}

func validScheme(value string) bool {
	return strings.EqualFold(value, "http") || strings.EqualFold(value, "https")
}

func validOriginHost(value string) bool {
	_, _, err := config.NormalizeOrigin("http://" + strings.TrimSpace(value))
	return err == nil
}

func firstHeaderCandidate(values []string, valid func(string) bool) string {
	for _, line := range values {
		for _, candidate := range strings.Split(line, ",") {
			candidate = strings.TrimSpace(candidate)
			if valid(candidate) {
				return candidate
			}
		}
	}
	return ""
}

func firstForwardedParameter(values []string, wanted string, valid func(string) bool) string {
	for _, line := range values {
		for _, element := range strings.Split(line, ",") {
			for _, parameter := range strings.Split(element, ";") {
				name, value, found := strings.Cut(parameter, "=")
				if !found || !strings.EqualFold(strings.TrimSpace(name), wanted) {
					continue
				}
				value, err := unquoteForwardedValue(strings.TrimSpace(value))
				if err == nil && valid(value) {
					return value
				}
			}
		}
	}
	return ""
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

func (p *Policy) forwardedClientIP(header http.Header, peer netip.Addr) netip.Addr {
	raw := strings.TrimSpace(header.Get("X-Forwarded-For"))
	if raw == "" {
		return peer
	}
	parts := strings.Split(raw, ",")
	chain := make([]netip.Addr, 0, len(parts)+1)
	for _, part := range parts {
		addr, err := parseForwardedAddress(part)
		if err != nil {
			return peer
		}
		chain = append(chain, addr)
	}
	chain = append(chain, peer)
	for i := len(chain) - 1; i >= 0; i-- {
		if !p.isTrusted(chain[i]) {
			return chain[i]
		}
	}
	return chain[0]
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
