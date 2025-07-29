package gorm_repo

import (
	"database/sql"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"server/config"
	"server/internal/models"
	"time"
)

// InitDB initializes the database connection using the provided configuration.
// It includes retry logic to handle initial connection failures, common in containerized environments.
func InitDB(cfg config.DatabaseConfig) *gorm.DB {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port)

	var db *gorm.DB
	var err error

	const maxRetries = 5
	const retryBaseDelay = 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		// Attempt to open the connection
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

		// If gorm.Open is successful, err will be nil. We then proceed to ping.
		if err == nil {
			var sqlDB *sql.DB
			sqlDB, err = db.DB() // Try to get the underlying sql.DB object
			if err == nil {
				err = sqlDB.Ping() // Try to ping the database
			}
		}

		// Check if the entire process (open + get handle + ping) was successful
		if err == nil {
			log.Printf("✅ Successfully connected to database '%s'", cfg.DBName)
			db.Exec("SET search_path TO public")

			// AutoMigrate all models
			err = db.AutoMigrate(
				&models.Asset{},
				&models.RefreshToken{},
				&models.User{},
				&models.Tag{},
				&models.AssetTag{},
				&models.Thumbnail{},
				&models.Album{},
				&models.AlbumAsset{},
			)
			if err != nil {
				log.Fatalf("❌ Failed to auto migrate database: %v", err)
			}

			// Create necessary extensions and indexes
			db.Exec("CREATE EXTENSION IF NOT EXISTS vector;")
			db.Exec(`CREATE INDEX IF NOT EXISTS assets_hnsw_idx
             ON assets USING hnsw (embedding vector_l2_ops)
             WITH (m = 16, ef_construction = 200)`)

			return db // Success! Return the connection.
		}

		// If we've reached here, it means an error occurred in this attempt.
		retryDelay := time.Duration(i+1) * retryBaseDelay
		log.Printf("⚠️ Failed to connect to database: %v. Retrying in %v... (%d/%d)",
			err, retryDelay, i+1, maxRetries)
		time.Sleep(retryDelay)
	}

	// If the loop finishes, it means we failed all attempts.
	log.Fatalf("❌ Failed to connect to database after %d attempts: %v", maxRetries, err)
	return nil // Will not be reached due to log.Fatalf
}
