package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Unified Configuration Management
// This package provides centralized configuration loading for both API and Worker services.
// It ensures consistent environment variable loading and configuration management across all services.

// DatabaseConfig holds all the configuration for the database connection.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// AppConfig holds general application configuration
type AppConfig struct {
	Port          string
	StagingPath   string
	QueueDir      string
	MLServiceAddr string
	Debug         bool
	LogLevel      string
}

// IsDevelopmentMode checks if the application is running in development mode
// Used consistently across all services to determine environment-specific configurations
func IsDevelopmentMode() bool {
	return strings.ToLower(os.Getenv("ENV")) == "development" ||
		strings.ToLower(os.Getenv("ENVIRONMENT")) == "development" ||
		os.Getenv("DEV_MODE") == "true"
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
		log.Println("🔧 Running in DEVELOPMENT mode")
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
			Host:     "localhost", // For development, connect to localhost
			Port:     "5432",
			User:     "postgres",
			Password: "postgres",
			DBName:   "lumiliophotos", // Match docker-compose database name
		}
	} else {
		// Production/Docker defaults
		cfg = DatabaseConfig{
			Host:     "db", // In Docker Compose, the hostname is the service name
			Port:     "5432",
			User:     "postgres",
			Password: "postgres",
			DBName:   "lumiliophotos", // Match docker-compose database name
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

	return cfg
}

// LoadAppConfig loads general application configuration
// Provides unified configuration for paths, ports, and service addresses
// Used by both API and Worker services to maintain consistency
func LoadAppConfig() AppConfig {
	isDev := IsDevelopmentMode()

	var cfg AppConfig

	// Set defaults based on environment
	if isDev {
		cfg = AppConfig{
			Port:          "8080",
			StagingPath:   "./staging",
			QueueDir:      "./queue",
			MLServiceAddr: "localhost:50051",
			Debug:         true,
			LogLevel:      "debug",
		}
	} else {
		cfg = AppConfig{
			Port:          "8080",
			StagingPath:   "/app/staging",
			QueueDir:      "/app/queue",
			MLServiceAddr: "ml:50051",
			Debug:         false,
			LogLevel:      "info",
		}
	}

	// Override with environment variables if set
	if port := os.Getenv("PORT"); port != "" {
		cfg.Port = port
	}
	if stagingPath := os.Getenv("STAGING_PATH"); stagingPath != "" {
		cfg.StagingPath = stagingPath
	}
	if queueDir := os.Getenv("QUEUE_DIR"); queueDir != "" {
		cfg.QueueDir = queueDir
	}
	if mlAddr := os.Getenv("ML_SERVICE_ADDR"); mlAddr != "" {
		cfg.MLServiceAddr = mlAddr
	}
	if debug := os.Getenv("DEBUG"); debug == "true" {
		cfg.Debug = true
	} else if debug == "false" {
		cfg.Debug = false
	}
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
	}

	return cfg
}
