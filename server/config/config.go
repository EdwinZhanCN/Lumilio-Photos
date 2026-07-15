package config

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const SchemaVersion = 1

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
	Host                  string
	Port                  string
	User                  string
	Password              string
	DBName                string
	SSL                   string
	BootstrapPasswordFile string
	RotatedPasswordFile   string
	BootstrapPassword     string
	ToolsBinDir           string
}

type ServerConfig struct {
	Port               string
	CORSAllowedOrigins []string
	WebRoot            string
}

type LoggingConfig struct {
	Level                  string
	LogDir                 string
	ConsoleFormat          string
	FileFormat             string
	RepositoryAuditVerbose bool
}

type StorageConfig struct{ Path string }

const (
	secretsDirName = ".secrets"
	cloudDirName   = ".cloud"
	primaryDirName = "primary"
	backupsDirName = "backups"
)

func (c StorageConfig) SecretsDir() string { return filepath.Join(c.Path, secretsDirName) }
func (c StorageConfig) CloudDir() string   { return filepath.Join(c.Path, cloudDirName) }
func (c StorageConfig) PrimaryDir() string { return filepath.Join(c.Path, primaryDirName) }
func (c StorageConfig) BackupsDir() string { return filepath.Join(c.Path, backupsDirName) }

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
	SecretKeyFile     string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	MediaTokenTTL     time.Duration
	WebAuthnRPName    string
	WebAuthnRPMode    string
	WebAuthnRPID      string
	WebAuthnRPOrigins []string
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
	Host                  *string `toml:"host"`
	Port                  *string `toml:"port"`
	User                  *string `toml:"user"`
	Name                  *string `toml:"name"`
	SSL                   *string `toml:"ssl"`
	BootstrapPasswordFile *string `toml:"bootstrap_password_file"`
	RotatedPasswordFile   *string `toml:"rotated_password_file"`
	ToolsBinDir           *string `toml:"tools_bin_dir"`
}
type serverManifest struct {
	Port               *string   `toml:"port"`
	CORSAllowedOrigins *[]string `toml:"cors_allowed_origins"`
	WebRoot            *string   `toml:"web_root"`
}
type loggingManifest struct {
	Level                  *string `toml:"level"`
	Dir                    *string `toml:"dir"`
	ConsoleFormat          *string `toml:"console_format"`
	FileFormat             *string `toml:"file_format"`
	RepositoryAuditVerbose *bool   `toml:"repository_audit_verbose"`
}
type storageManifest struct {
	Path *string `toml:"path"`
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
	SecretKeyFile     *string   `toml:"secret_key_file"`
	AccessTokenTTL    *string   `toml:"access_token_ttl"`
	RefreshTokenTTL   *string   `toml:"refresh_token_ttl"`
	MediaTokenTTL     *string   `toml:"media_token_ttl"`
	WebAuthnRPName    *string   `toml:"webauthn_rp_name"`
	WebAuthnRPMode    *string   `toml:"webauthn_rp_mode"`
	WebAuthnRPID      *string   `toml:"webauthn_rp_id"`
	WebAuthnRPOrigins *[]string `toml:"webauthn_rp_origins"`
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
		required(&p, "database.host", m.Database.Host)
		required(&p, "database.port", m.Database.Port)
		required(&p, "database.user", m.Database.User)
		required(&p, "database.name", m.Database.Name)
		required(&p, "database.ssl", m.Database.SSL)
		required(&p, "database.bootstrap_password_file", m.Database.BootstrapPasswordFile)
		required(&p, "database.rotated_password_file", m.Database.RotatedPasswordFile)
		required(&p, "database.tools_bin_dir", m.Database.ToolsBinDir)
	}
	if m.Server != nil {
		required(&p, "server.port", m.Server.Port)
		required(&p, "server.cors_allowed_origins", m.Server.CORSAllowedOrigins)
		required(&p, "server.web_root", m.Server.WebRoot)
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
		required(&p, "auth.webauthn_rp_name", m.Auth.WebAuthnRPName)
		required(&p, "auth.webauthn_rp_mode", m.Auth.WebAuthnRPMode)
		required(&p, "auth.webauthn_rp_id", m.Auth.WebAuthnRPID)
		required(&p, "auth.webauthn_rp_origins", m.Auth.WebAuthnRPOrigins)
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

	db := DatabaseConfig{
		Host: resolveHost(base, *m.Database.Host), Port: strings.TrimSpace(*m.Database.Port), User: strings.TrimSpace(*m.Database.User),
		DBName: strings.TrimSpace(*m.Database.Name), SSL: strings.ToLower(strings.TrimSpace(*m.Database.SSL)),
		BootstrapPasswordFile: resolvePath(base, *m.Database.BootstrapPasswordFile), RotatedPasswordFile: resolvePath(base, *m.Database.RotatedPasswordFile),
		ToolsBinDir: resolveOptionalPath(base, *m.Database.ToolsBinDir),
	}
	requireNonEmpty(&p, "database.host", db.Host)
	requirePort(&p, "database.port", db.Port)
	requireNonEmpty(&p, "database.user", db.User)
	requireNonEmpty(&p, "database.name", db.DBName)
	requireOneOf(&p, "database.ssl", db.SSL, "disable", "require", "verify-ca", "verify-full")
	requireNonEmpty(&p, "database.bootstrap_password_file", strings.TrimSpace(*m.Database.BootstrapPasswordFile))
	requireNonEmpty(&p, "database.rotated_password_file", strings.TrimSpace(*m.Database.RotatedPasswordFile))
	bootstrap, err := readRequiredSecret(db.BootstrapPasswordFile)
	if err != nil {
		p = append(p, fmt.Sprintf("database.bootstrap_password_file: %v", err))
	} else {
		db.BootstrapPassword = bootstrap
		db.Password = bootstrap
	}
	if rotated, exists, err := readOptionalSecret(db.RotatedPasswordFile); err != nil {
		p = append(p, fmt.Sprintf("database.rotated_password_file: %v", err))
	} else if exists {
		db.Password = rotated
	}

	server := ServerConfig{Port: strings.TrimSpace(*m.Server.Port), CORSAllowedOrigins: cleanStrings(*m.Server.CORSAllowedOrigins), WebRoot: resolveOptionalPath(base, *m.Server.WebRoot)}
	requirePort(&p, "server.port", server.Port)
	for i, origin := range server.CORSAllowedOrigins {
		validateOrigin(&p, fmt.Sprintf("server.cors_allowed_origins[%d]", i), origin)
	}

	logging := LoggingConfig{Level: strings.ToLower(strings.TrimSpace(*m.Logging.Level)), LogDir: resolvePath(base, *m.Logging.Dir), ConsoleFormat: strings.ToLower(strings.TrimSpace(*m.Logging.ConsoleFormat)), FileFormat: strings.ToLower(strings.TrimSpace(*m.Logging.FileFormat)), RepositoryAuditVerbose: *m.Logging.RepositoryAuditVerbose}
	requireOneOf(&p, "logging.level", logging.Level, "debug", "info", "warn", "error")
	requireNonEmpty(&p, "logging.dir", strings.TrimSpace(*m.Logging.Dir))
	requireOneOf(&p, "logging.console_format", logging.ConsoleFormat, "console", "json")
	requireOneOf(&p, "logging.file_format", logging.FileFormat, "console", "json")

	storage := StorageConfig{Path: resolvePath(base, *m.Storage.Path)}
	requireNonEmpty(&p, "storage.path", strings.TrimSpace(*m.Storage.Path))
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

	auth := AuthConfig{SecretKeyFile: resolvePath(base, *m.Auth.SecretKeyFile), WebAuthnRPName: strings.TrimSpace(*m.Auth.WebAuthnRPName), WebAuthnRPMode: strings.ToLower(strings.TrimSpace(*m.Auth.WebAuthnRPMode)), WebAuthnRPID: strings.TrimSpace(*m.Auth.WebAuthnRPID), WebAuthnRPOrigins: cleanStrings(*m.Auth.WebAuthnRPOrigins)}
	requireNonEmpty(&p, "auth.secret_key_file", strings.TrimSpace(*m.Auth.SecretKeyFile))
	requireNonEmpty(&p, "auth.webauthn_rp_name", auth.WebAuthnRPName)
	requireOneOf(&p, "auth.webauthn_rp_mode", auth.WebAuthnRPMode, "origin-derived", "fixed")
	auth.AccessTokenTTL = parsePositiveDuration(&p, "auth.access_token_ttl", *m.Auth.AccessTokenTTL)
	auth.RefreshTokenTTL = parsePositiveDuration(&p, "auth.refresh_token_ttl", *m.Auth.RefreshTokenTTL)
	auth.MediaTokenTTL = parsePositiveDuration(&p, "auth.media_token_ttl", *m.Auth.MediaTokenTTL)
	if auth.WebAuthnRPMode == "origin-derived" && (auth.WebAuthnRPID != "" || len(auth.WebAuthnRPOrigins) != 0) {
		p = append(p, "auth origin-derived mode requires empty webauthn_rp_id and webauthn_rp_origins")
	}
	if auth.WebAuthnRPMode == "fixed" {
		requireNonEmpty(&p, "auth.webauthn_rp_id", auth.WebAuthnRPID)
		if auth.WebAuthnRPID != "" && !validDomainName(auth.WebAuthnRPID) {
			p = append(p, "auth.webauthn_rp_id must be a domain name without scheme, port, or path")
		}
		if len(auth.WebAuthnRPOrigins) == 0 {
			p = append(p, "auth.webauthn_rp_origins must contain at least one origin in fixed mode")
		}
	}
	for i, origin := range auth.WebAuthnRPOrigins {
		validateOrigin(&p, fmt.Sprintf("auth.webauthn_rp_origins[%d]", i), origin)
	}

	transcode := TranscodeConfig{HardwareAccel: strings.ToLower(strings.TrimSpace(*m.Transcode.HardwareAccel))}
	requireOneOf(&p, "transcode.hardware_accel", transcode.HardwareAccel, "auto", "vaapi", "nvenc", "qsv", "none")

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
func requirePort(p *[]string, name, value string) {
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > 65535 {
		*p = append(*p, name+" must be a port between 1 and 65535")
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
func requireHTTPURL(p *[]string, name, value string) {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		*p = append(*p, name+" must be an absolute http(s) URL")
	}
}
func validateOrigin(p *[]string, name, value string) {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		*p = append(*p, name+" must be an http(s) origin")
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
func resolveHost(base, value string) string {
	value = strings.TrimSpace(value)
	if filepath.IsAbs(value) || strings.HasPrefix(value, ".") || strings.ContainsAny(value, `/\`) {
		return resolvePath(base, value)
	}
	return value
}
func readRequiredSecret(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", errors.New("secret file is empty")
	}
	return secret, nil
}
func readOptionalSecret(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", true, errors.New("secret file is empty")
	}
	return secret, true, nil
}
