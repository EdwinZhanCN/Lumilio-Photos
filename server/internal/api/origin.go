package api

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// NormalizeOrigin parses an exact HTTP(S) origin and removes only a trailing
// slash. Paths, credentials, query strings, and fragments are not origins.
func NormalizeOrigin(value string) (string, *url.URL, bool) {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil {
		return "", nil, false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if (scheme != "http" && scheme != "https") ||
		parsed.Host == "" || parsed.Hostname() == "" ||
		parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" {
		return "", nil, false
	}

	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}

	parsed.Scheme = scheme
	parsed.Host = host
	parsed.Path = ""
	parsed.RawPath = ""
	return parsed.String(), parsed, true
}

// RequestTargetOrigin reconstructs the browser-facing target origin. Reverse
// proxies are expected to overwrite X-Forwarded-Proto/Host rather than append
// untrusted client values.
func RequestTargetOrigin(r *http.Request) (string, *url.URL, bool) {
	if r == nil {
		return "", nil, false
	}

	scheme := firstForwardedValue(r.Header.Get("X-Forwarded-Proto"))
	if scheme != "http" && scheme != "https" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	host := firstForwardedValue(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	return NormalizeOrigin(scheme + "://" + host)
}

// IsSameRequestOrigin compares scheme, host, and port against the reconstructed
// browser-facing request target.
func IsSameRequestOrigin(r *http.Request, candidate string) bool {
	normalized, _, ok := NormalizeOrigin(candidate)
	if !ok {
		return false
	}
	target, _, ok := RequestTargetOrigin(r)
	return ok && normalized == target
}

func firstForwardedValue(value string) string {
	if comma := strings.IndexByte(value, ','); comma >= 0 {
		value = value[:comma]
	}
	return strings.ToLower(strings.TrimSpace(value))
}
