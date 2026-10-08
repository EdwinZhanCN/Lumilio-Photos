// Package ratelimit provides the bounded, in-memory fixed-window [Limiter]
// with lockout used for authentication endpoints. A [Policy] fixes attempts,
// window, lockout, and the maximum tracked keys so memory stays bounded.
//
//atlas:group http
package ratelimit
