package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestNewDesktopConfigInvariants(t *testing.T) {
	dir := t.TempDir()
	passwordFile := filepath.Join(dir, "db_password")
	if err := os.WriteFile(passwordFile, []byte("rotated-secret\n"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	cfg, err := NewDesktopConfig(DesktopParams{
		Port:          "6680",
		WebRoot:       "/bundle/web",
		LogDir:        filepath.Join(dir, "logs"),
		StoragePath:   "/Volumes/Photos/Lumilio Library",
		DBHost:        filepath.Join(dir, "postgres", "run"),
		PGPort:        "5487",
		DBUser:        "lumilio",
		DBName:        "lumiliophotos",
		PasswordFile:  passwordFile,
		SecretKeyPath: filepath.Join(dir, "lumilio_secret_key"),
		ExifToolPath:  "/bundle/exiftool",
		FFmpegPath:    "/bundle/ffmpeg",
		FFprobePath:   "/bundle/ffprobe",
		LumenStaticNodes: []string{
			"127.0.0.1:50051",
		},
	})
	if err != nil {
		t.Fatalf("NewDesktopConfig: %v", err)
	}

	if cfg.Environment != "production" {
		t.Fatalf("environment = %q, want production", cfg.Environment)
	}
	if cfg.ServerConfig.Port != "6680" || cfg.ServerConfig.WebRoot != "/bundle/web" {
		t.Fatalf("unexpected server config: %+v", cfg.ServerConfig)
	}
	if cfg.DatabaseConfig.Host != filepath.Join(dir, "postgres", "run") || cfg.DatabaseConfig.Port != "5487" {
		t.Fatalf("unexpected database socket config: %+v", cfg.DatabaseConfig)
	}
	if cfg.DatabaseConfig.Password != "rotated-secret" {
		t.Fatalf("expected password loaded from password_file, got %q", cfg.DatabaseConfig.Password)
	}
	if cfg.Auth.WebAuthnRPID != "localhost" {
		t.Fatalf("webauthn rp id = %q, want localhost", cfg.Auth.WebAuthnRPID)
	}
	if got, want := strings.Join(cfg.Auth.WebAuthnRPOrigins, ","), "http://localhost:6680"; got != want {
		t.Fatalf("webauthn origins = %q, want %q", got, want)
	}
	if !cfg.Lumen.DiscoveryEnabled || !cfg.Lumen.DiscoveryMDNSEnabled {
		t.Fatalf("desktop Lumen discovery should be enabled, got %+v", cfg.Lumen)
	}
	if got := strings.Join(cfg.Lumen.StaticNodes(), ","); got != "127.0.0.1:50051" {
		t.Fatalf("lumen static nodes = %q, want pinned local hub endpoint", got)
	}
	if cfg.Tools.ExifToolPath != "/bundle/exiftool" || cfg.Tools.FFmpegPath != "/bundle/ffmpeg" || cfg.Tools.FFprobePath != "/bundle/ffprobe" {
		t.Fatalf("unexpected tool paths: %+v", cfg.Tools)
	}
}

func TestNewDesktopConfigRejectsMissingRequiredParams(t *testing.T) {
	_, err := NewDesktopConfig(DesktopParams{})
	if err == nil {
		t.Fatal("expected missing desktop params to fail")
	}
	if !strings.Contains(err.Error(), "desktop config requires") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewDesktopConfigPrefersTypedPasswordFileOverEnv(t *testing.T) {
	dir := t.TempDir()
	typedPasswordFile := filepath.Join(dir, "typed_db_password")
	envPasswordFile := filepath.Join(dir, "env_db_password")
	if err := os.WriteFile(typedPasswordFile, []byte("typed-secret\n"), 0o600); err != nil {
		t.Fatalf("write typed password file: %v", err)
	}
	if err := os.WriteFile(envPasswordFile, []byte("env-secret\n"), 0o600); err != nil {
		t.Fatalf("write env password file: %v", err)
	}
	t.Setenv("LUMILIO_DB_PASSWORD_FILE", envPasswordFile)

	cfg, err := NewDesktopConfig(DesktopParams{
		Port:          "6680",
		LogDir:        filepath.Join(dir, "logs"),
		StoragePath:   filepath.Join(dir, "storage"),
		DBHost:        filepath.Join(dir, "postgres", "run"),
		PGPort:        "5487",
		DBUser:        "lumilio",
		DBName:        "lumiliophotos",
		PasswordFile:  typedPasswordFile,
		SecretKeyPath: filepath.Join(dir, "lumilio_secret_key"),
	})
	if err != nil {
		t.Fatalf("NewDesktopConfig: %v", err)
	}
	if cfg.DatabaseConfig.Password != "typed-secret" {
		t.Fatalf("desktop config should prefer typed password file, got %q", cfg.DatabaseConfig.Password)
	}
}

func TestWriteGeneratedTOMLDebugCopy(t *testing.T) {
	dir := t.TempDir()
	cfg, err := NewDesktopConfig(DesktopParams{
		Port:          "6680",
		WebRoot:       "/bundle/web",
		LogDir:        filepath.Join(dir, "logs"),
		StoragePath:   "/Volumes/Photos/Lumilio Library",
		DBHost:        filepath.Join(dir, "postgres", "run"),
		PGPort:        "5487",
		DBUser:        "lumilio",
		DBName:        "lumiliophotos",
		PasswordFile:  filepath.Join(dir, "db_password"),
		SecretKeyPath: filepath.Join(dir, "lumilio_secret_key"),
		ExifToolPath:  "/bundle/exiftool",
		FFmpegPath:    "/bundle/ffmpeg",
		FFprobePath:   "/bundle/ffprobe",
	})
	if err != nil {
		t.Fatalf("NewDesktopConfig: %v", err)
	}
	cfg.DatabaseConfig.Password = "do-not-write"

	path := filepath.Join(dir, "server.local.toml")
	if err := WriteGeneratedTOML(path, cfg); err != nil {
		t.Fatalf("WriteGeneratedTOML: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated toml: %v", err)
	}
	if strings.Contains(string(data), "do-not-write") {
		t.Fatalf("generated debug TOML must not contain raw database password:\n%s", string(data))
	}

	var decoded tomlConfig
	if err := toml.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("generated TOML should parse: %v\n%s", err, string(data))
	}
	if decoded.ServerConfig.Port != "6680" || decoded.Auth.WebAuthnRPID != "localhost" {
		t.Fatalf("unexpected generated TOML invariants: %+v %+v", decoded.ServerConfig, decoded.Auth)
	}
	if decoded.Tools.FFmpegPath != "/bundle/ffmpeg" {
		t.Fatalf("expected tool paths in generated TOML, got %+v", decoded.Tools)
	}
}
