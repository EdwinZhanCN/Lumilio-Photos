package servertransport

import "net/http"

// startExternal intentionally serves internal plaintext HTTP. The manifest
// requires proxy.mode=required for this TLS mode, and OriginPolicy rejects
// every application request that did not arrive through a trusted proxy.
func startExternal(addr string, handler http.Handler) (*Runtime, error) {
	return startHTTP(addr, handler, nil)
}
