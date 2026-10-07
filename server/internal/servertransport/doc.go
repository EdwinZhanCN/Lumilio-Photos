// Package servertransport owns the HTTP/TLS listeners for one application
// runtime generation. It keeps transport lifecycle separate from request
// origin policy: plaintext upstreams may sit behind any conventional reverse
// proxy, while ACME TLS is terminated in-process.
//
//atlas:group http
package servertransport
