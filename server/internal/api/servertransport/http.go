package servertransport

import (
	"fmt"
	"net"
	"net/http"
)

func startHTTP(addr string, handler http.Handler, manager certificateManager) (*Runtime, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	server := newHTTPServer(addr, handler)
	return newRuntime([]*http.Server{server}, []net.Listener{listener}, manager), nil
}
