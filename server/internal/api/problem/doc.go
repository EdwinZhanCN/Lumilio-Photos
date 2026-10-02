// Package problem defines Lumilio's language-neutral RFC 9457 vocabulary.
//
// It is intentionally a leaf package: handlers and transports may depend on
// these descriptors, while domain packages remain unaware of HTTP problem URIs.
//
//atlas:group http
package problem
