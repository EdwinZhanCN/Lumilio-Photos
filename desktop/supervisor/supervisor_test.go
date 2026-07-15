package supervisor

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckPortAvailable(t *testing.T) {
	s := New(Options{Logf: func(string, ...any) {}})

	// Occupy the port the same way the server binds it (all interfaces), matching
	// the real conflict the pre-flight guards against.
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Skipf("sandbox does not permit loopback listeners: %v", err)
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	if err := s.checkPortAvailable(port); !errors.Is(err, ErrPortInUse) {
		t.Errorf("occupied port: got %v, want ErrPortInUse", err)
	}

	// Once freed, the same port is available again.
	_ = ln.Close()
	if err := s.checkPortAvailable(port); err != nil {
		t.Errorf("freed port: got %v, want nil", err)
	}
}

// The generated desktop TOML is written privately and then loaded through the
// same strict server/config boundary used by standalone.
func TestDesktopServerConfigInvariants(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.toml")
	bootstrap := filepath.Join(dir, "bootstrap")
	if err := os.WriteFile(bootstrap, []byte("bootstrap-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := compileAndLoadServerManifest(path, serverManifestBindings{
		Port: "6680", BrowserOrigin: "http://localhost:6680", WebRoot: "/bundle/web",
		LogDir: "/Users/me/Library/Application Support/Lumilio Photos/logs", StoragePath: "/Volumes/Photos/Lumilio Library",
		DBHost: "/Users/me/Library/Application Support/Lumilio Photos/postgres/18/run", DBPort: "5487", DBUser: "lumilio", DBName: "lumiliophotos",
		BootstrapPasswordFile: bootstrap, RotatedPasswordFile: filepath.Join(dir, "rotated"), SecretKeyFile: "/secrets/lumilio_secret_key",
		PGBinDir: "/bundle/postgres/bin", ExifToolPath: "/bundle/exiftool", FFmpegPath: "/bundle/ffmpeg", FFprobePath: "/bundle/ffprobe",
		LumenStaticNode: "127.0.0.1:50051",
	})
	if err != nil {
		t.Fatalf("compileAndLoadServerManifest: %v", err)
	}
	if cfg.Auth.WebAuthnRPID != "localhost" {
		t.Fatalf("webauthn rp id = %q, want localhost", cfg.Auth.WebAuthnRPID)
	}
	if got, want := strings.Join(cfg.Auth.WebAuthnRPOrigins, ","), "http://localhost:6680"; got != want {
		t.Fatalf("webauthn origins = %q, want %q", got, want)
	}
	if cfg.DatabaseConfig.Host != "/Users/me/Library/Application Support/Lumilio Photos/postgres/18/run" {
		t.Fatalf("database host = %q", cfg.DatabaseConfig.Host)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "/bundle/web") || strings.Contains(string(data), "bootstrap-secret") {
		t.Fatalf("generated manifest must contain bindings but no secret content:\n%s", data)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("manifest mode = %v, err=%v", info.Mode().Perm(), err)
	}
	if !cfg.LoadedFromManifest() || cfg.ManifestPath != path || cfg.ServerConfig.Port != "6680" {
		t.Fatalf("manifest was not strict-loaded: %+v", cfg)
	}
	if cfg.Auth.WebAuthnRPID != "localhost" || strings.Join(cfg.Auth.WebAuthnRPOrigins, ",") != "http://localhost:6680" {
		t.Fatalf("unexpected auth config: %+v", cfg.Auth)
	}
	if cfg.DatabaseConfig.Host != "/Users/me/Library/Application Support/Lumilio Photos/postgres/18/run" || cfg.Tools.FFmpegPath != "/bundle/ffmpeg" {
		t.Fatalf("unexpected generated config: db=%+v tools=%+v", cfg.DatabaseConfig, cfg.Tools)
	}
}

func TestDesktopManifestWriteFailureBlocksLoad(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := compileAndLoadServerManifest(filepath.Join(blocker, "server.toml"), serverManifestBindings{})
	if err == nil || !strings.Contains(err.Error(), "write desktop server manifest") {
		t.Fatalf("expected write failure, got %v", err)
	}
}

func TestDesktopTemplateRejectsMissingBindingsOnStrictReload(t *testing.T) {
	_, err := compileAndLoadServerManifest(filepath.Join(t.TempDir(), "server.toml"), serverManifestBindings{})
	if err == nil || !strings.Contains(err.Error(), "reload generated desktop server manifest") {
		t.Fatalf("expected incomplete bindings to fail strict reload, got %v", err)
	}
}

func TestDesktopSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop-settings.json")

	// Missing file → zero value, no error.
	s, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("LoadSettings(missing): %v", err)
	}
	if s.StoragePath != "" {
		t.Errorf("expected empty StoragePath on first run, got %q", s.StoragePath)
	}

	want := DesktopSettings{StoragePath: "/Volumes/Photos/Lib"}
	if err := SaveSettings(path, want); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got, err := LoadSettings(path)
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got != want {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestEnsureSecretIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")

	if err := ensureSecret(path); err != nil {
		t.Fatalf("ensureSecret: %v", err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read secret: %v", err)
	}
	if len(first) != 64 { // 32 random bytes hex-encoded
		t.Errorf("secret length = %d, want 64 hex chars", len(first))
	}

	// A second call must not regenerate the secret.
	if err := ensureSecret(path); err != nil {
		t.Fatalf("ensureSecret (2nd): %v", err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read secret (2nd): %v", err)
	}
	if string(first) != string(second) {
		t.Error("ensureSecret regenerated an existing secret; keys would change across launches")
	}
}

func TestStorageReachable(t *testing.T) {
	dir := t.TempDir()
	if !storageReachable(dir) {
		t.Error("existing dir should be reachable")
	}
	// A not-yet-created child of an existing dir is reachable (creatable).
	if !storageReachable(filepath.Join(dir, "library")) {
		t.Error("creatable path (existing parent) should be reachable")
	}
	// A path under a non-existent parent (e.g. unmounted drive) is unreachable.
	if storageReachable(filepath.Join(dir, "missing", "library")) {
		t.Error("path under missing parent should be unreachable")
	}
}

func TestSocketDirFallbackOnLongPath(t *testing.T) {
	// A short app-data root keeps the socket under PGRun.
	short := filepath.Join(string(os.PathSeparator), "tmp", "ld")
	t.Setenv("LUMILIO_APP_DATA", short)
	p, err := NewPaths()
	if err != nil {
		t.Fatalf("NewPaths: %v", err)
	}
	if p.SocketDir() != p.PGRun {
		t.Errorf("short path: SocketDir() = %q, want PGRun %q", p.SocketDir(), p.PGRun)
	}

	// A very long app-data root forces the /tmp fallback to keep the socket path
	// within the platform limit.
	long := filepath.Join(short, strings.Repeat("verylongsegment/", 8))
	t.Setenv("LUMILIO_APP_DATA", long)
	p2, err := NewPaths()
	if err != nil {
		t.Fatalf("NewPaths: %v", err)
	}
	if p2.SocketDir() == p2.PGRun {
		t.Errorf("long path: SocketDir() should fall back to /tmp, got PGRun %q", p2.SocketDir())
	}
	if !strings.HasPrefix(p2.SocketDir(), os.TempDir()) {
		t.Errorf("long path: SocketDir() = %q, want a temp-dir fallback", p2.SocketDir())
	}
}

func TestWindowsPGListenAndHBAConf(t *testing.T) {
	// Windows PostgreSQL has no Unix sockets: loopback TCP + host hba rules.
	winConf := pgListenConf("windows", "127.0.0.1")
	if winConf != "listen_addresses = '127.0.0.1'" {
		t.Errorf("windows listen conf = %q", winConf)
	}
	if hba := pgHBAConf("windows"); !strings.Contains(hba, "host   all   all   127.0.0.1/32   scram-sha-256") {
		t.Errorf("windows hba must require scram on the loopback (reachable by every local user), got %q", hba)
	}

	// unix keeps the socket-only posture (no TCP listener at all).
	unixConf := pgListenConf("darwin", "/run/pg")
	if !strings.Contains(unixConf, "listen_addresses = ''") ||
		!strings.Contains(unixConf, "unix_socket_directories = '/run/pg'") {
		t.Errorf("unix listen conf = %q", unixConf)
	}
	if hba := pgHBAConf("darwin"); !strings.Contains(hba, "local   all   all   scram-sha-256") {
		t.Errorf("unix hba should require scram over the local socket, got %q", hba)
	}
}

func TestPGConfPathValueNormalizesBackslashes(t *testing.T) {
	// PostgreSQL's config parser treats backslashes as escapes; a raw Windows
	// path in postgresql.conf mangles the value and FATALs the postmaster. The
	// generated conf must use forward slashes (accepted on every platform).
	got := pgConfPathValue(`C:\Users\张三\AppData\Local\Lumilio Photos\logs`)
	if strings.Contains(got, `\`) {
		t.Errorf("conf path value still contains backslashes: %q", got)
	}
	if want := "C:/Users/张三/AppData/Local/Lumilio Photos/logs"; got != want {
		t.Errorf("pgConfPathValue = %q, want %q", got, want)
	}
	// Single quotes are still doubled for conf-string safety.
	if q := pgConfPathValue("/tmp/it's"); q != "/tmp/it''s" {
		t.Errorf("pgConfPathValue quote-escape = %q", q)
	}
}

func TestDBHostPerGOOS(t *testing.T) {
	p := &Paths{PGRun: "/short/run"}
	if got := dbHostForGOOS("windows", p); got != "127.0.0.1" {
		t.Errorf("windows DBHost = %q, want loopback", got)
	}
	if got := dbHostForGOOS("darwin", p); got != p.SocketDir() {
		t.Errorf("darwin DBHost = %q, want socket dir %q", got, p.SocketDir())
	}
}
