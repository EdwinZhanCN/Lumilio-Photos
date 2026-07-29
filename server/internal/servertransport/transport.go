// Package servertransport owns the HTTP/TLS listeners for one application
// runtime generation. It keeps transport lifecycle separate from request
// origin policy: plaintext upstreams may sit behind any conventional reverse
// proxy, while ACME TLS is terminated in-process.
package servertransport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"server/config"

	"go.uber.org/zap"
)

const (
	readHeaderTimeout = 10 * time.Second
	idleTimeout       = 2 * time.Minute
	maxHeaderBytes    = 1 << 20
)

// Runtime owns every listener and HTTP server for one application generation.
type Runtime struct {
	servers   []*http.Server
	listeners []net.Listener
	manager   certificateManager

	waitGroup sync.WaitGroup
	done      chan struct{}

	errorMutex sync.Mutex
	firstError error

	shutdownOnce sync.Once
	shutdownErr  error
}

// CertificateStatus is a non-secret snapshot suitable for runtime diagnostics.
type CertificateStatus struct {
	Hostname      string
	Status        string
	ExpiresAt     *time.Time
	LastManagedAt *time.Time
}

// Start binds and starts the configured transport. ACME certificate
// acquisition is synchronous: a failure closes both listeners and is returned
// to the caller instead of degrading to plaintext HTTP.
func Start(
	ctx context.Context,
	cfg config.ServerConfig,
	handler http.Handler,
	logger *zap.Logger,
) (*Runtime, error) {
	return start(ctx, cfg, handler, logger, newCertMagicManager)
}

func start(
	ctx context.Context,
	cfg config.ServerConfig,
	handler http.Handler,
	logger *zap.Logger,
	newManager certificateManagerFactory,
) (*Runtime, error) {
	switch cfg.TLS.Mode {
	case config.TLSModeOff:
		return startHTTP(cfg.Listen, handler, nil)
	case config.TLSModeACME:
		return startACME(ctx, cfg, handler, logger, newManager)
	default:
		return nil, fmt.Errorf("unsupported TLS mode %q", cfg.TLS.Mode)
	}
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}

func newRuntime(
	servers []*http.Server,
	listeners []net.Listener,
	manager certificateManager,
) *Runtime {
	runtime := &Runtime{
		servers:   servers,
		listeners: listeners,
		manager:   manager,
		done:      make(chan struct{}),
	}
	runtime.waitGroup.Add(len(servers))
	for index := range servers {
		server := servers[index]
		listener := listeners[index]
		go func() {
			defer runtime.waitGroup.Done()
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
				runtime.recordError(err)
				runtime.closeListeners()
			}
		}()
	}
	go func() {
		runtime.waitGroup.Wait()
		close(runtime.done)
	}()
	return runtime
}

func (r *Runtime) recordError(err error) {
	r.errorMutex.Lock()
	defer r.errorMutex.Unlock()
	if r.firstError == nil {
		r.firstError = err
	}
}

func (r *Runtime) closeListeners() {
	for _, listener := range r.listeners {
		_ = listener.Close()
	}
}

// Wait blocks until every listener has stopped and returns the first
// unexpected serving error, if any.
func (r *Runtime) Wait() error {
	<-r.done
	r.errorMutex.Lock()
	defer r.errorMutex.Unlock()
	return r.firstError
}

// CertificateStatus reports current in-process TLS certificate state.
func (r *Runtime) CertificateStatus() CertificateStatus {
	if r.manager == nil {
		return CertificateStatus{Status: "not_applicable"}
	}
	return r.manager.CertificateStatus()
}

// Shutdown gracefully drains all HTTP servers, closes listeners, stops
// CertMagic maintenance, and waits for serving goroutines to exit.
func (r *Runtime) Shutdown(ctx context.Context) error {
	r.shutdownOnce.Do(func() {
		var shutdownErrors []error
		for _, server := range r.servers {
			if err := server.Shutdown(ctx); err != nil {
				shutdownErrors = append(shutdownErrors, err)
			}
		}
		r.closeListeners()
		if r.manager != nil {
			r.manager.Stop()
		}
		select {
		case <-r.done:
		case <-ctx.Done():
			shutdownErrors = append(shutdownErrors, ctx.Err())
		}
		r.shutdownErr = errors.Join(shutdownErrors...)
	})
	return r.shutdownErr
}
