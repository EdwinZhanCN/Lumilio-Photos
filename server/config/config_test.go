package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const completeManifest = `schema_version = 1
environment = "development"
[database]
host = "localhost"
port = "5433"
user = "postgres"
name = "lumiliophotos"
ssl = "disable"
bootstrap_password_file = ".secrets/bootstrap"
rotated_password_file = "data/rotated"
tools_bin_dir = ""
[server]
port = "6680"
cors_allowed_origins = []
web_root = ""
[logging]
level = "debug"
dir = "logs"
console_format = "console"
file_format = "json"
repository_audit_verbose = false
[storage]
path = "data/storage"
cloud_state_path = "data/app-state/cloud"
backups_path = "data/app-state/backups"
[repository_scan]
enabled = true
interval_seconds = 300
settle_seconds = 5
max_concurrent_repos = 1
batch_size = 500
[geocoding]
provider = "disabled"
nominatim_endpoint = "https://nominatim.openstreetmap.org/reverse"
language = "en"
user_agent = "Lumilio-Photos/1.0"
[auth]
secret_key_file = "data/app-state/secrets/key"
access_token_ttl = "15m"
refresh_token_ttl = "168h"
media_token_ttl = "10m"
webauthn_rp_name = "Lumilio Photos"
webauthn_rp_mode = "origin-derived"
webauthn_rp_id = ""
webauthn_rp_origins = []
[transcode]
hardware_accel = "auto"
[lumen]
discovery_enabled = true
discovery_mdns_enabled = true
discovery_hub_url = ""
discovery_static_nodes = []
discovery_service_type = "_lumen._tcp"
discovery_domain = "local"
deployment_id = "local"
resolve_timeout = "3s"
connect_timeout = "3s"
rediscovery_backoff_min = "10s"
rediscovery_backoff_max = "2m"
scan_interval = "30s"
chunk_auto = true
chunk_threshold_bytes = 1048576
chunk_max_bytes = 262144
[tools]
exiftool_path = "exiftool"
ffmpeg_path = "bin/ffmpeg"
ffprobe_path = "/opt/ffprobe"
`

func writeManifestFixture(t *testing.T, contents string) string {
	t.Helper()
	contents = strings.ReplaceAll(contents, `"/opt/ffprobe"`, strconv.Quote(filepath.ToSlash(absoluteToolFixturePath())))
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".secrets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".secrets", "bootstrap"), []byte("bootstrap-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "server.toml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func absoluteToolFixturePath() string {
	if runtime.GOOS != "windows" {
		return "/opt/ffprobe"
	}
	return filepath.Join(filepath.VolumeName(os.TempDir())+string(filepath.Separator), "opt", "ffprobe")
}

func TestLoadAppConfigStrictCompleteManifest(t *testing.T) {
	path := writeManifestFixture(t, completeManifest)
	t.Setenv("SERVER_PORT", "9999")
	t.Setenv("DB_PASSWORD", "ambient-secret")
	t.Setenv("LUMEN_DISCOVERY_ENABLED", "false")

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	base := filepath.Dir(path)
	if !cfg.LoadedFromManifest() || cfg.SchemaVersion != 1 || cfg.ManifestPath != path || len(cfg.ManifestSHA256) != 64 {
		t.Fatalf("missing manifest provenance: %+v", cfg)
	}
	if cfg.ServerConfig.Port != "6680" || cfg.DatabaseConfig.Password != "bootstrap-secret" || !cfg.Lumen.DiscoveryEnabled {
		t.Fatalf("ambient environment changed config: %+v", cfg)
	}
	if cfg.StorageConfig.Path != filepath.Join(base, "data/storage") {
		t.Fatalf("storage path = %q", cfg.StorageConfig.Path)
	}
	if cfg.StorageConfig.CloudDir() != filepath.Join(base, "data/app-state/cloud") || cfg.StorageConfig.BackupsDir() != filepath.Join(base, "data/app-state/backups") {
		t.Fatalf("private storage paths = %+v", cfg.StorageConfig)
	}
	if cfg.Tools.FFmpegPath != filepath.Join(base, "bin/ffmpeg") || cfg.Tools.ExifToolPath != "exiftool" || cfg.Tools.FFprobePath != absoluteToolFixturePath() {
		t.Fatalf("tool path resolution = %+v", cfg.Tools)
	}
	if cfg.Auth.AccessTokenTTL != 15*time.Minute {
		t.Fatalf("access ttl = %v", cfg.Auth.AccessTokenTTL)
	}
}

func TestLoadAppConfigUsesRotatedSecretWhenPresent(t *testing.T) {
	path := writeManifestFixture(t, completeManifest)
	rotated := filepath.Join(filepath.Dir(path), "data", "rotated")
	if err := os.MkdirAll(filepath.Dir(rotated), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rotated, []byte("rotated-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabaseConfig.Password != "rotated-secret" || cfg.DatabaseConfig.BootstrapPassword != "bootstrap-secret" {
		t.Fatalf("unexpected secrets: %+v", cfg.DatabaseConfig)
	}
}

func TestLoadAppConfigRejectsUnknownAndLegacyFields(t *testing.T) {
	for name, contents := range map[string]string{
		"unknown":           completeManifest + "\nunknown_field = true\n",
		"legacy password":   strings.Replace(completeManifest, "host = \"localhost\"", "host = \"localhost\"\npassword = \"plaintext\"", 1),
		"legacy server log": strings.Replace(completeManifest, "port = \"6680\"", "port = \"6680\"\nlog_level = \"debug\"", 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadAppConfig(writeManifestFixture(t, contents))
			if err == nil || !strings.Contains(err.Error(), "strict mode") {
				t.Fatalf("expected strict unknown-field error, got %v", err)
			}
		})
	}
}

func TestLoadAppConfigAggregatesMissingFields(t *testing.T) {
	path := writeManifestFixture(t, "schema_version = 1\n")
	_, err := LoadAppConfig(path)
	if err == nil {
		t.Fatal("expected incomplete manifest to fail")
	}
	for _, want := range []string{"environment is required", "[database] is required", "[tools] is required"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestEveryManifestFieldIsRequired(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(completeManifest), "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name := strings.TrimSpace(strings.SplitN(trimmed, "=", 2)[0])
		t.Run(fmt.Sprintf("%s_%d", name, index), func(t *testing.T) {
			without := append([]string(nil), lines[:index]...)
			without = append(without, lines[index+1:]...)
			if _, err := LoadAppConfig(writeManifestFixture(t, strings.Join(without, "\n"))); err == nil {
				t.Fatalf("manifest unexpectedly loaded without line %q", line)
			}
		})
	}
}

func TestLoadAppConfigAggregatesInvalidValues(t *testing.T) {
	contents := strings.ReplaceAll(completeManifest, "interval_seconds = 300", "interval_seconds = 0")
	contents = strings.ReplaceAll(contents, "connect_timeout = \"3s\"", "connect_timeout = \"never\"")
	contents = strings.ReplaceAll(contents, "chunk_max_bytes = 262144", "chunk_max_bytes = 2097152")
	_, err := LoadAppConfig(writeManifestFixture(t, contents))
	if err == nil {
		t.Fatal("expected invalid manifest")
	}
	for _, want := range []string{"repository_scan.interval_seconds", "lumen.connect_timeout", "lumen.chunk_max_bytes"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not contain %q", err, want)
		}
	}
}

func TestLoadAppConfigRequiresReadableNonEmptyBootstrapSecret(t *testing.T) {
	path := writeManifestFixture(t, completeManifest)
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), ".secrets", "bootstrap"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadAppConfig(path)
	if err == nil || !strings.Contains(err.Error(), "secret file is empty") {
		t.Fatalf("expected empty secret error, got %v", err)
	}
}

func TestLoadAppConfigRequiresExplicitPath(t *testing.T) {
	if _, err := LoadAppConfig(""); err == nil {
		t.Fatal("expected empty path to fail")
	}
	missing := filepath.Join(t.TempDir(), "missing.toml")
	if _, err := LoadAppConfig(missing); err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("expected path in error, got %v", err)
	}
}

func TestLoadAppConfigRejectsPrivateStateInsideMediaRoot(t *testing.T) {
	cases := map[string]struct {
		old  string
		new  string
		want string
	}{
		"cloud state": {`cloud_state_path = "data/app-state/cloud"`, `cloud_state_path = "data/storage/.cloud"`, "storage.cloud_state_path"},
		"backups":     {`backups_path = "data/app-state/backups"`, `backups_path = "data/storage/backups"`, "storage.backups_path"},
		"logs":        {`dir = "logs"`, `dir = "data/storage/logs"`, "logging.dir"},
		"app secret":  {`secret_key_file = "data/app-state/secrets/key"`, `secret_key_file = "data/storage/.secrets/key"`, "auth.secret_key_file"},
		"db rotated":  {`rotated_password_file = "data/rotated"`, `rotated_password_file = "data/storage/.secrets/rotated"`, "database.rotated_password_file"},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			contents := strings.Replace(completeManifest, test.old, test.new, 1)
			_, err := LoadAppConfig(writeManifestFixture(t, contents))
			if err == nil || !strings.Contains(err.Error(), test.want+" must be outside storage.path") {
				t.Fatalf("expected %s separation error, got %v", test.want, err)
			}
		})
	}
}
