package db

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"os"
	"time"
)

func Connect(dbName string) *gorm.DB {
	// Get database connection details from environment variables with defaults
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432" // Changed to standard PostgreSQL port
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgres"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		dbHost, dbUser, dbPassword, dbName, dbPort)

	var db *gorm.DB
	var err error

	// Retry logic for database connection
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err == nil {
			// Test the connection
			sqlDB, dbErr := db.DB()
			if dbErr == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					log.Printf("Successfully connected to database '%s'", dbName)
					break
				}
			}
		}

		retryDelay := time.Duration(i+1) * 2 * time.Second
		log.Printf("Failed to connect to database: %v. Retrying in %v... (%d/%d)",
			err, retryDelay, i+1, maxRetries)
		time.Sleep(retryDelay)
	}

	if err != nil {
		log.Fatalf("Failed to connect to database after %d attempts: %v", maxRetries, err)
	}

	// Create schema if it doesn't exist and set search path
	db.Exec("CREATE SCHEMA IF NOT EXISTS public")
	db.Exec("SET search_path TO public")

	return db
}
