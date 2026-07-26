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

const completeManifest = `schema_version = 3
environment = "development"
[database]
path = "data/app-state/library.sqlite3"
[server]
listen = "127.0.0.1:6680"
primary_origin = "http://LOCALHOST:6657"
cors_allowed_origins = ["http://LOCALHOST:6657", "http://localhost:6657"]
web_root = ""
[server.tls]
mode = "off"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "disabled"
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
	if !cfg.LoadedFromManifest() || cfg.SchemaVersion != 3 || cfg.ManifestPath != path || len(cfg.ManifestSHA256) != 64 {
		t.Fatalf("missing manifest provenance: %+v", cfg)
	}
	if cfg.ServerConfig.Listen != "127.0.0.1:6680" ||
		cfg.ServerConfig.PrimaryOrigin != "http://localhost:6657" ||
		len(cfg.ServerConfig.CORSAllowedOrigins) != 1 ||
		cfg.DatabaseConfig.Path != filepath.Join(base, "data/app-state/library.sqlite3") ||
		!cfg.Lumen.DiscoveryEnabled {
		t.Fatalf("ambient environment changed config: %+v", cfg)
	}
	if cfg.Auth.PasskeyIdentity.Origin != cfg.ServerConfig.PrimaryOrigin ||
		cfg.Auth.PasskeyIdentity.RPID != "localhost" {
		t.Fatalf("passkey identity is not derived from primary origin: %+v", cfg.Auth.PasskeyIdentity)
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
	if cfg.Auth.RateLimit.IPAttempts != 60 ||
		cfg.Auth.RateLimit.SubjectAttempts != 8 ||
		cfg.Auth.RateLimit.Window != time.Minute ||
		cfg.Auth.RateLimit.Lockout != 5*time.Minute ||
		cfg.Auth.RateLimit.MaxEntries != 10_000 {
		t.Fatalf("auth rate limit = %+v", cfg.Auth.RateLimit)
	}
}

func TestLoadAppConfigBytesPreservesStrictLoaderAndManifestBase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.candidate.toml")
	contents := strings.ReplaceAll(
		completeManifest,
		`"/opt/ffprobe"`,
		strconv.Quote(filepath.ToSlash(absoluteToolFixturePath())),
	)
	data := []byte(contents)

	cfg, err := LoadAppConfigBytes(path, data)
	if err != nil {
		t.Fatalf("LoadAppConfigBytes: %v", err)
	}
	sum := sha256.Sum256(data)
	if cfg.ManifestPath != path || cfg.ManifestSHA256 != fmt.Sprintf("%x", sum) {
		t.Fatalf("manifest provenance = %q %q", cfg.ManifestPath, cfg.ManifestSHA256)
	}
	if cfg.DatabaseConfig.Path != filepath.Join(dir, "data/app-state/library.sqlite3") ||
		cfg.Tools.FFmpegPath != filepath.Join(dir, "bin/ffmpeg") {
		t.Fatalf("relative paths did not use candidate base: %+v", cfg)
	}

	if _, err := LoadAppConfigBytes(path, []byte(contents+"\nunknown_field = true\n")); err == nil {
		t.Fatal("LoadAppConfigBytes accepted an unknown field")
	}
}

func TestLoadAppConfigRejectsUnknownAndLegacyFields(t *testing.T) {
	for name, contents := range map[string]string{
		"unknown":            completeManifest + "\nunknown_field = true\n",
		"legacy host":        strings.Replace(completeManifest, "path = \"data/app-state/library.sqlite3\"", "path = \"data/app-state/library.sqlite3\"\nhost = \"localhost\"", 1),
		"legacy password":    strings.Replace(completeManifest, "path = \"data/app-state/library.sqlite3\"", "path = \"data/app-state/library.sqlite3\"\npassword = \"plaintext\"", 1),
		"legacy server port": strings.Replace(completeManifest, "listen = \"127.0.0.1:6680\"", "port = \"6680\"", 1),
		"legacy server log":  strings.Replace(completeManifest, "listen = \"127.0.0.1:6680\"", "listen = \"127.0.0.1:6680\"\nlog_level = \"debug\"", 1),
		"legacy RP fields":   strings.Replace(completeManifest, "name = \"Lumilio Photos\"", "name = \"Lumilio Photos\"\nwebauthn_rp_id = \"localhost\"", 1),
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
	path := writeManifestFixture(t, "schema_version = 3\n")
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

func TestLoadAppConfigRequiresExplicitPath(t *testing.T) {
	if _, err := LoadAppConfig(""); err == nil {
		t.Fatal("expected empty path to fail")
	}
	missing := filepath.Join(t.TempDir(), "missing.toml")
	if _, err := LoadAppConfig(missing); err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("expected path in error, got %v", err)
	}
}

func TestLoadAppConfigRejectsV1AndInMemoryDatabase(t *testing.T) {
	for name, contents := range map[string]string{
		"schema v2": strings.Replace(completeManifest, "schema_version = 3", "schema_version = 2", 1),
		"in memory": strings.Replace(completeManifest, `path = "data/app-state/library.sqlite3"`, `path = ":memory:"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := LoadAppConfig(writeManifestFixture(t, contents))
			if err == nil {
				t.Fatal("expected manifest rejection")
			}
		})
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
		"database":    {`path = "data/app-state/library.sqlite3"`, `path = "data/storage/library.sqlite3"`, "database.path"},
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

func TestLoadAppConfigAcceptsDeploymentProfiles(t *testing.T) {
	profiles := map[string]string{
		"desktop local": networkManifest(`
[server]
listen = "127.0.0.1:6680"
primary_origin = "http://localhost:6680"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "off"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "disabled"
trusted_cidrs = []
`),
		"desktop LAN HTTP": networkManifest(`
[server]
listen = "0.0.0.0:6680"
primary_origin = "http://localhost:6680"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "off"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "disabled"
trusted_cidrs = []
`),
		"desktop same-host external HTTPS": networkManifest(`
[server]
listen = "127.0.0.1:6680"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "external"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "required"
trusted_cidrs = ["127.0.0.1/32", "::1/128"]
`),
		"desktop remote external HTTPS": networkManifest(`
[server]
listen = "192.168.1.20:6680"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "external"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "required"
trusted_cidrs = ["192.168.1.10/32"]
`),
		"docker built-in ACME": networkManifest(`
[server]
listen = "0.0.0.0:8443"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = "/app/web"
[server.tls]
mode = "acme"
http_listen = "0.0.0.0:8080"
email = "admin@example.com"
storage_path = "data/app-state/tls"
[server.proxy]
mode = "disabled"
trusted_cidrs = []
`),
		"docker external proxy": networkManifest(`
[server]
listen = "0.0.0.0:6680"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = "/app/web"
[server.tls]
mode = "external"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "required"
trusted_cidrs = ["172.30.0.0/24"]
`),
		"development Vite origin": completeManifest,
	}

	for name, contents := range profiles {
		t.Run(name, func(t *testing.T) {
			cfg, err := LoadAppConfig(writeManifestFixture(t, contents))
			if err != nil {
				t.Fatalf("LoadAppConfig: %v", err)
			}
			if cfg.Auth.PasskeyIdentity.Origin != cfg.ServerConfig.PrimaryOrigin {
				t.Fatalf("passkey origin %q != primary origin %q", cfg.Auth.PasskeyIdentity.Origin, cfg.ServerConfig.PrimaryOrigin)
			}
			_, parsed, err := NormalizeOrigin(cfg.ServerConfig.PrimaryOrigin)
			if err != nil || cfg.Auth.PasskeyIdentity.RPID != parsed.Hostname() {
				t.Fatalf("passkey RP ID %q was not derived from primary hostname", cfg.Auth.PasskeyIdentity.RPID)
			}
		})
	}
}

func TestLoadAppConfigRejectsInvalidNetworkCombinations(t *testing.T) {
	cases := map[string]struct {
		server string
		want   string
	}{
		"ACME with HTTP primary": {`
[server]
listen = "0.0.0.0:8443"
primary_origin = "http://photos.example.com"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "acme"
http_listen = "0.0.0.0:8080"
email = "admin@example.com"
storage_path = "data/app-state/tls"
[server.proxy]
mode = "disabled"
trusted_cidrs = []
`, "requires an https primary origin"},
		"ACME with IP hostname": {`
[server]
listen = "0.0.0.0:8443"
primary_origin = "https://203.0.113.10"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "acme"
http_listen = "0.0.0.0:8080"
email = "admin@example.com"
storage_path = "data/app-state/tls"
[server.proxy]
mode = "disabled"
trusted_cidrs = []
`, "requires a public DNS hostname"},
		"external without proxy": {`
[server]
listen = "127.0.0.1:6680"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "external"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "disabled"
trusted_cidrs = []
`, "requires proxy mode required"},
		"required proxy without CIDR": {`
[server]
listen = "127.0.0.1:6680"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "external"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "required"
trusted_cidrs = []
`, "must contain at least one CIDR"},
		"off with HTTPS primary": {`
[server]
listen = "127.0.0.1:6680"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "off"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "disabled"
trusted_cidrs = []
`, "requires an http primary origin"},
		"passkey with HTTP LAN primary": {`
[server]
listen = "0.0.0.0:6680"
primary_origin = "http://192.168.1.20:6680"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "off"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "disabled"
trusted_cidrs = []
`, "exact http://localhost"},
		"disabled proxy with CIDR": {`
[server]
listen = "127.0.0.1:6680"
primary_origin = "http://localhost:6680"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "off"
http_listen = ""
email = ""
storage_path = ""
[server.proxy]
mode = "disabled"
trusted_cidrs = ["127.0.0.1/32"]
`, "must be empty when proxy mode is disabled"},
		"ACME listener collision": {`
[server]
listen = "0.0.0.0:8443"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = ""
[server.tls]
mode = "acme"
http_listen = "127.0.0.1:8443"
email = "admin@example.com"
storage_path = "data/app-state/tls"
[server.proxy]
mode = "disabled"
trusted_cidrs = []
`, "must not conflict"},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := LoadAppConfig(writeManifestFixture(t, networkManifest(test.server)))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q, got %v", test.want, err)
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
		t.Fatalf("normalized origin = %q, hostname = %q", normalized, parsed.Hostname())
	}

	for _, invalid := range []string{
		"https://photos.example.com/",
		"https://user@photos.example.com",
		"https://photos.example.com/path",
		"https://photos.example.com?query=1",
		"https://photos.example.com#fragment",
	} {
		if _, _, err := NormalizeOrigin(invalid); err == nil {
			t.Fatalf("expected invalid origin %q", invalid)
		}
	}
}

func TestGenerateDockerProductionProfiles(t *testing.T) {
	tests := []struct {
		name    string
		options InitOptions
		check   func(*testing.T, AppConfig)
	}{
		{
			name: "ACME",
			options: InitOptions{
				Profile: InitProfileDockerACME,
				Origin:  "https://photos.example.com",
				Email:   "admin@example.com",
			},
			check: func(t *testing.T, cfg AppConfig) {
				if cfg.ServerConfig.TLS.Mode != TLSModeACME ||
					cfg.ServerConfig.Listen != "0.0.0.0:8443" ||
					cfg.ServerConfig.TLS.HTTPListen != "0.0.0.0:8080" ||
					cfg.ServerConfig.TLS.StoragePath != "/data/app-state/tls" {
					t.Fatalf("ACME config = %+v", cfg.ServerConfig)
				}
			},
		},
		{
			name: "external proxy",
			options: InitOptions{
				Profile:           InitProfileDockerExternalProxy,
				Origin:            "https://photos.example.com",
				TrustedProxyCIDRs: []string{"172.30.0.0/24"},
			},
			check: func(t *testing.T, cfg AppConfig) {
				if cfg.ServerConfig.TLS.Mode != TLSModeExternal ||
					cfg.ServerConfig.Proxy.Mode != ProxyModeRequired ||
					len(cfg.ServerConfig.Proxy.TrustedCIDRs) != 1 {
					t.Fatalf("external proxy config = %+v", cfg.ServerConfig)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := GenerateManifest(test.options)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "server.toml")
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := LoadAppConfig(path)
			if err != nil {
				t.Fatalf("strict load generated profile: %v\n%s", err, data)
			}
			if cfg.Auth.PasskeyIdentity.Origin != "https://photos.example.com" ||
				cfg.Auth.PasskeyIdentity.RPID != "photos.example.com" {
				t.Fatalf("generated Passkey identity = %+v", cfg.Auth.PasskeyIdentity)
			}
			test.check(t, cfg)
		})
	}
}

func networkManifest(serverBlock string) string {
	start := strings.Index(completeManifest, "[server]")
	end := strings.Index(completeManifest, "[logging]")
	if start < 0 || end < 0 || end <= start {
		panic("complete manifest server block markers are missing")
	}
	return completeManifest[:start] + strings.TrimSpace(serverBlock) + "\n" + completeManifest[end:]
}
