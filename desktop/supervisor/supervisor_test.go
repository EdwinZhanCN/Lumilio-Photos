package supervisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	serverconfig "server/config"

	"github.com/pelletier/go-toml/v2"
)

// The desktop supervisor delegates runtime config construction to server/config.
// The typed config is load-bearing; the generated TOML is only a debug copy.
func TestDesktopServerConfigInvariants(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.local.toml")

	cfg, err := serverconfig.NewDesktopConfig(serverconfig.DesktopParams{
		Port:          "6680",
		WebRoot:       "/bundle/web",
		LogDir:        "/Users/me/Library/Application Support/Lumilio Photos/logs",
		StoragePath:   "/Volumes/Photos/Lumilio Library",
		SocketDir:     "/Users/me/Library/Application Support/Lumilio Photos/postgres/17/run",
		PGPort:        "5487",
		DBUser:        "lumilio",
		DBName:        "lumiliophotos",
		PasswordFile:  "/secrets/db_password",
		SecretKeyPath: "/secrets/lumilio_secret_key",
		ExifToolPath:  "/bundle/exiftool",
		FFmpegPath:    "/bundle/ffmpeg",
		FFprobePath:   "/bundle/ffprobe",
	})
	if err != nil {
		t.Fatalf("NewDesktopConfig: %v", err)
	}
	if cfg.Auth.WebAuthnRPID != "localhost" {
		t.Fatalf("webauthn rp id = %q, want localhost", cfg.Auth.WebAuthnRPID)
	}
	if got, want := strings.Join(cfg.Auth.WebAuthnRPOrigins, ","), "http://localhost:6680"; got != want {
		t.Fatalf("webauthn origins = %q, want %q", got, want)
	}
	if cfg.DatabaseConfig.Host != "/Users/me/Library/Application Support/Lumilio Photos/postgres/17/run" {
		t.Fatalf("database host = %q", cfg.DatabaseConfig.Host)
	}

	if err := serverconfig.WriteGeneratedTOML(path, cfg); err != nil {
		t.Fatalf("WriteGeneratedTOML: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var decoded struct {
		Server struct {
			Port    string `toml:"port"`
			WebRoot string `toml:"web_root"`
		} `toml:"server"`
		Logging struct {
			Dir string `toml:"dir"`
		} `toml:"logging"`
		Database struct {
			Host string `toml:"host"`
			Name string `toml:"name"`
		} `toml:"database"`
		Storage struct {
			Path string `toml:"path"`
		} `toml:"storage"`
		Auth struct {
			WebAuthnRPID      string   `toml:"webauthn_rp_id"`
			WebAuthnRPOrigins []string `toml:"webauthn_rp_origins"`
		} `toml:"auth"`
		Tools struct {
			ExifToolPath string `toml:"exiftool_path"`
			FFmpegPath   string `toml:"ffmpeg_path"`
			FFprobePath  string `toml:"ffprobe_path"`
		} `toml:"tools"`
	}
	if err := toml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("generated debug config should parse: %v\n%s", err, string(data))
	}
	if decoded.Server.Port != "6680" || decoded.Server.WebRoot != "/bundle/web" {
		t.Fatalf("unexpected generated server config: %+v", decoded.Server)
	}
	if decoded.Logging.Dir != "/Users/me/Library/Application Support/Lumilio Photos/logs" {
		t.Fatalf("unexpected generated log dir: %q", decoded.Logging.Dir)
	}
	if decoded.Database.Host != "/Users/me/Library/Application Support/Lumilio Photos/postgres/17/run" || decoded.Database.Name != "lumiliophotos" {
		t.Fatalf("unexpected generated database config: %+v", decoded.Database)
	}
	if decoded.Storage.Path != "/Volumes/Photos/Lumilio Library" {
		t.Fatalf("unexpected generated storage path: %q", decoded.Storage.Path)
	}
	if decoded.Auth.WebAuthnRPID != "localhost" || strings.Join(decoded.Auth.WebAuthnRPOrigins, ",") != "http://localhost:6680" {
		t.Fatalf("unexpected generated auth config: %+v", decoded.Auth)
	}
	if decoded.Tools.ExifToolPath != "/bundle/exiftool" || decoded.Tools.FFmpegPath != "/bundle/ffmpeg" || decoded.Tools.FFprobePath != "/bundle/ffprobe" {
		t.Fatalf("unexpected generated tool paths: %+v", decoded.Tools)
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
