package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const completeManifest = `schema_version = 6
environment = "development"
[database]
path = "data/app-state/library.sqlite3"
queue_path = "data/app-state/river.sqlite3"
[server]
listen = "127.0.0.1:6680"
cors_allowed_origins = ["http://LOCALHOST:6657", "http://localhost:6657"]
web_root = ""
[server.tls]
mode = "off"
hostname = ""
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
trusted_cidrs = []
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
interval_seconds = 300
settle_seconds = 5
[auth]
secret_key_file = "data/app-state/secrets/key"
access_token_ttl = "15m"
refresh_token_ttl = "168h"
media_token_ttl = "10m"
[auth.passkey]
enabled = true
name = "Lumilio Photos"
[auth.rate_limit]
ip_attempts = 60
subject_attempts = 8
window = "1m"
lockout = "5m"
max_entries = 10000
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
failure_cooldown_min = "10s"
failure_cooldown_max = "2m"
scan_interval = "30s"
chunk_auto = true
chunk_threshold_bytes = 1048576
chunk_max_bytes = 262144
[tools]
exiftool_path = "exiftool"
ffmpeg_path = "bin/ffmpeg"
ffprobe_path = "/opt/ffprobe"
[execution]
cpu = 3
disk_io = 4
image_codec = 2
video_codec = 1
inference = 2
memory_mib = 768
macro_workers = 16
max_waiting = 256
ffmpeg_threads = 3
ffmpeg_software_preset = "veryfast"
`

func writeManifestFixture(t *testing.T, contents string) string {
	t.Helper()
	contents = strings.ReplaceAll(contents, `"/opt/ffprobe"`, strconv.Quote(filepath.ToSlash(absoluteToolFixturePath())))
	path := filepath.Join(t.TempDir(), "server.toml")
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

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	base := filepath.Dir(path)
	if !cfg.LoadedFromManifest() || cfg.SchemaVersion != SchemaVersion || cfg.ManifestPath != path || len(cfg.ManifestSHA256) != 64 {
		t.Fatalf("missing manifest provenance: %+v", cfg)
	}
	if cfg.ServerConfig.Listen != "127.0.0.1:6680" ||
		len(cfg.ServerConfig.CORSAllowedOrigins) != 1 ||
		cfg.DatabaseConfig.Path != filepath.Join(base, "data/app-state/library.sqlite3") {
		t.Fatalf("resolved config = %+v", cfg)
	}
	if cfg.Auth.AccessTokenTTL != 15*time.Minute || !cfg.Auth.Passkey.Enabled {
		t.Fatalf("auth config = %+v", cfg.Auth)
	}
	if cfg.Execution.CPU != 3 || cfg.Execution.DiskIO != 4 || cfg.Execution.MemoryMiB != 768 {
		t.Fatalf("execution config = %+v", cfg.Execution)
	}
}

func TestLoadAppConfigBytesPreservesManifestBase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate.toml")
	data := []byte(strings.ReplaceAll(completeManifest, `"/opt/ffprobe"`, strconv.Quote(filepath.ToSlash(absoluteToolFixturePath()))))
	cfg, err := LoadAppConfigBytes(path, data)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if cfg.ManifestSHA256 != fmt.Sprintf("%x", sum) ||
		cfg.DatabaseConfig.Path != filepath.Join(filepath.Dir(path), "data/app-state/library.sqlite3") {
		t.Fatalf("manifest provenance or base is wrong: %+v", cfg)
	}
}

func TestResolvePathKeepsPOSIXAbsolutePathsAbsolute(t *testing.T) {
	base := filepath.Join(t.TempDir(), "manifest")
	if got, want := resolvePath(base, "/app/web"), filepath.Clean("/app/web"); got != want {
		t.Fatalf("resolvePath(%q) = %q, want %q", "/app/web", got, want)
	}
}

func TestLoadAppConfigRejectsUnknownRemovedAndMissingFields(t *testing.T) {
	for name, contents := range map[string]string{
		"unknown":                     completeManifest + "\nunknown_field = true\n",
		"primary origin":              strings.Replace(completeManifest, `listen = "127.0.0.1:6680"`, "listen = \"127.0.0.1:6680\"\nprimary_origin = \"https://photos.example.com\"", 1),
		"proxy mode":                  strings.Replace(completeManifest, "trusted_cidrs = []", "mode = \"required\"\ntrusted_cidrs = []", 1),
		"repository scan enabled":     strings.Replace(completeManifest, "interval_seconds = 300", "enabled = false\ninterval_seconds = 300", 1),
		"repository scan concurrency": strings.Replace(completeManifest, "settle_seconds = 5", "settle_seconds = 5\nmax_concurrent_repos = 1", 1),
		"repository scan batch size":  strings.Replace(completeManifest, "settle_seconds = 5", "settle_seconds = 5\nbatch_size = 500", 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadAppConfig(writeManifestFixture(t, contents))
			if err == nil || !strings.Contains(err.Error(), "strict mode") {
				t.Fatalf("expected strict unknown-field error, got %v", err)
			}
		})
	}

	_, err := LoadAppConfig(writeManifestFixture(t, "schema_version = 3\n"))
	if err == nil || !strings.Contains(err.Error(), "[server] is required") {
		t.Fatalf("expected aggregate missing-field error, got %v", err)
	}
}

func TestEveryManifestFieldIsRequired(t *testing.T) {
	lines := strings.Split(strings.TrimSpace(completeManifest), "\n")
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "[") {
			continue
		}
		t.Run(fmt.Sprintf("%d_%s", index, strings.SplitN(trimmed, "=", 2)[0]), func(t *testing.T) {
			without := append([]string(nil), lines[:index]...)
			without = append(without, lines[index+1:]...)
			if _, err := LoadAppConfig(writeManifestFixture(t, strings.Join(without, "\n"))); err == nil {
				t.Fatalf("manifest loaded without %q", line)
			}
		})
	}
}

func TestLoadAppConfigNetworkTopology(t *testing.T) {
	acme := strings.Replace(completeManifest, `listen = "127.0.0.1:6680"`, `listen = ":443"`, 1)
	acme = strings.Replace(acme, `mode = "off"
hostname = ""
http_listen = ""
email = ""
storage_path = ""`, `mode = "acme"
hostname = "photos.example.com"
http_listen = ":80"
email = "admin@example.com"
storage_path = "data/app-state/tls"`, 1)
	cfg, err := LoadAppConfig(writeManifestFixture(t, acme))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerConfig.TLS.Hostname != "photos.example.com" {
		t.Fatalf("ACME hostname = %q", cfg.ServerConfig.TLS.Hostname)
	}

	for name, contents := range map[string]string{
		"IP hostname": strings.Replace(acme, `hostname = "photos.example.com"`, `hostname = "203.0.113.10"`, 1),
		"localhost":   strings.Replace(acme, `hostname = "photos.example.com"`, `hostname = "localhost"`, 1),
		"collision":   strings.Replace(acme, `http_listen = ":80"`, `http_listen = "127.0.0.1:443"`, 1),
		"off fields":  strings.Replace(completeManifest, `hostname = ""`, `hostname = "photos.example.com"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadAppConfig(writeManifestFixture(t, contents)); err == nil {
				t.Fatal("expected invalid network topology")
			}
		})
	}
}

func TestNormalizeOriginCanonicalizesIDNAAndDefaultPorts(t *testing.T) {
	normalized, parsed, err := NormalizeOrigin("HTTPS://BÜCHER.Example:443")
	if err != nil {
		t.Fatal(err)
	}
	if normalized != "https://xn--bcher-kva.example" || parsed.Hostname() != "xn--bcher-kva.example" {
		t.Fatalf("normalized origin = %q", normalized)
	}
	for _, invalid := range []string{"https://photos.example.com/", "https://user@photos.example.com", "https://photos.example.com/path"} {
		if _, _, err := NormalizeOrigin(invalid); err == nil {
			t.Fatalf("accepted invalid origin %q", invalid)
		}
	}
}

func TestGenerateDockerProductionProfiles(t *testing.T) {
	tests := []struct {
		profile ProfileName
		inputs  ProfileInputs
	}{
		{ProfileDockerHTTP, ProfileInputs{}},
		{ProfileDockerACME, ProfileInputs{Hostname: "photos.example.com", Email: "admin@example.com"}},
	}
	for _, test := range tests {
		t.Run(string(test.profile), func(t *testing.T) {
			data, err := GenerateManifest(test.profile, test.inputs)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadAppConfigBytes(filepath.Join(t.TempDir(), "server.toml"), data)
			if err != nil {
				t.Fatalf("strict load generated profile: %v", err)
			}
			if test.profile == ProfileDockerHTTP && cfg.ServerConfig.Listen != ":6680" {
				t.Fatalf("HTTP listener = %q", cfg.ServerConfig.Listen)
			}
			if test.profile == ProfileDockerACME && cfg.ServerConfig.TLS.Hostname != "photos.example.com" {
				t.Fatalf("ACME config = %+v", cfg.ServerConfig.TLS)
			}
		})
	}
}
