package config

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/net/idna"
)

const SchemaVersion = 3

// AppConfig is the fully resolved, runtime-immutable configuration consumed by
// server/app. Production hosts obtain it only from LoadAppConfig.
type AppConfig struct {
	SchemaVersion  int
	ManifestPath   string
	ManifestSHA256 string
	Environment    string
	DatabaseConfig DatabaseConfig
	ServerConfig   ServerConfig
	LoggingConfig  LoggingConfig
	StorageConfig  StorageConfig
	RepositoryScan RepositoryScanConfig
	Geocoding      GeocodingConfig
	Auth           AuthConfig
	Transcode      TranscodeConfig
	Lumen          LumenConfig
	Tools          ToolsConfig
	loaded         bool
}

// LoadedFromManifest reports whether the strict loader produced this value.
func (c AppConfig) LoadedFromManifest() bool { return c.loaded }

type DatabaseConfig struct {
	Path string
}

type ServerConfig struct {
	Listen             string
	PrimaryOrigin      string
	CORSAllowedOrigins []string
	WebRoot            string
	TLS                TLSConfig
	Proxy              ProxyConfig
}

type TLSMode string

const (
	TLSModeOff      TLSMode = "off"
	TLSModeACME     TLSMode = "acme"
	TLSModeExternal TLSMode = "external"
)

type TLSConfig struct {
	Mode        TLSMode
	HTTPListen  string
	Email       string
	StoragePath string
}

type ProxyMode string

const (
	ProxyModeDisabled ProxyMode = "disabled"
	ProxyModeRequired ProxyMode = "required"
)

type ProxyConfig struct {
	Mode         ProxyMode
	TrustedCIDRs []netip.Prefix
}

type LoggingConfig struct {
	Level                  string
	LogDir                 string
	ConsoleFormat          string
	FileFormat             string
	RepositoryAuditVerbose bool
}

type StorageConfig struct {
	// Path is the configured default repository root. It contains only the
	// .lumilioroot marker and repository directories.
	Path string
	// CloudStatePath stores provider sessions and credential artifacts. It must
	// stay outside Path because it is machine-bound private state.
	CloudStatePath string
	// BackupsPath is an explicit database-backup destination. Desktop binds it
	// to local app data; standalone operators may choose another private mount.
	BackupsPath string
}

func (c StorageConfig) CloudDir() string   { return c.CloudStatePath }
func (c StorageConfig) BackupsDir() string { return c.BackupsPath }

type RepositoryScanConfig struct {
	Enabled            bool
	IntervalSeconds    int
	SettleSeconds      int
	MaxConcurrentRepos int
	BatchSize          int
}

type GeocodingConfig struct {
	Provider          string
	NominatimEndpoint string
	Language          string
	UserAgent         string
}

type AuthConfig struct {
	SecretKeyFile   string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	MediaTokenTTL   time.Duration
	Passkey         PasskeyConfig
	// PasskeyIdentity is derived exclusively from server.primary_origin.
	// It is resolved by the strict loader and is never a manifest input.
	PasskeyIdentity PasskeyIdentity
	RateLimit       AuthRateLimitConfig
}

type PasskeyConfig struct {
	Enabled bool
	Name    string
}

type PasskeyIdentity struct {
	Origin string
	RPID   string
}

type AuthRateLimitConfig struct {
	IPAttempts      int
	SubjectAttempts int
	Window          time.Duration
	Lockout         time.Duration
	MaxEntries      int
}

type TranscodeConfig struct{ HardwareAccel string }

type LumenConfig struct {
	DiscoveryEnabled      bool
	DiscoveryMDNSEnabled  bool
	DiscoveryHubURL       string
	DiscoveryStaticNodes  []string
	DiscoveryServiceType  string
	DiscoveryDomain       string
	DeploymentID          string
	ResolveTimeout        time.Duration
	ConnectTimeout        time.Duration
	RediscoveryBackoffMin time.Duration
	RediscoveryBackoffMax time.Duration
	ScanInterval          time.Duration
	ChunkAuto             bool
	ChunkThresholdBytes   int
	ChunkMaxBytes         int
}

func (c LumenConfig) StaticNodes() []string {
	return append([]string(nil), c.DiscoveryStaticNodes...)
}

func (c LumenConfig) Enabled() bool { return c.DiscoveryEnabled }

// manifest uses pointers for every value so an omitted field is distinct from
// a deliberately configured false, zero, empty string, or empty array.
type manifest struct {
	SchemaVersion  *int                    `toml:"schema_version"`
	Environment    *string                 `toml:"environment"`
	Database       *databaseManifest       `toml:"database"`
	Server         *serverManifest         `toml:"server"`
	Logging        *loggingManifest        `toml:"logging"`
	Storage        *storageManifest        `toml:"storage"`
	RepositoryScan *repositoryScanManifest `toml:"repository_scan"`
	Geocoding      *geocodingManifest      `toml:"geocoding"`
	Auth           *authManifest           `toml:"auth"`
	Transcode      *transcodeManifest      `toml:"transcode"`
	Lumen          *lumenManifest          `toml:"lumen"`
	Tools          *toolsManifest          `toml:"tools"`
}

type databaseManifest struct {
	Path *string `toml:"path"`
}
type serverManifest struct {
	Listen             *string        `toml:"listen"`
	PrimaryOrigin      *string        `toml:"primary_origin"`
	CORSAllowedOrigins *[]string      `toml:"cors_allowed_origins"`
	WebRoot            *string        `toml:"web_root"`
	TLS                *tlsManifest   `toml:"tls"`
	Proxy              *proxyManifest `toml:"proxy"`
}
type tlsManifest struct {
	Mode        *string `toml:"mode"`
	HTTPListen  *string `toml:"http_listen"`
	Email       *string `toml:"email"`
	StoragePath *string `toml:"storage_path"`
}
type proxyManifest struct {
	Mode         *string   `toml:"mode"`
	TrustedCIDRs *[]string `toml:"trusted_cidrs"`
}
type loggingManifest struct {
	Level                  *string `toml:"level"`
	Dir                    *string `toml:"dir"`
	ConsoleFormat          *string `toml:"console_format"`
	FileFormat             *string `toml:"file_format"`
	RepositoryAuditVerbose *bool   `toml:"repository_audit_verbose"`
}
type storageManifest struct {
	Path           *string `toml:"path"`
	CloudStatePath *string `toml:"cloud_state_path"`
	BackupsPath    *string `toml:"backups_path"`
}
type repositoryScanManifest struct {
	Enabled            *bool `toml:"enabled"`
	IntervalSeconds    *int  `toml:"interval_seconds"`
	SettleSeconds      *int  `toml:"settle_seconds"`
	MaxConcurrentRepos *int  `toml:"max_concurrent_repos"`
	BatchSize          *int  `toml:"batch_size"`
}
type geocodingManifest struct {
	Provider          *string `toml:"provider"`
	NominatimEndpoint *string `toml:"nominatim_endpoint"`
	Language          *string `toml:"language"`
	UserAgent         *string `toml:"user_agent"`
}
type authManifest struct {
	SecretKeyFile   *string                `toml:"secret_key_file"`
	AccessTokenTTL  *string                `toml:"access_token_ttl"`
	RefreshTokenTTL *string                `toml:"refresh_token_ttl"`
	MediaTokenTTL   *string                `toml:"media_token_ttl"`
	Passkey         *passkeyManifest       `toml:"passkey"`
	RateLimit       *authRateLimitManifest `toml:"rate_limit"`
}
type passkeyManifest struct {
	Enabled *bool   `toml:"enabled"`
	Name    *string `toml:"name"`
}
type authRateLimitManifest struct {
	IPAttempts      *int    `toml:"ip_attempts"`
	SubjectAttempts *int    `toml:"subject_attempts"`
	Window          *string `toml:"window"`
	Lockout         *string `toml:"lockout"`
	MaxEntries      *int    `toml:"max_entries"`
}
type transcodeManifest struct {
	HardwareAccel *string `toml:"hardware_accel"`
}
type lumenManifest struct {
	DiscoveryEnabled      *bool     `toml:"discovery_enabled"`
	DiscoveryMDNSEnabled  *bool     `toml:"discovery_mdns_enabled"`
	DiscoveryHubURL       *string   `toml:"discovery_hub_url"`
	DiscoveryStaticNodes  *[]string `toml:"discovery_static_nodes"`
	DiscoveryServiceType  *string   `toml:"discovery_service_type"`
	DiscoveryDomain       *string   `toml:"discovery_domain"`
	DeploymentID          *string   `toml:"deployment_id"`
	ResolveTimeout        *string   `toml:"resolve_timeout"`
	ConnectTimeout        *string   `toml:"connect_timeout"`
	RediscoveryBackoffMin *string   `toml:"rediscovery_backoff_min"`
	RediscoveryBackoffMax *string   `toml:"rediscovery_backoff_max"`
	ScanInterval          *string   `toml:"scan_interval"`
	ChunkAuto             *bool     `toml:"chunk_auto"`
	ChunkThresholdBytes   *int      `toml:"chunk_threshold_bytes"`
	ChunkMaxBytes         *int      `toml:"chunk_max_bytes"`
}
type toolsManifest struct {
	ExifToolPath *string `toml:"exiftool_path"`
	FFmpegPath   *string `toml:"ffmpeg_path"`
	FFprobePath  *string `toml:"ffprobe_path"`
}

// LoadAppConfig strictly loads one complete runtime manifest. It never searches
// for files, reads environment variables, or fills missing fields.
func LoadAppConfig(path string) (AppConfig, error) {
	if strings.TrimSpace(path) == "" {
		return AppConfig{}, errors.New("config path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return AppConfig{}, fmt.Errorf("resolve config path %q: %w", path, err)
	}
	absPath = filepath.Clean(absPath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return AppConfig{}, fmt.Errorf("read runtime manifest %s: %w", absPath, err)
	}

	var raw manifest
	decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return AppConfig{}, fmt.Errorf("decode runtime manifest %s: %w", absPath, err)
	}

	problems := validateManifestPresence(raw)
	if len(problems) != 0 {
		return AppConfig{}, invalidConfig(problems)
	}
	cfg, problems := resolveManifest(raw, filepath.Dir(absPath))
	if len(problems) != 0 {
		return AppConfig{}, invalidConfig(problems)
	}
	sum := sha256.Sum256(data)
	cfg.SchemaVersion = *raw.SchemaVersion
	cfg.ManifestPath = absPath
	cfg.ManifestSHA256 = fmt.Sprintf("%x", sum)
	cfg.loaded = true
	return cfg, nil
}

func validateManifestPresence(m manifest) []string {
	var p []string
	required(&p, "schema_version", m.SchemaVersion)
	required(&p, "environment", m.Environment)
	requiredSection(&p, "database", m.Database)
	requiredSection(&p, "server", m.Server)
	requiredSection(&p, "logging", m.Logging)
	requiredSection(&p, "storage", m.Storage)
	requiredSection(&p, "repository_scan", m.RepositoryScan)
	requiredSection(&p, "geocoding", m.Geocoding)
	requiredSection(&p, "auth", m.Auth)
	requiredSection(&p, "transcode", m.Transcode)
	requiredSection(&p, "lumen", m.Lumen)
	requiredSection(&p, "tools", m.Tools)
	if m.Database != nil {
		required(&p, "database.path", m.Database.Path)
	}
	if m.Server != nil {
		required(&p, "server.listen", m.Server.Listen)
		required(&p, "server.primary_origin", m.Server.PrimaryOrigin)
		required(&p, "server.cors_allowed_origins", m.Server.CORSAllowedOrigins)
		required(&p, "server.web_root", m.Server.WebRoot)
		requiredSection(&p, "server.tls", m.Server.TLS)
		if m.Server.TLS != nil {
			required(&p, "server.tls.mode", m.Server.TLS.Mode)
			required(&p, "server.tls.http_listen", m.Server.TLS.HTTPListen)
			required(&p, "server.tls.email", m.Server.TLS.Email)
			required(&p, "server.tls.storage_path", m.Server.TLS.StoragePath)
		}
		requiredSection(&p, "server.proxy", m.Server.Proxy)
		if m.Server.Proxy != nil {
			required(&p, "server.proxy.mode", m.Server.Proxy.Mode)
			required(&p, "server.proxy.trusted_cidrs", m.Server.Proxy.TrustedCIDRs)
		}
	}
	if m.Logging != nil {
		required(&p, "logging.level", m.Logging.Level)
		required(&p, "logging.dir", m.Logging.Dir)
		required(&p, "logging.console_format", m.Logging.ConsoleFormat)
		required(&p, "logging.file_format", m.Logging.FileFormat)
		required(&p, "logging.repository_audit_verbose", m.Logging.RepositoryAuditVerbose)
	}
	if m.Storage != nil {
		required(&p, "storage.path", m.Storage.Path)
		required(&p, "storage.cloud_state_path", m.Storage.CloudStatePath)
		required(&p, "storage.backups_path", m.Storage.BackupsPath)
	}
	if m.RepositoryScan != nil {
		required(&p, "repository_scan.enabled", m.RepositoryScan.Enabled)
		required(&p, "repository_scan.interval_seconds", m.RepositoryScan.IntervalSeconds)
		required(&p, "repository_scan.settle_seconds", m.RepositoryScan.SettleSeconds)
		required(&p, "repository_scan.max_concurrent_repos", m.RepositoryScan.MaxConcurrentRepos)
		required(&p, "repository_scan.batch_size", m.RepositoryScan.BatchSize)
	}
	if m.Geocoding != nil {
		required(&p, "geocoding.provider", m.Geocoding.Provider)
		required(&p, "geocoding.nominatim_endpoint", m.Geocoding.NominatimEndpoint)
		required(&p, "geocoding.language", m.Geocoding.Language)
		required(&p, "geocoding.user_agent", m.Geocoding.UserAgent)
	}
	if m.Auth != nil {
		required(&p, "auth.secret_key_file", m.Auth.SecretKeyFile)
		required(&p, "auth.access_token_ttl", m.Auth.AccessTokenTTL)
		required(&p, "auth.refresh_token_ttl", m.Auth.RefreshTokenTTL)
		required(&p, "auth.media_token_ttl", m.Auth.MediaTokenTTL)
		requiredSection(&p, "auth.passkey", m.Auth.Passkey)
		if m.Auth.Passkey != nil {
			required(&p, "auth.passkey.enabled", m.Auth.Passkey.Enabled)
			required(&p, "auth.passkey.name", m.Auth.Passkey.Name)
		}
		requiredSection(&p, "auth.rate_limit", m.Auth.RateLimit)
		if m.Auth.RateLimit != nil {
			required(&p, "auth.rate_limit.ip_attempts", m.Auth.RateLimit.IPAttempts)
			required(&p, "auth.rate_limit.subject_attempts", m.Auth.RateLimit.SubjectAttempts)
			required(&p, "auth.rate_limit.window", m.Auth.RateLimit.Window)
			required(&p, "auth.rate_limit.lockout", m.Auth.RateLimit.Lockout)
			required(&p, "auth.rate_limit.max_entries", m.Auth.RateLimit.MaxEntries)
		}
	}
	if m.Transcode != nil {
		required(&p, "transcode.hardware_accel", m.Transcode.HardwareAccel)
	}
	if m.Lumen != nil {
		required(&p, "lumen.discovery_enabled", m.Lumen.DiscoveryEnabled)
		required(&p, "lumen.discovery_mdns_enabled", m.Lumen.DiscoveryMDNSEnabled)
		required(&p, "lumen.discovery_hub_url", m.Lumen.DiscoveryHubURL)
		required(&p, "lumen.discovery_static_nodes", m.Lumen.DiscoveryStaticNodes)
		required(&p, "lumen.discovery_service_type", m.Lumen.DiscoveryServiceType)
		required(&p, "lumen.discovery_domain", m.Lumen.DiscoveryDomain)
		required(&p, "lumen.deployment_id", m.Lumen.DeploymentID)
		required(&p, "lumen.resolve_timeout", m.Lumen.ResolveTimeout)
		required(&p, "lumen.connect_timeout", m.Lumen.ConnectTimeout)
		required(&p, "lumen.rediscovery_backoff_min", m.Lumen.RediscoveryBackoffMin)
		required(&p, "lumen.rediscovery_backoff_max", m.Lumen.RediscoveryBackoffMax)
		required(&p, "lumen.scan_interval", m.Lumen.ScanInterval)
		required(&p, "lumen.chunk_auto", m.Lumen.ChunkAuto)
		required(&p, "lumen.chunk_threshold_bytes", m.Lumen.ChunkThresholdBytes)
		required(&p, "lumen.chunk_max_bytes", m.Lumen.ChunkMaxBytes)
	}
	if m.Tools != nil {
		required(&p, "tools.exiftool_path", m.Tools.ExifToolPath)
		required(&p, "tools.ffmpeg_path", m.Tools.FFmpegPath)
		required(&p, "tools.ffprobe_path", m.Tools.FFprobePath)
	}
	return p
}

func required[T any](p *[]string, name string, value *T) {
	if value == nil {
		*p = append(*p, name+" is required")
	}
}
func requiredSection[T any](p *[]string, name string, value *T) {
	if value == nil {
		*p = append(*p, "["+name+"] is required")
	}
}

func resolveManifest(m manifest, base string) (AppConfig, []string) {
	var p []string
	if *m.SchemaVersion != SchemaVersion {
		p = append(p, fmt.Sprintf("schema_version must be %d", SchemaVersion))
	}
	environment := normalizedRequired(&p, "environment", *m.Environment)
	if environment != "development" && environment != "production" && environment != "test" {
		p = append(p, "environment must be one of development, production, test")
	}

	rawDatabasePath := strings.TrimSpace(*m.Database.Path)
	db := DatabaseConfig{Path: resolvePath(base, rawDatabasePath)}
	requireNonEmpty(&p, "database.path", rawDatabasePath)
	if rawDatabasePath == ":memory:" {
		p = append(p, "database.path must be a persistent filesystem path")
	}

	server := ServerConfig{
		Listen:        strings.TrimSpace(*m.Server.Listen),
		PrimaryOrigin: strings.TrimSpace(*m.Server.PrimaryOrigin),
		WebRoot:       resolveOptionalPath(base, *m.Server.WebRoot),
		TLS: TLSConfig{
			Mode:        TLSMode(strings.ToLower(strings.TrimSpace(*m.Server.TLS.Mode))),
			HTTPListen:  strings.TrimSpace(*m.Server.TLS.HTTPListen),
			Email:       strings.TrimSpace(*m.Server.TLS.Email),
			StoragePath: resolveOptionalPath(base, *m.Server.TLS.StoragePath),
		},
		Proxy: ProxyConfig{
			Mode: ProxyMode(strings.ToLower(strings.TrimSpace(*m.Server.Proxy.Mode))),
		},
	}
	validateListenAddress(&p, "server.listen", server.Listen)

	normalizedPrimary, primaryURL, originErr := NormalizeOrigin(server.PrimaryOrigin)
	if originErr != nil {
		p = append(p, "server.primary_origin must be an exact http(s) origin: "+originErr.Error())
	} else {
		server.PrimaryOrigin = normalizedPrimary
	}

	server.CORSAllowedOrigins = normalizeOriginList(
		&p,
		"server.cors_allowed_origins",
		*m.Server.CORSAllowedOrigins,
	)
	requireOneOf(&p, "server.tls.mode", string(server.TLS.Mode), string(TLSModeOff), string(TLSModeACME), string(TLSModeExternal))
	requireOneOf(&p, "server.proxy.mode", string(server.Proxy.Mode), string(ProxyModeDisabled), string(ProxyModeRequired))
	server.Proxy.TrustedCIDRs = parseTrustedCIDRs(&p, *m.Server.Proxy.TrustedCIDRs)
	validateNetworkTopology(&p, server, primaryURL)

	logging := LoggingConfig{Level: strings.ToLower(strings.TrimSpace(*m.Logging.Level)), LogDir: resolvePath(base, *m.Logging.Dir), ConsoleFormat: strings.ToLower(strings.TrimSpace(*m.Logging.ConsoleFormat)), FileFormat: strings.ToLower(strings.TrimSpace(*m.Logging.FileFormat)), RepositoryAuditVerbose: *m.Logging.RepositoryAuditVerbose}
	requireOneOf(&p, "logging.level", logging.Level, "debug", "info", "warn", "error")
	requireNonEmpty(&p, "logging.dir", strings.TrimSpace(*m.Logging.Dir))
	requireOneOf(&p, "logging.console_format", logging.ConsoleFormat, "console", "json")
	requireOneOf(&p, "logging.file_format", logging.FileFormat, "console", "json")

	storage := StorageConfig{
		Path:           resolvePath(base, *m.Storage.Path),
		CloudStatePath: resolvePath(base, *m.Storage.CloudStatePath),
		BackupsPath:    resolvePath(base, *m.Storage.BackupsPath),
	}
	requireNonEmpty(&p, "storage.path", strings.TrimSpace(*m.Storage.Path))
	requireNonEmpty(&p, "storage.cloud_state_path", strings.TrimSpace(*m.Storage.CloudStatePath))
	requireNonEmpty(&p, "storage.backups_path", strings.TrimSpace(*m.Storage.BackupsPath))
	requireOutsidePath(&p, "storage.cloud_state_path", storage.CloudStatePath, storage.Path)
	requireOutsidePath(&p, "storage.backups_path", storage.BackupsPath, storage.Path)
	requireOutsidePath(&p, "logging.dir", logging.LogDir, storage.Path)
	requireOutsidePath(&p, "database.path", db.Path, storage.Path)
	requireOutsidePath(&p, "database.path", db.Path, storage.BackupsPath)
	if server.TLS.StoragePath != "" {
		requireOutsidePath(&p, "server.tls.storage_path", server.TLS.StoragePath, storage.Path)
	}
	scan := RepositoryScanConfig{Enabled: *m.RepositoryScan.Enabled, IntervalSeconds: *m.RepositoryScan.IntervalSeconds, SettleSeconds: *m.RepositoryScan.SettleSeconds, MaxConcurrentRepos: *m.RepositoryScan.MaxConcurrentRepos, BatchSize: *m.RepositoryScan.BatchSize}
	requirePositive(&p, "repository_scan.interval_seconds", scan.IntervalSeconds)
	requirePositive(&p, "repository_scan.settle_seconds", scan.SettleSeconds)
	requirePositive(&p, "repository_scan.max_concurrent_repos", scan.MaxConcurrentRepos)
	requirePositive(&p, "repository_scan.batch_size", scan.BatchSize)

	geocoding := GeocodingConfig{Provider: strings.ToLower(strings.TrimSpace(*m.Geocoding.Provider)), NominatimEndpoint: strings.TrimSpace(*m.Geocoding.NominatimEndpoint), Language: strings.TrimSpace(*m.Geocoding.Language), UserAgent: strings.TrimSpace(*m.Geocoding.UserAgent)}
	requireOneOf(&p, "geocoding.provider", geocoding.Provider, "disabled", "nominatim")
	requireNonEmpty(&p, "geocoding.nominatim_endpoint", geocoding.NominatimEndpoint)
	requireHTTPURL(&p, "geocoding.nominatim_endpoint", geocoding.NominatimEndpoint)
	requireNonEmpty(&p, "geocoding.language", geocoding.Language)
	requireNonEmpty(&p, "geocoding.user_agent", geocoding.UserAgent)

	auth := AuthConfig{
		SecretKeyFile: resolvePath(base, *m.Auth.SecretKeyFile),
		Passkey: PasskeyConfig{
			Enabled: *m.Auth.Passkey.Enabled,
			Name:    strings.TrimSpace(*m.Auth.Passkey.Name),
		},
	}
	requireNonEmpty(&p, "auth.secret_key_file", strings.TrimSpace(*m.Auth.SecretKeyFile))
	requireOutsidePath(&p, "auth.secret_key_file", auth.SecretKeyFile, storage.Path)
	requireNonEmpty(&p, "auth.passkey.name", auth.Passkey.Name)
	auth.AccessTokenTTL = parsePositiveDuration(&p, "auth.access_token_ttl", *m.Auth.AccessTokenTTL)
	auth.RefreshTokenTTL = parsePositiveDuration(&p, "auth.refresh_token_ttl", *m.Auth.RefreshTokenTTL)
	auth.MediaTokenTTL = parsePositiveDuration(&p, "auth.media_token_ttl", *m.Auth.MediaTokenTTL)
	auth.RateLimit = AuthRateLimitConfig{
		IPAttempts:      *m.Auth.RateLimit.IPAttempts,
		SubjectAttempts: *m.Auth.RateLimit.SubjectAttempts,
		MaxEntries:      *m.Auth.RateLimit.MaxEntries,
	}
	requirePositive(&p, "auth.rate_limit.ip_attempts", auth.RateLimit.IPAttempts)
	requirePositive(&p, "auth.rate_limit.subject_attempts", auth.RateLimit.SubjectAttempts)
	requirePositive(&p, "auth.rate_limit.max_entries", auth.RateLimit.MaxEntries)
	auth.RateLimit.Window = parsePositiveDuration(&p, "auth.rate_limit.window", *m.Auth.RateLimit.Window)
	auth.RateLimit.Lockout = parsePositiveDuration(&p, "auth.rate_limit.lockout", *m.Auth.RateLimit.Lockout)
	if primaryURL != nil {
		auth.PasskeyIdentity = PasskeyIdentity{
			Origin: server.PrimaryOrigin,
			RPID:   primaryURL.Hostname(),
		}
		if auth.Passkey.Enabled {
			validatePasskeyOrigin(&p, primaryURL)
		}
	}

	transcode := TranscodeConfig{HardwareAccel: strings.ToLower(strings.TrimSpace(*m.Transcode.HardwareAccel))}
	requireOneOf(&p, "transcode.hardware_accel", transcode.HardwareAccel, "auto", "vaapi", "nvenc", "qsv", "videotoolbox", "none")

	lumen := LumenConfig{DiscoveryEnabled: *m.Lumen.DiscoveryEnabled, DiscoveryMDNSEnabled: *m.Lumen.DiscoveryMDNSEnabled, DiscoveryHubURL: strings.TrimSpace(*m.Lumen.DiscoveryHubURL), DiscoveryStaticNodes: cleanStrings(*m.Lumen.DiscoveryStaticNodes), DiscoveryServiceType: strings.TrimSpace(*m.Lumen.DiscoveryServiceType), DiscoveryDomain: strings.TrimSpace(*m.Lumen.DiscoveryDomain), DeploymentID: strings.TrimSpace(*m.Lumen.DeploymentID), ChunkAuto: *m.Lumen.ChunkAuto, ChunkThresholdBytes: *m.Lumen.ChunkThresholdBytes, ChunkMaxBytes: *m.Lumen.ChunkMaxBytes}
	requireNonEmpty(&p, "lumen.discovery_service_type", lumen.DiscoveryServiceType)
	requireNonEmpty(&p, "lumen.discovery_domain", lumen.DiscoveryDomain)
	requireNonEmpty(&p, "lumen.deployment_id", lumen.DeploymentID)
	if !validMDNSServiceType(lumen.DiscoveryServiceType) {
		p = append(p, "lumen.discovery_service_type must look like _service._tcp or _service._udp")
	}
	if !validDomainName(lumen.DiscoveryDomain) {
		p = append(p, "lumen.discovery_domain must be a valid domain name")
	}
	lumen.ResolveTimeout = parsePositiveDuration(&p, "lumen.resolve_timeout", *m.Lumen.ResolveTimeout)
	lumen.ConnectTimeout = parsePositiveDuration(&p, "lumen.connect_timeout", *m.Lumen.ConnectTimeout)
	lumen.RediscoveryBackoffMin = parsePositiveDuration(&p, "lumen.rediscovery_backoff_min", *m.Lumen.RediscoveryBackoffMin)
	lumen.RediscoveryBackoffMax = parsePositiveDuration(&p, "lumen.rediscovery_backoff_max", *m.Lumen.RediscoveryBackoffMax)
	lumen.ScanInterval = parsePositiveDuration(&p, "lumen.scan_interval", *m.Lumen.ScanInterval)
	if lumen.RediscoveryBackoffMax < lumen.RediscoveryBackoffMin {
		p = append(p, "lumen.rediscovery_backoff_max must be greater than or equal to rediscovery_backoff_min")
	}
	if lumen.DiscoveryHubURL != "" {
		requireHTTPURL(&p, "lumen.discovery_hub_url", lumen.DiscoveryHubURL)
	}
	for i, node := range lumen.DiscoveryStaticNodes {
		if _, _, err := net.SplitHostPort(node); err != nil {
			p = append(p, fmt.Sprintf("lumen.discovery_static_nodes[%d] must be host:port", i))
		}
	}
	if lumen.DiscoveryEnabled && !lumen.DiscoveryMDNSEnabled && lumen.DiscoveryHubURL == "" && len(lumen.DiscoveryStaticNodes) == 0 {
		p = append(p, "lumen discovery_enabled requires at least one backend")
	}
	requirePositive(&p, "lumen.chunk_threshold_bytes", lumen.ChunkThresholdBytes)
	requirePositive(&p, "lumen.chunk_max_bytes", lumen.ChunkMaxBytes)
	if lumen.ChunkMaxBytes > lumen.ChunkThresholdBytes {
		p = append(p, "lumen.chunk_max_bytes must be less than or equal to chunk_threshold_bytes")
	}

	tools := ToolsConfig{ExifToolPath: resolveCommand(base, *m.Tools.ExifToolPath), FFmpegPath: resolveCommand(base, *m.Tools.FFmpegPath), FFprobePath: resolveCommand(base, *m.Tools.FFprobePath)}
	requireNonEmpty(&p, "tools.exiftool_path", tools.ExifToolPath)
	requireNonEmpty(&p, "tools.ffmpeg_path", tools.FFmpegPath)
	requireNonEmpty(&p, "tools.ffprobe_path", tools.FFprobePath)

	return AppConfig{Environment: environment, DatabaseConfig: db, ServerConfig: server, LoggingConfig: logging, StorageConfig: storage, RepositoryScan: scan, Geocoding: geocoding, Auth: auth, Transcode: transcode, Lumen: lumen, Tools: tools}, p
}

func invalidConfig(p []string) error {
	return fmt.Errorf("invalid runtime manifest: %s", strings.Join(p, "; "))
}
func normalizedRequired(p *[]string, name, value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	requireNonEmpty(p, name, v)
	return v
}
func requireNonEmpty(p *[]string, name, value string) {
	if strings.TrimSpace(value) == "" {
		*p = append(*p, name+" must be non-empty")
	}
}
func requirePositive(p *[]string, name string, value int) {
	if value <= 0 {
		*p = append(*p, name+" must be positive")
	}
}
func requireOneOf(p *[]string, name, value string, allowed ...string) {
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	*p = append(*p, fmt.Sprintf("%s must be one of %s", name, strings.Join(allowed, ", ")))
}

func validateListenAddress(p *[]string, name, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		*p = append(*p, name+" must be non-empty")
		return
	}
	if strings.Contains(value, "://") {
		*p = append(*p, name+" must be host:port, not a URL")
		return
	}
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		*p = append(*p, name+" must be a complete host:port address")
		return
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		*p = append(*p, name+" port must be between 1 and 65535")
	}
	if strings.Contains(host, "%") {
		*p = append(*p, name+" must not contain an IPv6 zone")
	}
}

// NormalizeOrigin parses and serializes one exact browser HTTP(S) origin.
// It performs IDNA lookup canonicalization, lower-cases scheme/hostname, and
// removes default ports. Paths (including a trailing slash), credentials,
// query strings, fragments, and IPv6 zones are rejected.
func NormalizeOrigin(value string) (string, *url.URL, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil {
		return "", nil, errors.New("invalid URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", nil, errors.New("scheme must be http or https")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return "", nil, errors.New("hostname is required")
	}
	if parsed.User != nil {
		return "", nil, errors.New("userinfo is not allowed")
	}
	if parsed.Path != "" || parsed.RawPath != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", nil, errors.New("path, query, and fragment are not allowed")
	}

	rawHost := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if strings.Contains(rawHost, "%") {
		return "", nil, errors.New("IPv6 zones are not allowed")
	}
	host := rawHost
	if ip, ipErr := netip.ParseAddr(rawHost); ipErr == nil {
		host = ip.String()
	} else {
		host, err = idna.Lookup.ToASCII(rawHost)
		if err != nil || !validDomainName(host) {
			return "", nil, errors.New("hostname is not a valid IDNA domain")
		}
		host = strings.ToLower(host)
	}

	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	serializedHost := host
	if port != "" {
		n, portErr := strconv.Atoi(port)
		if portErr != nil || n < 1 || n > 65535 {
			return "", nil, errors.New("port must be between 1 and 65535")
		}
		serializedHost = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		serializedHost = "[" + host + "]"
	}

	normalized := scheme + "://" + serializedHost
	normalizedURL, err := url.Parse(normalized)
	if err != nil {
		return "", nil, errors.New("failed to serialize origin")
	}
	return normalized, normalizedURL, nil
}

func normalizeOriginList(p *[]string, name string, values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for i, value := range values {
		normalized, _, err := NormalizeOrigin(value)
		if err != nil {
			*p = append(*p, fmt.Sprintf("%s[%d] must be an exact http(s) origin: %v", name, i, err))
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func parseTrustedCIDRs(p *[]string, values []string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(values))
	seen := make(map[netip.Prefix]struct{}, len(values))
	for i, value := range values {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(value))
		if err != nil {
			*p = append(*p, fmt.Sprintf("server.proxy.trusted_cidrs[%d] must be a valid CIDR", i))
			continue
		}
		prefix = prefix.Masked()
		if prefix.Bits() == 0 {
			*p = append(*p, fmt.Sprintf("server.proxy.trusted_cidrs[%d] must not trust an entire address family", i))
			continue
		}
		if _, exists := seen[prefix]; exists {
			continue
		}
		seen[prefix] = struct{}{}
		out = append(out, prefix)
	}
	return out
}

func validateNetworkTopology(p *[]string, server ServerConfig, primary *url.URL) {
	if server.Proxy.Mode == ProxyModeDisabled && len(server.Proxy.TrustedCIDRs) != 0 {
		*p = append(*p, "server.proxy.trusted_cidrs must be empty when proxy mode is disabled")
	}
	if server.Proxy.Mode == ProxyModeRequired && len(server.Proxy.TrustedCIDRs) == 0 {
		*p = append(*p, "server.proxy.trusted_cidrs must contain at least one CIDR when proxy mode is required")
	}
	if primary == nil {
		return
	}

	switch server.TLS.Mode {
	case TLSModeOff:
		if primary.Scheme != "http" {
			*p = append(*p, "server.tls.mode off requires an http primary origin")
		}
		requireEmptyTLSFields(p, server.TLS)
		if server.Proxy.Mode != ProxyModeDisabled {
			*p = append(*p, "server.tls.mode off requires proxy mode disabled")
		}
	case TLSModeExternal:
		if primary.Scheme != "https" {
			*p = append(*p, "server.tls.mode external requires an https primary origin")
		}
		requireEmptyTLSFields(p, server.TLS)
		if server.Proxy.Mode != ProxyModeRequired {
			*p = append(*p, "server.tls.mode external requires proxy mode required")
		}
	case TLSModeACME:
		if primary.Scheme != "https" {
			*p = append(*p, "server.tls.mode acme requires an https primary origin")
		}
		if primary.Hostname() == "localhost" || net.ParseIP(primary.Hostname()) != nil {
			*p = append(*p, "server.tls.mode acme requires a public DNS hostname")
		}
		if server.Proxy.Mode != ProxyModeDisabled {
			*p = append(*p, "server.tls.mode acme requires proxy mode disabled")
		}
		validateListenAddress(p, "server.tls.http_listen", server.TLS.HTTPListen)
		if listenAddressesConflict(server.Listen, server.TLS.HTTPListen) {
			*p = append(*p, "server.listen and server.tls.http_listen must not conflict")
		}
		if address, err := mail.ParseAddress(server.TLS.Email); err != nil || address.Address != server.TLS.Email {
			*p = append(*p, "server.tls.email must be a valid email address")
		}
		requireNonEmpty(p, "server.tls.storage_path", server.TLS.StoragePath)
	}
}

func requireEmptyTLSFields(p *[]string, tls TLSConfig) {
	if tls.HTTPListen != "" {
		*p = append(*p, "server.tls.http_listen must be empty unless TLS mode is acme")
	}
	if tls.Email != "" {
		*p = append(*p, "server.tls.email must be empty unless TLS mode is acme")
	}
	if tls.StoragePath != "" {
		*p = append(*p, "server.tls.storage_path must be empty unless TLS mode is acme")
	}
}

func listenAddressesConflict(left, right string) bool {
	leftHost, leftPort, leftErr := net.SplitHostPort(left)
	rightHost, rightPort, rightErr := net.SplitHostPort(right)
	if leftErr != nil || rightErr != nil || leftPort != rightPort {
		return false
	}
	if leftHost == rightHost || leftHost == "" || rightHost == "" {
		return true
	}
	leftIP, leftIPErr := netip.ParseAddr(leftHost)
	rightIP, rightIPErr := netip.ParseAddr(rightHost)
	if leftIPErr != nil || rightIPErr != nil {
		return strings.EqualFold(leftHost, rightHost)
	}
	if leftIP == rightIP {
		return true
	}
	return (leftIP.IsUnspecified() && leftIP.Is4() == rightIP.Is4()) ||
		(rightIP.IsUnspecified() && rightIP.Is4() == leftIP.Is4())
}

func validatePasskeyOrigin(p *[]string, origin *url.URL) {
	host := origin.Hostname()
	if origin.Scheme == "http" && host != "localhost" {
		*p = append(*p, "auth.passkey.enabled requires https or exact http://localhost[:port] primary origin")
		return
	}
	if origin.Scheme == "https" {
		if net.ParseIP(host) != nil || !validDomainName(host) {
			*p = append(*p, "auth.passkey.enabled requires an https domain name or exact http://localhost[:port]")
		}
	}
}
func requireOutsidePath(p *[]string, name, candidate, root string) {
	if strings.TrimSpace(candidate) == "" || strings.TrimSpace(root) == "" {
		return
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil || filepath.IsAbs(rel) {
		return
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		*p = append(*p, name+" must be outside storage.path")
	}
}
func requireHTTPURL(p *[]string, name, value string) {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		*p = append(*p, name+" must be an absolute http(s) URL")
	}
}
func parsePositiveDuration(p *[]string, name, value string) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil || d <= 0 {
		*p = append(*p, name+" must be a positive duration")
		return 0
	}
	return d
}
func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.TrimSpace(value))
	}
	return out
}
func validMDNSServiceType(value string) bool {
	parts := strings.Split(value, ".")
	return len(parts) == 2 && strings.HasPrefix(parts[0], "_") && len(parts[0]) > 1 && (parts[1] == "_tcp" || parts[1] == "_udp")
}
func validDomainName(value string) bool {
	value = strings.TrimSuffix(strings.TrimSpace(value), ".")
	if value == "" || len(value) > 253 || strings.ContainsAny(value, "/:") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}
func resolvePath(base, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(base, value))
}
func resolveOptionalPath(base, value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return resolvePath(base, value)
}
func resolveCommand(base, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || (!filepath.IsAbs(value) && !strings.ContainsAny(value, `/\`)) {
		return value
	}
	return resolvePath(base, value)
}
