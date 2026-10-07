// Package httporigin derives the request-facing browser origin, so operators
// never configure one canonical public URL. [Policy.Resolve] normalizes
// Forwarded and X-Forwarded-* target metadata and the browser Origin into a
// [RequestContext]; server.proxy.trusted_cidrs is used only to recover the
// client IP from X-Forwarded-For. Reverse proxies must overwrite, not append,
// X-Forwarded-Proto and X-Forwarded-Host.
//
// Same-origin access is dynamic and zero-config, including LAN addresses,
// public domains behind a proxy, and the Desktop at http://localhost:6680.
// server.cors_allowed_origins is only the exact allowlist for credentialed
// cross-origin sessions ([Policy.IsCORSAllowed]); other origins may still call
// public or Bearer endpoints but never receive Allow-Credentials. Passkey
// challenges bind the normalized Origin and its hostname RP ID
// ([Policy.PasskeyAvailability]).
//
//atlas:group http
package httporigin
