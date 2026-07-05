package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppConfigWithOptionsResolvesPrecedenceWithoutAmbientEnv(t *testing.T) {
	dir := t.TempDir()
	passwordFile := filepath.Join(dir, "db_password")
	if err := os.WriteFile(passwordFile, []byte("rotated-secret\n"), 0o600); err != nil {
		t.Fatalf("write password file: %v", err)
	}

	configFile := filepath.Join(dir, "server.local.toml")
	if err := os.WriteFile(configFile, []byte(`
environment = "production"

[server]
port = "9000"
log_level = "warn"
cors_allowed_origins = ["http://toml.example"]
web_root = "/toml/web"

[logging]
level = "warn"
dir = "/toml/logs"
console_format = "json"
file_format = "json"
repository_audit_verbose = "toml"

[database]
host = "toml-db"
port = "15432"
user = "toml-user"
password = "toml-bootstrap"
password_file = "`+passwordFile+`"
name = "tomldb"
ssl = "require"

[storage]
path = "/toml/storage"

[repository_scan]
enabled = false
interval_seconds = 600
settle_seconds = 9
max_concurrent_repos = 3
batch_size = 99

[geocoding]
provider = "nominatim"
nominatim_endpoint = "https://geo.example/reverse"
language = "en"
user_agent = "TomlAgent/1.0"

[auth]
secret_key_path = "/toml/secret"
access_token_ttl = "10m"
refresh_token_ttl = "24h"
media_token_ttl = "5m"
webauthn_rp_name = "Toml RP"
webauthn_rp_id = "toml.example"
webauthn_rp_origins = ["https://toml.example"]

[transcode]
hardware_accel = "none"

[lumen]
discovery_enabled = false
discovery_mdns_enabled = false
discovery_hub_url = "https://lumen.example"

[tools]
exiftool_path = "/toml/exiftool"
ffmpeg_path = "/toml/ffmpeg"
ffprobe_path = "/toml/ffprobe"
`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	t.Setenv("SERVER_PORT", "9999")
	t.Setenv("DB_PASSWORD", "ambient-password")
	t.Setenv("STORAGE_PATH", "/ambient/storage")

	cfg, err := LoadAppConfigWithOptions(LoadOptions{
		Environment: "development",
		ConfigFile:  configFile,
		Env: map[string]string{
			"SERVER_ENV":                   "development",
			"SERVER_PORT":                  "6680",
			"SERVER_LOG_LEVEL":             "debug",
			"LOG_DIR":                      "/env/logs",
			"DB_HOST":                      "env-db",
			"DB_PASSWORD":                  "env-bootstrap",
			"STORAGE_PATH":                 "/env/storage",
			"LUMILIO_SECRET_KEY":           "/env/secret",
			"LUMEN_DISCOVERY_MDNS_ENABLED": "true",
			"EXIFTOOL_PATH":                "/env/exiftool",
		},
	})
	if err != nil {
		t.Fatalf("LoadAppConfigWithOptions: %v", err)
	}

	if cfg.Environment != "development" {
		t.Fatalf("Environment = %q, want development from explicit env map", cfg.Environment)
	}
	if cfg.ServerConfig.Port != "6680" {
		t.Fatalf("ServerConfig.Port = %q, want env-map override 6680", cfg.ServerConfig.Port)
	}
	if cfg.ServerConfig.LogLevel != "debug" || cfg.LoggingConfig.Level != "debug" {
		t.Fatalf("SERVER_LOG_LEVEL should update server and logging levels, got server=%q logging=%q", cfg.ServerConfig.LogLevel, cfg.LoggingConfig.Level)
	}
	if cfg.LoggingConfig.LogDir != "/env/logs" {
		t.Fatalf("LoggingConfig.LogDir = %q, want env-map override /env/logs", cfg.LoggingConfig.LogDir)
	}
	if cfg.DatabaseConfig.Host != "env-db" {
		t.Fatalf("DatabaseConfig.Host = %q, want env-map override env-db", cfg.DatabaseConfig.Host)
	}
	if cfg.DatabaseConfig.Password != "rotated-secret" {
		t.Fatalf("DatabaseConfig.Password = %q, want password_file to beat env bootstrap password", cfg.DatabaseConfig.Password)
	}
	if cfg.StorageConfig.Path != "/env/storage" {
		t.Fatalf("StorageConfig.Path = %q, want env-map override /env/storage", cfg.StorageConfig.Path)
	}
	if cfg.Auth.SecretKeyPath != "/env/secret" {
		t.Fatalf("Auth.SecretKeyPath = %q, want env-map override /env/secret", cfg.Auth.SecretKeyPath)
	}
	if !cfg.Lumen.DiscoveryMDNSEnabled {
		t.Fatalf("Lumen.DiscoveryMDNSEnabled = false, want env-map override true")
	}
	if cfg.Tools.ExifToolPath != "/env/exiftool" || cfg.Tools.FFmpegPath != "/toml/ffmpeg" {
		t.Fatalf("unexpected tool paths after env-map override: %+v", cfg.Tools)
	}
}

func TestLoadAppConfigWithOptionsValidatesConfigValues(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "server.local.toml")
	if err := os.WriteFile(configFile, []byte(`
[server]
port = "6680"
log_level = "verbose"

[geocoding]
provider = "somewhere"

[transcode]
hardware_accel = "magic"
`), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	_, err := LoadAppConfigWithOptions(LoadOptions{
		Environment: "development",
		ConfigFile:  configFile,
	})
	if err == nil {
		t.Fatal("expected invalid config values to fail validation")
	}

	message := err.Error()
	for _, field := range []string{
		"server.log_level",
		"geocoding.provider",
		"transcode.hardware_accel",
	} {
		if !strings.Contains(message, field) {
			t.Fatalf("validation error %q should mention %s", message, field)
		}
	}
}

func TestLoadAppConfigWithOptionsReportsExplicitMissingConfigFile(t *testing.T) {
	_, err := LoadAppConfigWithOptions(LoadOptions{
		Environment:       "development",
		ConfigFile:        filepath.Join(t.TempDir(), "missing.toml"),
		RequireConfigFile: true,
	})
	if err == nil {
		t.Fatal("expected missing required config file to fail")
	}
	if !strings.Contains(err.Error(), "missing.toml") {
		t.Fatalf("missing config error should include path, got %v", err)
	}
}

func TestLumenConfigEnabled(t *testing.T) {
	cases := []struct {
		name string
		cfg  LumenConfig
		want bool
	}{
		{"discovery off", LumenConfig{DiscoveryEnabled: false, DiscoveryMDNSEnabled: true}, false},
		{"no backend", LumenConfig{DiscoveryEnabled: true}, false},
		{"blank hub url is no backend", LumenConfig{DiscoveryEnabled: true, DiscoveryHubURL: "  "}, false},
		{"mdns backend", LumenConfig{DiscoveryEnabled: true, DiscoveryMDNSEnabled: true}, true},
		{"hub backend", LumenConfig{DiscoveryEnabled: true, DiscoveryHubURL: "http://gw:5866"}, true},
		{"static backend", LumenConfig{DiscoveryEnabled: true, DiscoveryStaticNodes: []string{"10.0.0.5:50051"}}, true},
		{"blank static entries are no backend", LumenConfig{DiscoveryEnabled: true, DiscoveryStaticNodes: []string{" ", ""}}, false},
	}
	for _, tc := range cases {
		if got := tc.cfg.Enabled(); got != tc.want {
			t.Errorf("%s: Enabled() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
