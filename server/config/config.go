package config

import (
	"os"
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
	cfg := DatabaseConfig{
		// Set defaults for easy local development
		Host:     "db", // In Docker Compose, the hostname is the service name.
		Port:     "5432",
		User:     "postgres",
		Password: "postgres",
		DBName:   "phasma_db", // A sensible default name
	}

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
