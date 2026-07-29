package servertransport

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"server/config"

	"go.uber.org/zap"
)

func TestOffServesHTTP(t *testing.T) {
	runtime, err := Start(context.Background(), config.ServerConfig{
		Listen: "127.0.0.1:0",
		TLS:    config.TLSConfig{Mode: config.TLSModeOff},
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), zap.NewNop())
	if err != nil {
		t.Fatalf("start transport: %v", err)
	}
	t.Cleanup(func() { shutdownRuntime(t, runtime) })

	response, err := http.Get("http://" + runtime.listeners[0].Addr().String())
	if err != nil {
		t.Fatalf("GET HTTP: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestACMEServesHTTPSChallengeAndRedirect(t *testing.T) {
	manager := newFakeCertificateManager(t)
	cfg := config.ServerConfig{
		Listen: "127.0.0.1:0",
		TLS: config.TLSConfig{
			Hostname:   "photos.example.com",
			Mode:       config.TLSModeACME,
			HTTPListen: "127.0.0.1:0",
		},
	}
	runtime, err := start(
		context.Background(),
		cfg,
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
		zap.NewNop(),
		func(config.ServerConfig, *zap.Logger) (certificateManager, error) {
			return manager, nil
		},
	)
	if err != nil {
		t.Fatalf("start ACME transport: %v", err)
	}
	t.Cleanup(func() { shutdownRuntime(t, runtime) })

	if len(manager.managedDomains) != 1 || manager.managedDomains[0] != "photos.example.com" {
		t.Fatalf("managed domains = %#v", manager.managedDomains)
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Test fixture certificate.
		},
	}
	response, err := client.Get("https://" + runtime.listeners[0].Addr().String())
	if err != nil {
		t.Fatalf("GET HTTPS: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("HTTPS status = %d", response.StatusCode)
	}
	if got := response.Header.Get("Strict-Transport-Security"); got != "max-age=31536000" {
		t.Fatalf("HSTS = %q", got)
	}

	httpAddr := runtime.listeners[1].Addr().String()
	challengeResponse, err := http.Get("http://" + httpAddr + "/.well-known/acme-challenge/test-token")
	if err != nil {
		t.Fatalf("GET challenge: %v", err)
	}
	challengeResponse.Body.Close()
	if challengeResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("challenge status = %d", challengeResponse.StatusCode)
	}

	redirectClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	redirectResponse, err := redirectClient.Get("http://" + httpAddr + "/albums/夏?sort=new")
	if err != nil {
		t.Fatalf("GET redirect: %v", err)
	}
	redirectResponse.Body.Close()
	if redirectResponse.StatusCode != http.StatusPermanentRedirect {
		t.Fatalf("redirect status = %d", redirectResponse.StatusCode)
	}
	if got, want := redirectResponse.Header.Get("Location"), "https://photos.example.com/albums/%E5%A4%8F?sort=new"; got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
}

func TestACMEAcquisitionFailureClosesBothListeners(t *testing.T) {
	manager := newFakeCertificateManager(t)
	manager.manageErr = errors.New("fixture issuer unavailable")
	httpsAddr := unusedAddress(t)
	httpAddr := unusedAddress(t)

	runtime, err := start(
		context.Background(),
		config.ServerConfig{
			Listen: httpsAddr,
			TLS: config.TLSConfig{
				Hostname:   "photos.example.com",
				Mode:       config.TLSModeACME,
				HTTPListen: httpAddr,
			},
		},
		http.NotFoundHandler(),
		zap.NewNop(),
		func(config.ServerConfig, *zap.Logger) (certificateManager, error) {
			return manager, nil
		},
	)
	if runtime != nil {
		t.Fatal("runtime must be nil after certificate failure")
	}
	if err == nil || !errors.Is(err, manager.manageErr) {
		t.Fatalf("start error = %v", err)
	}
	if !manager.stopped {
		t.Fatal("certificate manager was not stopped")
	}
	assertAddressReusable(t, httpsAddr)
	assertAddressReusable(t, httpAddr)
}

func TestACMEShutdownReleasesBothPorts(t *testing.T) {
	manager := newFakeCertificateManager(t)
	httpsAddr := unusedAddress(t)
	httpAddr := unusedAddress(t)
	runtime, err := start(
		context.Background(),
		config.ServerConfig{
			Listen: httpsAddr,
			TLS: config.TLSConfig{
				Hostname:   "photos.example.com",
				Mode:       config.TLSModeACME,
				HTTPListen: httpAddr,
			},
		},
		http.NotFoundHandler(),
		zap.NewNop(),
		func(config.ServerConfig, *zap.Logger) (certificateManager, error) {
			return manager, nil
		},
	)
	if err != nil {
		t.Fatalf("start ACME transport: %v", err)
	}
	shutdownRuntime(t, runtime)
	if !manager.stopped {
		t.Fatal("certificate manager was not stopped")
	}
	assertAddressReusable(t, httpsAddr)
	assertAddressReusable(t, httpAddr)
}

func TestCertMagicStoragePathErrorIsDiagnostic(t *testing.T) {
	parent := t.TempDir()
	notDirectory := filepath.Join(parent, "storage-file")
	if err := os.WriteFile(notDirectory, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := newCertMagicManager(config.ServerConfig{
		TLS: config.TLSConfig{
			Hostname:    "photos.example.com",
			Email:       "admin@example.com",
			StoragePath: notDirectory,
		},
	}, zap.NewNop())
	if err == nil || !containsAll(err.Error(), "ACME storage", notDirectory) {
		t.Fatalf("storage error = %v", err)
	}
}

type fakeCertificateManager struct {
	tlsConfig      *tls.Config
	managedDomains []string
	manageErr      error
	stopped        bool
	stopMutex      sync.Mutex
}

func newFakeCertificateManager(t *testing.T) *fakeCertificateManager {
	t.Helper()
	return &fakeCertificateManager{tlsConfig: fixtureTLSConfig(t)}
}

func (m *fakeCertificateManager) ManageSync(_ context.Context, domains []string) error {
	m.managedDomains = append([]string(nil), domains...)
	return m.manageErr
}

func (m *fakeCertificateManager) TLSConfig() *tls.Config {
	return m.tlsConfig
}

func (m *fakeCertificateManager) HTTPChallengeHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/acme-challenge/test-token" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *fakeCertificateManager) CertificateStatus() CertificateStatus {
	status := CertificateStatus{
		Hostname: "photos.example.com",
		Status:   "active",
	}
	if len(m.tlsConfig.Certificates) > 0 && len(m.tlsConfig.Certificates[0].Certificate) > 0 {
		if leaf, err := x509.ParseCertificate(m.tlsConfig.Certificates[0].Certificate[0]); err == nil {
			expiresAt := leaf.NotAfter.UTC()
			status.ExpiresAt = &expiresAt
		}
	}
	return status
}

func (m *fakeCertificateManager) Stop() {
	m.stopMutex.Lock()
	defer m.stopMutex.Unlock()
	m.stopped = true
}

func fixtureTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "photos.example.com"},
		DNSNames:     []string{"photos.example.com"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{der},
			PrivateKey:  privateKey,
		}},
		MinVersion: tls.VersionTLS12,
	}
}

func shutdownRuntime(t *testing.T, runtime *Runtime) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown runtime: %v", err)
	}
}

func unusedAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func assertAddressReusable(t *testing.T, address string) {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatalf("address %s was not released: %v", address, err)
	}
	_ = listener.Close()
}

func containsAll(value string, fragments ...string) bool {
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			return false
		}
	}
	return true
}
