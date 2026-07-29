package servertransport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"server/config"

	"github.com/caddyserver/certmagic"
	"go.uber.org/zap"
)

type certificateManager interface {
	ManageSync(context.Context, []string) error
	TLSConfig() *tls.Config
	HTTPChallengeHandler(http.Handler) http.Handler
	CertificateStatus() CertificateStatus
	Stop()
}

type certificateManagerFactory func(config.ServerConfig, *zap.Logger) (certificateManager, error)

type certMagicManager struct {
	config   *certmagic.Config
	issuer   *certmagic.ACMEIssuer
	cache    *certmagic.Cache
	hostname string
	stopOnce sync.Once

	statusMutex   sync.RWMutex
	lastManagedAt *time.Time
	lastError     error
}

func newCertMagicManager(cfg config.ServerConfig, logger *zap.Logger) (certificateManager, error) {
	if err := os.MkdirAll(cfg.TLS.StoragePath, 0o700); err != nil {
		return nil, fmt.Errorf("create ACME storage %s: %w", cfg.TLS.StoragePath, err)
	}
	storageInfo, err := os.Stat(cfg.TLS.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("inspect ACME storage %s: %w", cfg.TLS.StoragePath, err)
	}
	if !storageInfo.IsDir() {
		return nil, fmt.Errorf("ACME storage path is not a directory: %s", cfg.TLS.StoragePath)
	}
	if err := os.Chmod(cfg.TLS.StoragePath, 0o700); err != nil {
		return nil, fmt.Errorf("protect ACME storage %s: %w", cfg.TLS.StoragePath, err)
	}

	var magicConfig *certmagic.Config
	cache := certmagic.NewCache(certmagic.CacheOptions{
		GetConfigForCert: func(certmagic.Certificate) (*certmagic.Config, error) {
			if magicConfig == nil {
				return nil, errors.New("CertMagic configuration is not initialized")
			}
			return magicConfig, nil
		},
		Logger: logger,
	})
	magicConfig = certmagic.New(cache, certmagic.Config{
		Storage:           &certmagic.FileStorage{Path: filepath.Clean(cfg.TLS.StoragePath)},
		Logger:            logger,
		DefaultServerName: cfg.TLS.Hostname,
	})
	issuerTemplate := certmagic.DefaultACME
	issuerTemplate.Email = cfg.TLS.Email
	issuerTemplate.Agreed = true
	issuer := certmagic.NewACMEIssuer(magicConfig, issuerTemplate)
	magicConfig.Issuers = []certmagic.Issuer{issuer}

	return &certMagicManager{
		config:   magicConfig,
		issuer:   issuer,
		cache:    cache,
		hostname: cfg.TLS.Hostname,
	}, nil
}

func (m *certMagicManager) ManageSync(ctx context.Context, domains []string) error {
	err := m.config.ManageSync(ctx, domains)
	now := time.Now().UTC()
	m.statusMutex.Lock()
	m.lastManagedAt = &now
	m.lastError = err
	m.statusMutex.Unlock()
	return err
}

func (m *certMagicManager) TLSConfig() *tls.Config {
	return m.config.TLSConfig()
}

func (m *certMagicManager) HTTPChallengeHandler(next http.Handler) http.Handler {
	return m.issuer.HTTPChallengeHandler(next)
}

func (m *certMagicManager) CertificateStatus() CertificateStatus {
	m.statusMutex.RLock()
	status := CertificateStatus{
		Hostname:      m.hostname,
		LastManagedAt: cloneTime(m.lastManagedAt),
	}
	lastError := m.lastError
	m.statusMutex.RUnlock()
	if lastError != nil {
		status.Status = "error"
		return status
	}
	tlsConfig := m.config.TLSConfig()
	if tlsConfig.GetCertificate == nil {
		status.Status = "initializing"
		return status
	}
	certificate, err := tlsConfig.GetCertificate(&tls.ClientHelloInfo{ServerName: m.hostname})
	if err != nil || certificate == nil || len(certificate.Certificate) == 0 {
		status.Status = "initializing"
		return status
	}
	leaf := certificate.Leaf
	if leaf == nil {
		leaf, err = x509.ParseCertificate(certificate.Certificate[0])
		if err != nil {
			status.Status = "error"
			return status
		}
	}
	expiresAt := leaf.NotAfter.UTC()
	status.Status = "active"
	status.ExpiresAt = &expiresAt
	return status
}

func (m *certMagicManager) Stop() {
	m.stopOnce.Do(m.cache.Stop)
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func startACME(
	ctx context.Context,
	cfg config.ServerConfig,
	handler http.Handler,
	logger *zap.Logger,
	newManager certificateManagerFactory,
) (*Runtime, error) {
	manager, err := newManager(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("initialize ACME certificate manager: %w", err)
	}

	httpsListener, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		manager.Stop()
		return nil, fmt.Errorf("listen for ACME HTTPS on %s: %w", cfg.Listen, err)
	}
	httpListener, err := net.Listen("tcp", cfg.TLS.HTTPListen)
	if err != nil {
		_ = httpsListener.Close()
		manager.Stop()
		return nil, fmt.Errorf("listen for ACME HTTP challenge on %s: %w", cfg.TLS.HTTPListen, err)
	}

	httpsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		handler.ServeHTTP(w, r)
	})
	redirectHandler := manager.HTTPChallengeHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + cfg.TLS.Hostname + r.URL.EscapedPath()
		if r.URL.EscapedPath() == "" {
			target += "/"
		}
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusPermanentRedirect)
	}))

	httpsServer := newHTTPServer(cfg.Listen, httpsHandler)
	httpServer := newHTTPServer(cfg.TLS.HTTPListen, redirectHandler)
	runtime := newRuntime(
		[]*http.Server{httpsServer, httpServer},
		[]net.Listener{tls.NewListener(httpsListener, manager.TLSConfig()), httpListener},
		manager,
	)

	if err := manager.ManageSync(ctx, []string{cfg.TLS.Hostname}); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return nil, errors.Join(
			fmt.Errorf("obtain ACME certificate for %s: %w", cfg.TLS.Hostname, err),
			runtime.Shutdown(shutdownCtx),
		)
	}
	return runtime, nil
}
