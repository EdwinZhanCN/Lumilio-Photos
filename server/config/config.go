package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/pelletier/go-toml/v2"
)

// DatabaseConfig holds all the configuration for the database connection.
type DatabaseConfig struct {
	Host         string `toml:"host"`
	Port         string `toml:"port"`
	User         string `toml:"user"`
	Password     string `toml:"password"`
	PasswordFile string `toml:"password_file"`
	DBName       string `toml:"name"`
	SSL          string `toml:"ssl"`
}

// AppConfig holds general application configuration
type AppConfig struct {
	Environment    string `toml:"environment"`
	DatabaseConfig DatabaseConfig
	ServerConfig   ServerConfig
	LoggingConfig  LoggingConfig
	StorageConfig  StorageConfig
	LLMConfig      LLMConfig
	MLConfig       MLConfig
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
	// WebRoot, when set, makes the API server also serve the SPA bundle from
	// this directory (with index.html fallback). Empty = API-only, which is the
	// docker/web default (a separate static server hosts the bundle there). The
	// desktop build points this at the bundled web build so localhost:6680 serves
	// the full app. Env override: SERVER_WEB_ROOT.
	WebRoot string `toml:"web_root"`
}

type LoggingConfig struct {
	Level                  string `toml:"level"`
	LogDir                 string `toml:"dir"`
	ConsoleFormat          string `toml:"console_format"`
	FileFormat             string `toml:"file_format"`
	RepositoryAuditVerbose string `toml:"repository_audit_verbose"`
}

type StorageConfig struct {
	Path              string `toml:"path"`
	Strategy          string `toml:"strategy"`
	PreserveFilename  bool   `toml:"preserve_filename"`
	DuplicateHandling string `toml:"duplicate_handling"`
}

type LLMConfig struct {
	AgentEnabled bool   `toml:"agent_enabled"`
	Provider     string `toml:"provider"`
	APIKey       string `toml:"api_key"`
	ModelName    string `toml:"model_name"`
	BaseURL      string `toml:"base_url"`
}

func (c LLMConfig) EffectiveProvider() string {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider == "" {
		return "ark"
	}
	return provider
}

func (c LLMConfig) IsConfigured() bool {
	modelName := strings.TrimSpace(c.ModelName)
	if modelName == "" {
		return false
	}

	switch c.EffectiveProvider() {
	case "ollama":
		return strings.TrimSpace(c.BaseURL) != ""
	default:
		return strings.TrimSpace(c.APIKey) != ""
	}
}

type MLConfig struct {
	SemanticEnabled         bool `toml:"semantic_enabled"`
	BioCLIPEnabled          bool `toml:"bioclip_enabled"`
	OCREnabled              bool `toml:"ocr_enabled"`
	FaceEnabled             bool `toml:"face_enabled"`
	ZeroshotClassifyEnabled bool `toml:"zeroshot_classify_enabled"`
}

func (c MLConfig) HasManualTasksEnabled() bool {
	return c.SemanticEnabled || c.BioCLIPEnabled || c.OCREnabled || c.FaceEnabled || c.ZeroshotClassifyEnabled
}

func (c MLConfig) HasRuntimeDemand() bool {
	return c.HasManualTasksEnabled()
}

// RepositoryScanConfig controls periodic repository free-workspace scanning.
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

// TranscodeConfig controls hardware acceleration for video transcoding.
type TranscodeConfig struct {
	// HardwareAccel: auto, vaapi, nvenc, qsv, none
	HardwareAccel string `toml:"hardware_accel"`
}

type LumenConfig struct {
	DiscoveryEnabled     bool   `toml:"discovery_enabled"`
	DiscoveryMDNSEnabled bool   `toml:"discovery_mdns_enabled"`
	DiscoveryHubURL      string `toml:"discovery_hub_url"`
	ConnectionInsecure   bool   `toml:"connection_insecure"`
}

type tomlConfig struct {
	Environment    string               `toml:"environment"`
	DatabaseConfig DatabaseConfig       `toml:"database"`
	ServerConfig   ServerConfig         `toml:"server"`
	LoggingConfig  LoggingConfig        `toml:"logging"`
	StorageConfig  StorageConfig        `toml:"storage"`
	RepositoryScan RepositoryScanConfig `toml:"repository_scan"`
	Geocoding      GeocodingConfig      `toml:"geocoding"`
	MLConfig       MLConfig             `toml:"ml"`
	LLMConfig      LLMConfig            `toml:"llm"`
	Auth           AuthConfig           `toml:"auth"`
	Transcode      TranscodeConfig      `toml:"transcode"`
	Lumen          LumenConfig          `toml:"lumen"`
	Tools          ToolsConfig          `toml:"tools"`
}

// LoadTranscodeConfig returns transcoding configuration.
func LoadTranscodeConfig() TranscodeConfig {
	cfg := TranscodeConfig{
		HardwareAccel: "none",
	}
	if v := strings.ToLower(strings.TrimSpace(os.Getenv("TRANSCODE_HW_ACCEL"))); v != "" {
		cfg.HardwareAccel = v
	}
	if cfg.HardwareAccel == "auto" {
		cfg.HardwareAccel = detectHardwareAccel()
	}
	return cfg
}

func detectHardwareAccel() string {
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		return "vaapi"
	}
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return "nvenc"
	}
	return "none"
}

// IsDevelopmentMode checks if the application is running in development mode
func IsDevelopmentMode() bool {
	return strings.ToLower(os.Getenv("SERVER_ENV")) == "development"
}

// LoadEnvironment loads environment variables from appropriate .env file
// This function should be called in the init() function of both API and Worker main.go files
// It automatically loads .env.development in development mode, .env otherwise
func LoadEnvironment() {
	if IsDevelopmentMode() {
		_ = godotenv.Load(".env.development")
		return
	}

	_ = godotenv.Load(".env")
	if IsDevelopmentMode() {
		_ = godotenv.Load(".env.development")
	}
}

// LoadDBConfig loads database settings from environment variables
// Used by both API and Worker services for consistent database configuration
func LoadDBConfig() DatabaseConfig {
	cfg, _ := LoadAppConfigWithError()
	return cfg.DatabaseConfig
}

// LoadAppConfig loads general application configuration
func LoadAppConfig() AppConfig {
	cfg, _ := LoadAppConfigWithError()
	return cfg
}

func LoadAppConfigWithError() (AppConfig, error) {
	cfg := defaultAppConfig()
	if err := loadTOMLConfig(&cfg); err != nil {
		return AppConfig{}, err
	}
	applyEnvOverrides(&cfg)
	applyDBPasswordFile(&cfg)
	return cfg, nil
}

// DBPasswordFilePath returns the default path to the rotated database password
// secret. Prefer ResolveDBPasswordFilePath when a DatabaseConfig is available.
func DBPasswordFilePath() string {
	if v := strings.TrimSpace(os.Getenv("LUMILIO_DB_PASSWORD_FILE")); v != "" {
		return v
	}
	return filepath.Join("data", "storage", ".secrets", "db_password")
}

// ResolveDBPasswordFilePath returns the configured rotated database password
// path. First-run setup writes the high-entropy password here; on subsequent
// boots this file is the authoritative credential, superseding the temporary
// bootstrap password supplied through env or TOML.
func ResolveDBPasswordFilePath(dbConfig DatabaseConfig) string {
	if v := strings.TrimSpace(dbConfig.PasswordFile); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("LUMILIO_DB_PASSWORD_FILE")); v != "" {
		return v
	}
	return DBPasswordFilePath()
}

// applyDBPasswordFile prefers the rotated database password persisted on disk
// over the temporary bootstrap password. Missing or empty files are ignored so
// first boot can connect with the env-provided temporary credential.
func applyDBPasswordFile(cfg *AppConfig) {
	data, err := os.ReadFile(ResolveDBPasswordFilePath(cfg.DatabaseConfig))
	if err != nil {
		return
	}
	if password := strings.TrimSpace(string(data)); password != "" {
		cfg.DatabaseConfig.Password = password
	}
}

func LoadServerConfig() ServerConfig {
	cfg, _ := LoadAppConfigWithError()
	return cfg.ServerConfig
}

// LoadMLConfig loads ML/AI service configuration from environment variables
func LoadMLConfig() MLConfig {
	cfg, _ := LoadAppConfigWithError()
	return cfg.MLConfig
}

// LoadLLMConfig loads LLM settings such as API key and model name from environment variables
func LoadLLMConfig() LLMConfig {
	cfg, _ := LoadAppConfigWithError()
	return cfg.LLMConfig
}

// LoadRepositoryScanConfig loads file tree scan settings.
func LoadRepositoryScanConfig() RepositoryScanConfig {
	cfg, _ := LoadAppConfigWithError()
	return cfg.RepositoryScan
}

func defaultAppConfig() AppConfig {
	return defaultAppConfigForEnvironment(os.Getenv("SERVER_ENV"))
}

func defaultAppConfigForEnvironment(environment string) AppConfig {
	environment = strings.ToLower(strings.TrimSpace(environment))
	if environment == "" {
		environment = "production"
	}
	dbHost := "db"
	logLevel := "info"
	mlDefaults := MLConfig{SemanticEnabled: true, BioCLIPEnabled: true, OCREnabled: true, FaceEnabled: true}
	if environment == "development" {
		dbHost = "localhost"
		logLevel = "debug"
		mlDefaults = MLConfig{}
	}

	return AppConfig{
		Environment: environment,
		DatabaseConfig: DatabaseConfig{
			Host:         dbHost,
			Port:         "5432",
			User:         "postgres",
			Password:     "postgres",
			PasswordFile: DBPasswordFilePath(),
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
			LogDir:        "server/logs",
			ConsoleFormat: "console",
			FileFormat:    "json",
		},
		StorageConfig: StorageConfig{
			Path:              "data/storage",
			Strategy:          "date",
			PreserveFilename:  true,
			DuplicateHandling: "rename",
		},
		RepositoryScan: RepositoryScanConfig{
			Enabled:            true,
			IntervalSeconds:    300,
			SettleSeconds:      5,
			MaxConcurrentRepos: 1,
			BatchSize:          500,
		},
		Geocoding: GeocodingConfig{
			Provider:  "disabled",
			Language:  "en",
			UserAgent: "Lumilio-Photos/1.0",
		},
		MLConfig: mlDefaults,
		Auth: AuthConfig{
			AccessTokenTTL:  "15m",
			RefreshTokenTTL: "168h",
			MediaTokenTTL:   "10m",
		},
		Transcode: TranscodeConfig{
			HardwareAccel: "none",
		},
		Lumen: LumenConfig{
			DiscoveryEnabled:     true,
			DiscoveryMDNSEnabled: false,
			ConnectionInsecure:   true,
		},
	}
}

func loadTOMLConfig(cfg *AppConfig) error {
	path, explicit := configFilePath(cfg.Environment)
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if explicit || !os.IsNotExist(err) {
			return fmt.Errorf("load server config %s: %w", path, err)
		}
		return nil
	}

	var fileCfg tomlConfig
	fileCfg.Environment = cfg.Environment
	fileCfg.DatabaseConfig = cfg.DatabaseConfig
	fileCfg.ServerConfig = cfg.ServerConfig
	fileCfg.LoggingConfig = cfg.LoggingConfig
	fileCfg.StorageConfig = cfg.StorageConfig
	fileCfg.RepositoryScan = cfg.RepositoryScan
	fileCfg.Geocoding = cfg.Geocoding
	fileCfg.MLConfig = cfg.MLConfig
	fileCfg.LLMConfig = cfg.LLMConfig
	fileCfg.Auth = cfg.Auth
	fileCfg.Transcode = cfg.Transcode
	fileCfg.Lumen = cfg.Lumen
	fileCfg.Tools = cfg.Tools

	if err := toml.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parse server config %s: %w", path, err)
	}

	cfg.Environment = strings.ToLower(strings.TrimSpace(fileCfg.Environment))
	cfg.DatabaseConfig = fileCfg.DatabaseConfig
	cfg.ServerConfig = fileCfg.ServerConfig
	cfg.LoggingConfig = fileCfg.LoggingConfig
	cfg.StorageConfig = fileCfg.StorageConfig
	cfg.RepositoryScan = fileCfg.RepositoryScan
	cfg.Geocoding = fileCfg.Geocoding
	cfg.MLConfig = fileCfg.MLConfig
	cfg.LLMConfig = fileCfg.LLMConfig
	cfg.Auth = fileCfg.Auth
	cfg.Transcode = fileCfg.Transcode
	cfg.Lumen = fileCfg.Lumen
	cfg.Tools = fileCfg.Tools

	return nil
}

func configFilePath(environment string) (string, bool) {
	if explicit := strings.TrimSpace(os.Getenv("SERVER_CONFIG_FILE")); explicit != "" {
		return explicit, true
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
	return "", false
}

func applyEnvOverrides(cfg *AppConfig) {
	if value := envString("SERVER_ENV"); value != "" {
		cfg.Environment = strings.ToLower(value)
	}
	if value := envString("SERVER_PORT"); value != "" {
		cfg.ServerConfig.Port = value
	}
	if value := envString("SERVER_LOG_LEVEL"); value != "" {
		cfg.ServerConfig.LogLevel = value
		cfg.LoggingConfig.Level = value
	}
	if value := envString("SERVER_CORS_ALLOWED_ORIGINS"); value != "" {
		cfg.ServerConfig.CORSAllowedOrigins = splitCSV(value)
	}
	if value := envString("SERVER_WEB_ROOT"); value != "" {
		cfg.ServerConfig.WebRoot = value
	}

	if value := envString("LOG_LEVEL"); value != "" {
		cfg.LoggingConfig.Level = value
	}
	if value := envString("LOG_DIR"); value != "" {
		cfg.LoggingConfig.LogDir = value
	}
	if value := envString("LOG_FORMAT_CONSOLE"); value != "" {
		cfg.LoggingConfig.ConsoleFormat = strings.ToLower(value)
	}
	if value := envString("LOG_FORMAT_FILE"); value != "" {
		cfg.LoggingConfig.FileFormat = strings.ToLower(value)
	}

	if value := envString("DB_HOST"); value != "" {
		cfg.DatabaseConfig.Host = value
	}
	if value := envString("DB_PORT"); value != "" {
		cfg.DatabaseConfig.Port = value
	}
	if value := envString("DB_USER"); value != "" {
		cfg.DatabaseConfig.User = value
	}
	if value := envString("DB_PASSWORD"); value != "" {
		cfg.DatabaseConfig.Password = value
	}
	if value := envString("DB_PASSWORD_FILE"); value != "" {
		cfg.DatabaseConfig.PasswordFile = value
	}
	if value := envString("LUMILIO_DB_PASSWORD_FILE"); value != "" {
		cfg.DatabaseConfig.PasswordFile = value
	}
	if value := envString("DB_NAME"); value != "" {
		cfg.DatabaseConfig.DBName = value
	}
	if value := envString("DB_SSL"); value != "" {
		cfg.DatabaseConfig.SSL = value
	}

	if value := envString("STORAGE_PATH"); value != "" {
		cfg.StorageConfig.Path = value
	}
	if value := envString("STORAGE_STRATEGY"); value != "" {
		cfg.StorageConfig.Strategy = value
	}
	if value, ok := envBool("STORAGE_PRESERVE_FILENAME"); ok {
		cfg.StorageConfig.PreserveFilename = value
	}
	if value := envString("STORAGE_DUPLICATE_HANDLING"); value != "" {
		cfg.StorageConfig.DuplicateHandling = value
	}

	if value, ok := envBool("REPOSITORY_SCAN_ENABLED"); ok {
		cfg.RepositoryScan.Enabled = value
	}
	if value, ok := envPositiveInt("REPOSITORY_SCAN_INTERVAL_SECONDS"); ok {
		cfg.RepositoryScan.IntervalSeconds = value
	}
	if value, ok := envPositiveInt("REPOSITORY_SCAN_SETTLE_SECONDS"); ok {
		cfg.RepositoryScan.SettleSeconds = value
	}
	if value, ok := envPositiveInt("REPOSITORY_SCAN_MAX_CONCURRENT_REPOS"); ok {
		cfg.RepositoryScan.MaxConcurrentRepos = value
	}
	if value, ok := envPositiveInt("REPOSITORY_SCAN_BATCH_SIZE"); ok {
		cfg.RepositoryScan.BatchSize = value
	}

	if value := envString("GEOCODING_PROVIDER"); value != "" {
		cfg.Geocoding.Provider = value
	}
	if value := envString("GEOCODING_NOMINATIM_ENDPOINT"); value != "" {
		cfg.Geocoding.NominatimEndpoint = value
	}
	if value := envString("GEOCODING_LANGUAGE"); value != "" {
		cfg.Geocoding.Language = value
	}
	if value := envString("GEOCODING_USER_AGENT"); value != "" {
		cfg.Geocoding.UserAgent = value
	}

	if value, ok := envBool("ML_SEMANTIC_ENABLED"); ok {
		cfg.MLConfig.SemanticEnabled = value
	}
	if value, ok := envBool("ML_BIOCLIP_ENABLED"); ok {
		cfg.MLConfig.BioCLIPEnabled = value
	}
	if value, ok := envBool("ML_OCR_ENABLED"); ok {
		cfg.MLConfig.OCREnabled = value
	}
	if value, ok := envBool("ML_FACE_ENABLED"); ok {
		cfg.MLConfig.FaceEnabled = value
	}
	if value, ok := envBool("ML_ZEROSHOT_CLASSIFY_ENABLED"); ok {
		cfg.MLConfig.ZeroshotClassifyEnabled = value
	}

	if value, ok := envBool("LLM_AGENT_ENABLED"); ok {
		cfg.LLMConfig.AgentEnabled = value
	}
	if value := envString("LLM_PROVIDER"); value != "" {
		cfg.LLMConfig.Provider = value
	}
	if value := envString("LLM_API_KEY"); value != "" {
		cfg.LLMConfig.APIKey = value
	}
	if value := envString("LLM_MODEL_NAME"); value != "" {
		cfg.LLMConfig.ModelName = value
	}
	if value := envString("LLM_BASE_URL"); value != "" {
		cfg.LLMConfig.BaseURL = value
	}

	if value := envString("LUMILIO_SECRET_KEY"); value != "" {
		cfg.Auth.SecretKeyPath = value
	}
	if value := envString("ACCESS_TOKEN_TTL"); value != "" {
		cfg.Auth.AccessTokenTTL = value
	}
	if value := envString("REFRESH_TOKEN_TTL"); value != "" {
		cfg.Auth.RefreshTokenTTL = value
	}
	if value := envString("MEDIA_TOKEN_TTL"); value != "" {
		cfg.Auth.MediaTokenTTL = value
	}
	if value := envString("WEBAUTHN_RP_NAME"); value != "" {
		cfg.Auth.WebAuthnRPName = value
	}
	if value := envString("WEBAUTHN_RP_ID"); value != "" {
		cfg.Auth.WebAuthnRPID = value
	}
	if value := envString("WEBAUTHN_RP_ORIGINS"); value != "" {
		cfg.Auth.WebAuthnRPOrigins = splitCSV(value)
	}
	if value := envString("REPO_AUDIT_VERBOSE"); value != "" {
		cfg.LoggingConfig.RepositoryAuditVerbose = value
	}

	if value := envString("TRANSCODE_HW_ACCEL"); value != "" {
		cfg.Transcode.HardwareAccel = strings.ToLower(value)
	}

	if value := envString("EXIFTOOL_PATH"); value != "" {
		cfg.Tools.ExifToolPath = value
	}
	if value := envString("FFMPEG_PATH"); value != "" {
		cfg.Tools.FFmpegPath = value
	}
	if value := envString("FFPROBE_PATH"); value != "" {
		cfg.Tools.FFprobePath = value
	}

	if value, ok := envBool("LUMEN_DISCOVERY_ENABLED"); ok {
		cfg.Lumen.DiscoveryEnabled = value
	}
	if value, ok := envBool("LUMEN_DISCOVERY_MDNS_ENABLED"); ok {
		cfg.Lumen.DiscoveryMDNSEnabled = value
	}
	if value := envString("LUMEN_DISCOVERY_HUB_URL"); value != "" {
		cfg.Lumen.DiscoveryHubURL = value
	}
	if value, ok := envBool("LUMEN_CONNECTION_INSECURE"); ok {
		cfg.Lumen.ConnectionInsecure = value
	}
}

func ApplyRuntimeEnvDefaults(cfg AppConfig) {
	setEnvDefault("SERVER_ENV", cfg.Environment)
	setEnvDefault("SERVER_PORT", cfg.ServerConfig.Port)
	setEnvDefault("SERVER_LOG_LEVEL", cfg.ServerConfig.LogLevel)
	setEnvDefault("SERVER_CORS_ALLOWED_ORIGINS", strings.Join(cfg.ServerConfig.CORSAllowedOrigins, ","))
	setEnvDefault("SERVER_WEB_ROOT", cfg.ServerConfig.WebRoot)
	setEnvDefault("LOG_LEVEL", cfg.LoggingConfig.Level)
	setEnvDefault("LOG_DIR", cfg.LoggingConfig.LogDir)
	setEnvDefault("LOG_FORMAT_CONSOLE", cfg.LoggingConfig.ConsoleFormat)
	setEnvDefault("LOG_FORMAT_FILE", cfg.LoggingConfig.FileFormat)
	setEnvDefault("REPO_AUDIT_VERBOSE", cfg.LoggingConfig.RepositoryAuditVerbose)

	setEnvDefault("DB_HOST", cfg.DatabaseConfig.Host)
	setEnvDefault("DB_PORT", cfg.DatabaseConfig.Port)
	setEnvDefault("DB_USER", cfg.DatabaseConfig.User)
	setEnvDefault("DB_PASSWORD", cfg.DatabaseConfig.Password)
	setEnvDefault("DB_NAME", cfg.DatabaseConfig.DBName)
	setEnvDefault("DB_SSL", cfg.DatabaseConfig.SSL)

	setEnvDefault("STORAGE_PATH", cfg.StorageConfig.Path)
	setEnvDefault("STORAGE_STRATEGY", cfg.StorageConfig.Strategy)
	setEnvDefault("STORAGE_PRESERVE_FILENAME", strconv.FormatBool(cfg.StorageConfig.PreserveFilename))
	setEnvDefault("STORAGE_DUPLICATE_HANDLING", cfg.StorageConfig.DuplicateHandling)

	setEnvDefault("REPOSITORY_SCAN_ENABLED", strconv.FormatBool(cfg.RepositoryScan.Enabled))
	setEnvDefault("REPOSITORY_SCAN_INTERVAL_SECONDS", strconv.Itoa(cfg.RepositoryScan.IntervalSeconds))
	setEnvDefault("REPOSITORY_SCAN_SETTLE_SECONDS", strconv.Itoa(cfg.RepositoryScan.SettleSeconds))
	setEnvDefault("REPOSITORY_SCAN_MAX_CONCURRENT_REPOS", strconv.Itoa(cfg.RepositoryScan.MaxConcurrentRepos))
	setEnvDefault("REPOSITORY_SCAN_BATCH_SIZE", strconv.Itoa(cfg.RepositoryScan.BatchSize))

	setEnvDefault("GEOCODING_PROVIDER", cfg.Geocoding.Provider)
	setEnvDefault("GEOCODING_NOMINATIM_ENDPOINT", cfg.Geocoding.NominatimEndpoint)
	setEnvDefault("GEOCODING_LANGUAGE", cfg.Geocoding.Language)
	setEnvDefault("GEOCODING_USER_AGENT", cfg.Geocoding.UserAgent)

	setEnvDefault("ML_SEMANTIC_ENABLED", strconv.FormatBool(cfg.MLConfig.SemanticEnabled))
	setEnvDefault("ML_BIOCLIP_ENABLED", strconv.FormatBool(cfg.MLConfig.BioCLIPEnabled))
	setEnvDefault("ML_OCR_ENABLED", strconv.FormatBool(cfg.MLConfig.OCREnabled))
	setEnvDefault("ML_FACE_ENABLED", strconv.FormatBool(cfg.MLConfig.FaceEnabled))
	setEnvDefault("ML_ZEROSHOT_CLASSIFY_ENABLED", strconv.FormatBool(cfg.MLConfig.ZeroshotClassifyEnabled))

	setEnvDefault("LLM_AGENT_ENABLED", strconv.FormatBool(cfg.LLMConfig.AgentEnabled))
	setEnvDefault("LLM_PROVIDER", cfg.LLMConfig.Provider)
	setEnvDefault("LLM_API_KEY", cfg.LLMConfig.APIKey)
	setEnvDefault("LLM_MODEL_NAME", cfg.LLMConfig.ModelName)
	setEnvDefault("LLM_BASE_URL", cfg.LLMConfig.BaseURL)

	setEnvDefault("LUMILIO_SECRET_KEY", cfg.Auth.SecretKeyPath)
	setEnvDefault("ACCESS_TOKEN_TTL", cfg.Auth.AccessTokenTTL)
	setEnvDefault("REFRESH_TOKEN_TTL", cfg.Auth.RefreshTokenTTL)
	setEnvDefault("MEDIA_TOKEN_TTL", cfg.Auth.MediaTokenTTL)
	setEnvDefault("WEBAUTHN_RP_NAME", cfg.Auth.WebAuthnRPName)
	setEnvDefault("WEBAUTHN_RP_ID", cfg.Auth.WebAuthnRPID)
	setEnvDefault("WEBAUTHN_RP_ORIGINS", strings.Join(cfg.Auth.WebAuthnRPOrigins, ","))

	setEnvDefault("TRANSCODE_HW_ACCEL", cfg.Transcode.HardwareAccel)

	// External media tool overrides. Empty values are skipped by setEnvDefault,
	// so the bare command name (resolved via PATH) remains the default.
	setEnvDefault("EXIFTOOL_PATH", cfg.Tools.ExifToolPath)
	setEnvDefault("FFMPEG_PATH", cfg.Tools.FFmpegPath)
	setEnvDefault("FFPROBE_PATH", cfg.Tools.FFprobePath)

	setEnvDefault("LUMEN_DISCOVERY_ENABLED", strconv.FormatBool(cfg.Lumen.DiscoveryEnabled))
	setEnvDefault("LUMEN_DISCOVERY_MDNS_ENABLED", strconv.FormatBool(cfg.Lumen.DiscoveryMDNSEnabled))
	setEnvDefault("LUMEN_DISCOVERY_HUB_URL", cfg.Lumen.DiscoveryHubURL)
	setEnvDefault("LUMEN_CONNECTION_INSECURE", strconv.FormatBool(cfg.Lumen.ConnectionInsecure))
}

func envString(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func envBool(key string) (bool, bool) {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch raw {
	case "true", "1", "yes", "on":
		return true, true
	case "false", "0", "no", "off":
		return false, true
	default:
		return false, false
	}
}

func envPositiveInt(key string) (int, bool) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func setEnvDefault(key string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if _, ok := os.LookupEnv(key); ok {
		return
	}
	_ = os.Setenv(key, value)
}
