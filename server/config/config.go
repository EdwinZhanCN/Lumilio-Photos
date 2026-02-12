package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// DatabaseConfig holds all the configuration for the database connection.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSL      string
}

// AppConfig holds general application configuration
type AppConfig struct {
	ServerConfig   ServerConfig
	LLMConfig      LLMConfig
	MLConfig       MLConfig
	WatchmanConfig WatchmanConfig
}

type ServerConfig struct {
	Port     string `env:"SERVER_PORT,default=8080"`
	LogLevel string `env:"SERVER_LOG_LEVEL,default=info"`
}

type LLMConfig struct {
	AgentEnabled bool   `env:"LLM_AGENT_ENABLED,default=false"`
	Provider     string `env:"LLM_PROVIDER,default="`
	APIKey       string `env:"LLM_API_KEY,default="`
	ModelName    string `env:"LLM_MODEL_NAME,default="`
	BaseURL      string `env:"LLM_BASE_URL,default="`
}

type MLConfig struct {
	CLIPEnabled    bool `env:"ML_CLIP_ENABLED,default=false"`
	OCREnabled     bool `env:"ML_OCR_ENABLED,default=false"`
	CaptionEnabled bool `env:"ML_CAPTION_ENABLED,default=false"`
	FaceEnabled    bool `env:"ML_FACE_ENABLED,default=false"`
}

// WatchmanConfig controls repository file tree monitoring.
type WatchmanConfig struct {
	Enabled             bool
	SocketPath          string
	SettleSeconds       int
	InitialScan         bool
	PollFallbackSeconds int
}

// IsDevelopmentMode checks if the application is running in development mode
func IsDevelopmentMode() bool {
	return strings.ToLower(os.Getenv("SERVER_ENV")) == "development"
}

// LoadEnvironment loads environment variables from appropriate .env file
// This function should be called in the init() function of both API and Worker main.go files
// It automatically loads .env.development in development mode, .env otherwise
func LoadEnvironment() {
	isDev := IsDevelopmentMode()

	// Choose appropriate env file
	envFile := ".env"
	if isDev {
		// Try development-specific env file first
		if _, err := os.Stat(".env.development"); err == nil {
			envFile = ".env.development"
		}
	}

	// Try to load .env file but continue if it's not found
	if err := godotenv.Load(envFile); err != nil {
		log.Printf("Running without %s file, using environment variables", envFile)
	} else {
		log.Printf("Environment variables loaded from %s file", envFile)
	}

	if isDev {
		log.Println("Running in DEVELOPMENT mode")
	}
}

// LoadDBConfig loads database settings from environment variables
// Used by both API and Worker services for consistent database configuration
func LoadDBConfig() DatabaseConfig {
	isDev := IsDevelopmentMode()

	var cfg DatabaseConfig

	if isDev {
		// Development defaults - connect to localhost
		cfg = DatabaseConfig{
			Host:     "localhost",
			Port:     "5432",
			User:     "postgres",
			Password: "postgres",
			DBName:   "lumiliophotos",
			SSL:      "disable",
		}
	} else {
		// Production/Docker defaults
		cfg = DatabaseConfig{
			Host:     "db",
			Port:     "5432",
			User:     "postgres",
			Password: "postgres",
			DBName:   "lumiliophotos",
			SSL:      "disable",
		}
	}

	// Override with environment variables if set
	if host := os.Getenv("DB_HOST"); host != "" {
		cfg.Host = host
	}
	if port := os.Getenv("DB_PORT"); port != "" {
		cfg.Port = port
	}
	if user := os.Getenv("DB_USER"); user != "" {
		cfg.User = user
	}
	if password := os.Getenv("DB_PASSWORD"); password != "" {
		cfg.Password = password
	}
	if dbname := os.Getenv("DB_NAME"); dbname != "" {
		cfg.DBName = dbname
	}
	if ssl := os.Getenv("DB_SSL"); ssl != "" {
		cfg.SSL = ssl
	}

	return cfg
}

// LoadAppConfig loads general application configuration
func LoadAppConfig() AppConfig {
	var cfg AppConfig
	cfg.ServerConfig = LoadServerConfig()
	cfg.MLConfig = LoadMLConfig()
	cfg.LLMConfig = LoadLLMConfig()
	cfg.WatchmanConfig = LoadWatchmanConfig()

	return cfg
}

func LoadServerConfig() ServerConfig {
	var cfg ServerConfig

	// Default to development settings
	isDev := IsDevelopmentMode()
	if isDev {
		cfg = ServerConfig{
			Port:     "8080",
			LogLevel: "debug",
		}
	} else {
		cfg = ServerConfig{
			Port:     "8080",
			LogLevel: "info",
		}
	}

	// Override with environment variables if set
	if port := os.Getenv("SERVER_PORT"); port != "" {
		cfg.Port = port
	}
	if logLevel := os.Getenv("SERVER_LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	return cfg
}

// LoadMLConfig loads ML/AI service configuration from environment variables
func LoadMLConfig() MLConfig {
	var cfg MLConfig

	// Default to disabled for development, can be overridden by environment variables
	isDev := IsDevelopmentMode()
	if isDev {
		cfg = MLConfig{
			CLIPEnabled:    false,
			OCREnabled:     false,
			CaptionEnabled: false,
			FaceEnabled:    false,
		}
	} else {
		cfg = MLConfig{
			CLIPEnabled:    true,
			OCREnabled:     true,
			CaptionEnabled: true,
			FaceEnabled:    true,
		}
	}

	// Override with environment variables if set
	if clipEnabled := os.Getenv("ML_CLIP_ENABLED"); clipEnabled == "true" {
		cfg.CLIPEnabled = true
	} else if clipEnabled == "false" {
		cfg.CLIPEnabled = false
	}

	if ocrEnabled := os.Getenv("ML_OCR_ENABLED"); ocrEnabled == "true" {
		cfg.OCREnabled = true
	} else if ocrEnabled == "false" {
		cfg.OCREnabled = false
	}

	if captionEnabled := os.Getenv("ML_CAPTION_ENABLED"); captionEnabled == "true" {
		cfg.CaptionEnabled = true
	} else if captionEnabled == "false" {
		cfg.CaptionEnabled = false
	}

	if faceEnabled := os.Getenv("ML_FACE_ENABLED"); faceEnabled == "true" {
		cfg.FaceEnabled = true
	} else if faceEnabled == "false" {
		cfg.FaceEnabled = false
	}

	return cfg
}

// LoadLLMConfig loads LLM settings such as API key and model name from environment variables
func LoadLLMConfig() LLMConfig {
	var cfg LLMConfig

	if enabled := os.Getenv("LLM_AGENT_ENABLED"); enabled == "true" {
		cfg.AgentEnabled = true
	} else {
		cfg.AgentEnabled = false
	}

	if apiKey := os.Getenv("LLM_API_KEY"); apiKey != "" {
		cfg.APIKey = apiKey
	}
	if model := os.Getenv("LLM_MODEL_NAME"); model != "" {
		cfg.ModelName = model
	}
	//providers accroding to eino framework:
	//ark, deepseek, openai, claude, qwen, qianfan, gemini
	if provider := os.Getenv("LLM_PROVIDER"); provider != "" {
		cfg.Provider = provider
	}

	if baseURL := os.Getenv("LLM_BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}

	return cfg
}

// LoadWatchmanConfig loads file tree monitoring settings.
func LoadWatchmanConfig() WatchmanConfig {
	cfg := WatchmanConfig{
		Enabled:             false,
		SocketPath:          "",
		SettleSeconds:       3,
		InitialScan:         true,
		PollFallbackSeconds: 0,
	}

	if enabled := strings.ToLower(strings.TrimSpace(os.Getenv("WATCHMAN_ENABLED"))); enabled == "true" {
		cfg.Enabled = true
	}

	if socketPath := strings.TrimSpace(os.Getenv("WATCHMAN_SOCK")); socketPath != "" {
		cfg.SocketPath = socketPath
	}

	if settleRaw := strings.TrimSpace(os.Getenv("WATCHMAN_SETTLE_SECONDS")); settleRaw != "" {
		if settleSeconds, err := strconv.Atoi(settleRaw); err == nil && settleSeconds > 0 {
			cfg.SettleSeconds = settleSeconds
		}
	}

	if initialScanRaw := strings.ToLower(strings.TrimSpace(os.Getenv("WATCHMAN_INITIAL_SCAN"))); initialScanRaw == "false" {
		cfg.InitialScan = false
	}

	if pollRaw := strings.TrimSpace(os.Getenv("WATCHMAN_POLL_FALLBACK_SECONDS")); pollRaw != "" {
		if pollSeconds, err := strconv.Atoi(pollRaw); err == nil && pollSeconds >= 0 {
			cfg.PollFallbackSeconds = pollSeconds
		}
	}

	return cfg
}
