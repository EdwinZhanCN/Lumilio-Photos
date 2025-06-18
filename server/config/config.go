package config

import (
	"os"
	"strings"
)

// DatabaseConfig holds all the configuration for the database connection.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

// LoadDBConfig loads database settings from environment variables.
func LoadDBConfig() DatabaseConfig {
	// Check if we're in development mode
	isDev := strings.ToLower(os.Getenv("ENV")) == "development" ||
		strings.ToLower(os.Getenv("ENVIRONMENT")) == "development" ||
		os.Getenv("DEV_MODE") == "true"

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
