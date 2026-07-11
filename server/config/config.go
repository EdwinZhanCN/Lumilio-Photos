package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// LoadOptions describes every external input used to resolve AppConfig.
// Callers decide how to collect environment values; this package does not read
// process environment variables unless they are explicitly supplied here.
type LoadOptions struct {
	Environment       string
	ConfigFile        string
	RequireConfigFile bool
	Env               map[string]string
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host              string `toml:"host"`
	Port              string `toml:"port"`
	User              string `toml:"user"`
	Password          string `toml:"password"`
	PasswordFile      string `toml:"password_file"`
	DBName            string `toml:"name"`
	SSL               string `toml:"ssl"`
	BootstrapPassword string `toml:"-"`
	// ToolsBinDir optionally pins the directory holding version-matched
	// PostgreSQL client tools (pg_dump/psql) for the backup engine. Desktop
	// sets it to the bundled bin dir; empty means autodetect (Debian layout,
	// then PATH with a major-version check).
	ToolsBinDir string `toml:"tools_bin_dir"`
}

// AppConfig is the single typed runtime contract consumed by server/app. It
// holds only runtime-immutable boot configuration (TOML / supervisor-injected).
// Runtime-mutable settings (LLM, ML, repository behaviour) live in the database
// and are modelled by server/internal/settings.
type AppConfig struct {
	Environment    string `toml:"environment"`
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
}

type ServerConfig struct {
	Port               string   `toml:"port"`
	LogLevel           string   `toml:"log_level"`
	CORSAllowedOrigins []string `toml:"cors_allowed_origins"`
	WebRoot            string   `toml:"web_root"`
}

type LoggingConfig struct {
	Level                  string `toml:"level"`
	LogDir                 string `toml:"dir"`
	ConsoleFormat          string `toml:"console_format"`
	FileFormat             string `toml:"file_format"`
	RepositoryAuditVerbose string `toml:"repository_audit_verbose"`
}

// StorageConfig holds the immutable storage root. Every well-known subdirectory
// is derived from Path by convention, so locations are never runtime-settable;
// only repository behaviour (in server/internal/settings) is.
type StorageConfig struct {
	Path string `toml:"path"`
}

// Subdirectory names derived from the storage root. These are conventions, not
// configurable values.
const (
	secretsDirName = ".secrets"
	cloudDirName   = ".cloud"
	primaryDirName = "primary"
	backupsDirName = "backups"

	dbPasswordFileName = "db_password"
	secretKeyFileName  = "lumilio_secret_key"
)

// SecretsDir is <path>/.secrets, holding db_password and the app secret key.
func (c StorageConfig) SecretsDir() string {
	return filepath.Join(c.Path, secretsDirName)
}

// CloudDir is <path>/.cloud, the cloud sync working area.
func (c StorageConfig) CloudDir() string {
	return filepath.Join(c.Path, cloudDirName)
}

// PrimaryDir is <path>/primary, the primary repository's physical location.
func (c StorageConfig) PrimaryDir() string {
	return filepath.Join(c.Path, primaryDirName)
}

// BackupsDir is <path>/backups. Database dumps live with the media on purpose:
// backing up the storage root then captures assets and metadata together.
func (c StorageConfig) BackupsDir() string {
	return filepath.Join(c.Path, backupsDirName)
}

// DBPasswordPath is the rotated database password secret file.
func (c StorageConfig) DBPasswordPath() string {
	return filepath.Join(c.SecretsDir(), dbPasswordFileName)
}

// SecretKeyPath is the app secret key file.
func (c StorageConfig) SecretKeyPath() string {
	return filepath.Join(c.SecretsDir(), secretKeyFileName)
}

type RepositoryScanConfig struct {
	Enabled            bool `toml:"enabled"`
	IntervalSeconds    int  `toml:"interval_seconds"`
	SettleSeconds      int  `toml:"settle_seconds"`
	MaxConcurrentRepos int  `toml:"max_concurrent_repos"`
	BatchSize          int  `toml:"batch_size"`
}

type GeocodingConfig struct {
	Provider          string `toml:"provider"`
	NominatimEndpoint string `toml:"nominatim_endpoint"`
	Language          string `toml:"language"`
	UserAgent         string `toml:"user_agent"`
}

type AuthConfig struct {
	SecretKeyPath     string   `toml:"secret_key_path"`
	AccessTokenTTL    string   `toml:"access_token_ttl"`
	RefreshTokenTTL   string   `toml:"refresh_token_ttl"`
	MediaTokenTTL     string   `toml:"media_token_ttl"`
	WebAuthnRPName    string   `toml:"webauthn_rp_name"`
	WebAuthnRPID      string   `toml:"webauthn_rp_id"`
	WebAuthnRPOrigins []string `toml:"webauthn_rp_origins"`
}

type TranscodeConfig struct {
	HardwareAccel string `toml:"hardware_accel"`
}

type LumenConfig struct {
	DiscoveryEnabled     bool   `toml:"discovery_enabled"`
	DiscoveryMDNSEnabled bool   `toml:"discovery_mdns_enabled"`
	DiscoveryHubURL      string `toml:"discovery_hub_url"`
	// DiscoveryStaticNodes pins Lumen node gRPC endpoints ("host:port") that
	// are used without dynamic discovery. Backends are additive: mDNS, the
	// gateway hub URL, and static nodes all run when configured.
	DiscoveryStaticNodes []string `toml:"discovery_static_nodes"`
}

// StaticNodes returns the configured static node endpoints with blank entries
// removed.
func (c LumenConfig) StaticNodes() []string {
	nodes := make([]string, 0, len(c.DiscoveryStaticNodes))
	for _, node := range c.DiscoveryStaticNodes {
		if node = strings.TrimSpace(node); node != "" {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// Enabled reports whether the Lumen ML integration is active: discovery is on
// and at least one discovery backend (mDNS, a gateway hub URL, or static
// nodes) is configured. When false the server boots with ML features disabled
// instead of failing startup.
func (c LumenConfig) Enabled() bool {
	return c.DiscoveryEnabled &&
		(c.DiscoveryMDNSEnabled || strings.TrimSpace(c.DiscoveryHubURL) != "" || len(c.StaticNodes()) > 0)
}

type tomlConfig struct {
	Environment    string               `toml:"environment"`
	DatabaseConfig DatabaseConfig       `toml:"database"`
	ServerConfig   ServerConfig         `toml:"server"`
	LoggingConfig  LoggingConfig        `toml:"logging"`
	StorageConfig  StorageConfig        `toml:"storage"`
	RepositoryScan RepositoryScanConfig `toml:"repository_scan"`
	Geocoding      GeocodingConfig      `toml:"geocoding"`
	Auth           AuthConfig           `toml:"auth"`
	Transcode      TranscodeConfig      `toml:"transcode"`
	Lumen          LumenConfig          `toml:"lumen"`
	Tools          ToolsConfig          `toml:"tools"`
}

// ProcessEnv returns a snapshot of the current process environment suitable for
// LoadOptions.Env. Keeping this at the process boundary makes tests and desktop
// hosts independent from ambient state.
func ProcessEnv() map[string]string {
	env := make(map[string]string, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func LoadAppConfigWithOptions(opts LoadOptions) (AppConfig, error) {
	env := envMap(opts.Env)
	environment := firstNonEmpty(env.get("SERVER_ENV"), opts.Environment)
	cfg := defaultAppConfigForEnvironment(environment)

	if err := loadTOMLConfig(&cfg, opts); err != nil {
		return AppConfig{}, err
	}
	applyEnvOverrides(&cfg, env)
	deriveConfigPaths(&cfg)
	applyDBPasswordFile(&cfg)
	if err := validateAppConfig(cfg); err != nil {
		return AppConfig{}, err
	}
	return cfg, nil
}

func defaultAppConfigForEnvironment(environment string) AppConfig {
	environment = strings.ToLower(strings.TrimSpace(environment))
	if environment == "" {
		environment = "production"
	}

	dbHost := "db"
	logLevel := "info"
	logDir := "server/logs"
	transcodeAccel := "none"
	lumenMDNS := false
	if environment == "development" {
		dbHost = "localhost"
		logLevel = "debug"
		logDir = "logs"
		transcodeAccel = "auto"
		lumenMDNS = true
	}

	return AppConfig{
		Environment: environment,
		DatabaseConfig: DatabaseConfig{
			Host:         dbHost,
			Port:         "5432",
			User:         "postgres",
			Password:     "postgres",
			PasswordFile: defaultDBPasswordFilePath(),
			DBName:       "lumiliophotos",
			SSL:          "disable",
		},
		ServerConfig: ServerConfig{
			Port:     "8080",
			LogLevel: logLevel,
			CORSAllowedOrigins: []string{
				"http://localhost:6657",
				"https://localhost:6657",
			},
		},
		LoggingConfig: LoggingConfig{
			Level:         logLevel,
			LogDir:        logDir,
			ConsoleFormat: "console",
			FileFormat:    "json",
		},
		StorageConfig: StorageConfig{
			Path: "data/storage",
		},
		RepositoryScan: RepositoryScanConfig{
			Enabled:            true,
			IntervalSeconds:    300,
			SettleSeconds:      5,
			MaxConcurrentRepos: 1,
			BatchSize:          500,
		},
		Geocoding: GeocodingConfig{
			Provider:          "disabled",
			NominatimEndpoint: "https://nominatim.openstreetmap.org/reverse",
			Language:          "en",
			UserAgent:         "Lumilio-Photos/1.0",
		},
		Auth: AuthConfig{
			AccessTokenTTL:  "15m",
			RefreshTokenTTL: "168h",
			MediaTokenTTL:   "10m",
			WebAuthnRPName:  "Lumilio Photos",
		},
		Transcode: TranscodeConfig{
			HardwareAccel: transcodeAccel,
		},
		Lumen: LumenConfig{
			DiscoveryEnabled:     true,
			DiscoveryMDNSEnabled: lumenMDNS,
		},
	}
}

func defaultDBPasswordFilePath() string {
	return StorageConfig{Path: filepath.Join("data", "storage")}.DBPasswordPath()
}

func loadTOMLConfig(cfg *AppConfig, opts LoadOptions) error {
	path, explicit := resolveConfigFile(opts)
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if explicit || opts.RequireConfigFile || !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("load server config %s: %w", path, err)
		}
		return nil
	}

	fileCfg := tomlConfigFromAppConfig(*cfg)
	if err := toml.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parse server config %s: %w", path, err)
	}

	*cfg = appConfigFromTOMLConfig(fileCfg)
	cfg.Environment = strings.ToLower(strings.TrimSpace(cfg.Environment))
	if cfg.Environment == "" {
		cfg.Environment = strings.ToLower(strings.TrimSpace(firstNonEmpty(opts.Environment, "production")))
	}
	return nil
}

func resolveConfigFile(opts LoadOptions) (string, bool) {
	if path := strings.TrimSpace(opts.ConfigFile); path != "" {
		return path, true
	}
	candidates := []string{
		filepath.Join("config", "server.local.toml"),
		filepath.Join("server", "config", "server.local.toml"),
		filepath.Join("/app", "config", "server.local.toml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, false
		}
	}
	if opts.RequireConfigFile {
		return candidates[0], false
	}
	return "", false
}

type envMap map[string]string

func (e envMap) get(key string) string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e[key])
}

func (e envMap) bool(key string) (bool, bool) {
	raw := strings.ToLower(e.get(key))
	switch raw {
	case "true", "1", "yes", "on":
		return true, true
	case "false", "0", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func (e envMap) positiveInt(key string) (int, bool) {
	raw := e.get(key)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func applyEnvOverrides(cfg *AppConfig, env envMap) {
	if value := env.get("SERVER_ENV"); value != "" {
		cfg.Environment = strings.ToLower(value)
	}
	if value := env.get("SERVER_PORT"); value != "" {
		cfg.ServerConfig.Port = value
	}
	if value := env.get("SERVER_LOG_LEVEL"); value != "" {
		level := strings.ToLower(value)
		cfg.ServerConfig.LogLevel = level
		cfg.LoggingConfig.Level = level
	}
	if value := env.get("SERVER_CORS_ALLOWED_ORIGINS"); value != "" {
		cfg.ServerConfig.CORSAllowedOrigins = splitCSV(value)
	}
	if value := env.get("SERVER_WEB_ROOT"); value != "" {
		cfg.ServerConfig.WebRoot = value
	}

	if value := env.get("LOG_LEVEL"); value != "" {
		cfg.LoggingConfig.Level = strings.ToLower(value)
	}
	if value := env.get("LOG_DIR"); value != "" {
		cfg.LoggingConfig.LogDir = value
	}
	if value := env.get("LOG_FORMAT_CONSOLE"); value != "" {
		cfg.LoggingConfig.ConsoleFormat = strings.ToLower(value)
	}
	if value := env.get("LOG_FORMAT_FILE"); value != "" {
		cfg.LoggingConfig.FileFormat = strings.ToLower(value)
	}
	if value := env.get("REPO_AUDIT_VERBOSE"); value != "" {
		cfg.LoggingConfig.RepositoryAuditVerbose = value
	}

	if value := env.get("DB_HOST"); value != "" {
		cfg.DatabaseConfig.Host = value
	}
	if value := env.get("DB_PORT"); value != "" {
		cfg.DatabaseConfig.Port = value
	}
	if value := env.get("DB_USER"); value != "" {
		cfg.DatabaseConfig.User = value
	}
	if value := env.get("DB_PASSWORD"); value != "" {
		cfg.DatabaseConfig.Password = value
	}
	if value := firstNonEmpty(env.get("LUMILIO_DB_PASSWORD_FILE"), env.get("DB_PASSWORD_FILE")); value != "" {
		cfg.DatabaseConfig.PasswordFile = value
	}
	if value := env.get("DB_NAME"); value != "" {
		cfg.DatabaseConfig.DBName = value
	}
	if value := env.get("DB_SSL"); value != "" {
		cfg.DatabaseConfig.SSL = value
	}

	if value := env.get("STORAGE_PATH"); value != "" {
		cfg.StorageConfig.Path = value
	}

	if value, ok := env.bool("REPOSITORY_SCAN_ENABLED"); ok {
		cfg.RepositoryScan.Enabled = value
	}
	if value, ok := env.positiveInt("REPOSITORY_SCAN_INTERVAL_SECONDS"); ok {
		cfg.RepositoryScan.IntervalSeconds = value
	}
	if value, ok := env.positiveInt("REPOSITORY_SCAN_SETTLE_SECONDS"); ok {
		cfg.RepositoryScan.SettleSeconds = value
	}
	if value, ok := env.positiveInt("REPOSITORY_SCAN_MAX_CONCURRENT_REPOS"); ok {
		cfg.RepositoryScan.MaxConcurrentRepos = value
	}
	if value, ok := env.positiveInt("REPOSITORY_SCAN_BATCH_SIZE"); ok {
		cfg.RepositoryScan.BatchSize = value
	}

	if value := env.get("GEOCODING_PROVIDER"); value != "" {
		cfg.Geocoding.Provider = strings.ToLower(value)
	}
	if value := env.get("GEOCODING_NOMINATIM_ENDPOINT"); value != "" {
		cfg.Geocoding.NominatimEndpoint = value
	}
	if value := env.get("GEOCODING_LANGUAGE"); value != "" {
		cfg.Geocoding.Language = value
	}
	if value := env.get("GEOCODING_USER_AGENT"); value != "" {
		cfg.Geocoding.UserAgent = value
	}

	if value := env.get("LUMILIO_SECRET_KEY"); value != "" {
		cfg.Auth.SecretKeyPath = value
	}
	if value := env.get("ACCESS_TOKEN_TTL"); value != "" {
		cfg.Auth.AccessTokenTTL = value
	}
	if value := env.get("REFRESH_TOKEN_TTL"); value != "" {
		cfg.Auth.RefreshTokenTTL = value
	}
	if value := env.get("MEDIA_TOKEN_TTL"); value != "" {
		cfg.Auth.MediaTokenTTL = value
	}
	if value := env.get("WEBAUTHN_RP_NAME"); value != "" {
		cfg.Auth.WebAuthnRPName = value
	}
	if value := env.get("WEBAUTHN_RP_ID"); value != "" {
		cfg.Auth.WebAuthnRPID = value
	}
	if value := env.get("WEBAUTHN_RP_ORIGINS"); value != "" {
		cfg.Auth.WebAuthnRPOrigins = splitCSV(value)
	}

	if value := env.get("TRANSCODE_HW_ACCEL"); value != "" {
		cfg.Transcode.HardwareAccel = strings.ToLower(value)
	}

	if value := env.get("EXIFTOOL_PATH"); value != "" {
		cfg.Tools.ExifToolPath = value
	}
	if value := env.get("FFMPEG_PATH"); value != "" {
		cfg.Tools.FFmpegPath = value
	}
	if value := env.get("FFPROBE_PATH"); value != "" {
		cfg.Tools.FFprobePath = value
	}

	if value, ok := env.bool("LUMEN_DISCOVERY_ENABLED"); ok {
		cfg.Lumen.DiscoveryEnabled = value
	}
	if value, ok := env.bool("LUMEN_DISCOVERY_MDNS_ENABLED"); ok {
		cfg.Lumen.DiscoveryMDNSEnabled = value
	}
	if value := env.get("LUMEN_DISCOVERY_HUB_URL"); value != "" {
		cfg.Lumen.DiscoveryHubURL = value
	}
	if value := env.get("LUMEN_DISCOVERY_STATIC_NODES"); value != "" {
		var nodes []string
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				nodes = append(nodes, part)
			}
		}
		cfg.Lumen.DiscoveryStaticNodes = nodes
	}
}

func applyDBPasswordFile(cfg *AppConfig) {
	if strings.TrimSpace(cfg.DatabaseConfig.PasswordFile) == "" {
		cfg.DatabaseConfig.PasswordFile = defaultDBPasswordFilePath()
	}
	cfg.DatabaseConfig.BootstrapPassword = cfg.DatabaseConfig.Password
	data, err := os.ReadFile(cfg.DatabaseConfig.PasswordFile)
	if err != nil {
		return
	}
	if password := strings.TrimSpace(string(data)); password != "" {
		cfg.DatabaseConfig.Password = password
	}
}

func deriveConfigPaths(cfg *AppConfig) {
	if strings.TrimSpace(cfg.Auth.SecretKeyPath) == "" {
		cfg.Auth.SecretKeyPath = cfg.StorageConfig.SecretKeyPath()
	}
}

func validateAppConfig(cfg AppConfig) error {
	var problems []string

	requireOneOf(&problems, "server.log_level", cfg.ServerConfig.LogLevel, "debug", "info", "warn", "error")
	requireOneOf(&problems, "logging.level", cfg.LoggingConfig.Level, "debug", "info", "warn", "error")
	requireOneOf(&problems, "logging.console_format", cfg.LoggingConfig.ConsoleFormat, "console", "json")
	requireOneOf(&problems, "logging.file_format", cfg.LoggingConfig.FileFormat, "console", "json")
	requireOneOf(&problems, "geocoding.provider", cfg.Geocoding.Provider, "disabled", "nominatim")
	requireOneOf(&problems, "transcode.hardware_accel", cfg.Transcode.HardwareAccel, "auto", "vaapi", "nvenc", "qsv", "none")

	if cfg.ServerConfig.Port == "" {
		problems = append(problems, "server.port is required")
	}
	if cfg.DatabaseConfig.Host == "" {
		problems = append(problems, "database.host is required")
	}
	if cfg.DatabaseConfig.Port == "" {
		problems = append(problems, "database.port is required")
	}
	if cfg.DatabaseConfig.User == "" {
		problems = append(problems, "database.user is required")
	}
	if cfg.DatabaseConfig.DBName == "" {
		problems = append(problems, "database.name is required")
	}
	if cfg.StorageConfig.Path == "" {
		problems = append(problems, "storage.path is required")
	}
	if cfg.RepositoryScan.IntervalSeconds <= 0 {
		problems = append(problems, "repository_scan.interval_seconds must be positive")
	}
	if cfg.RepositoryScan.SettleSeconds <= 0 {
		problems = append(problems, "repository_scan.settle_seconds must be positive")
	}
	if cfg.RepositoryScan.MaxConcurrentRepos <= 0 {
		problems = append(problems, "repository_scan.max_concurrent_repos must be positive")
	}
	if cfg.RepositoryScan.BatchSize <= 0 {
		problems = append(problems, "repository_scan.batch_size must be positive")
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid server config: %s", strings.Join(problems, "; "))
	}
	return nil
}

func requireOneOf(problems *[]string, field string, value string, allowed ...string) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if normalized == candidate {
			return
		}
	}
	*problems = append(*problems, fmt.Sprintf("%s must be one of %s", field, strings.Join(allowed, ", ")))
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func appConfigFromTOMLConfig(cfg tomlConfig) AppConfig {
	return AppConfig{
		Environment:    cfg.Environment,
		DatabaseConfig: cfg.DatabaseConfig,
		ServerConfig:   cfg.ServerConfig,
		LoggingConfig:  cfg.LoggingConfig,
		StorageConfig:  cfg.StorageConfig,
		RepositoryScan: cfg.RepositoryScan,
		Geocoding:      cfg.Geocoding,
		Auth:           cfg.Auth,
		Transcode:      cfg.Transcode,
		Lumen:          cfg.Lumen,
		Tools:          cfg.Tools,
	}
}
