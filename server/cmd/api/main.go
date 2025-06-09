package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"server/config"
	"server/internal/api"
	"server/internal/api/handler"
	"server/internal/models"
	"server/internal/repository/gorm_repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/utils"

	"github.com/joho/godotenv"
)

func init() {
	log.SetOutput(os.Stdout)

	// Try to load .env file but continue if it's not found
	if err := godotenv.Load(); err != nil {
		log.Println("Running without .env file, using environment variables")
	} else {
		log.Println("Environment variables loaded from .env file")
	}
}

func main() {
	dbConfig := config.LoadDBConfig()
	// 添加测试日志，验证日志配置是否生效
	log.Println("Starting application...")
	// Connect to the database
	database := gorm_repo.InitDB(dbConfig)

	// Auto-migrate database models
	log.Println("Running database migrations...")
	err := database.AutoMigrate(
		&models.Asset{},     // New unified asset model
		&models.Thumbnail{}, // Updated thumbnail model
		&models.Tag{},
		&models.Album{},
	)

	if err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}
	log.Println("Database migrations completed successfully")

	// Defer closing the database connection
	sqlDB, err := database.DB()
	if err != nil {
		panic(err)
	}

	defer func(sqlDB *sql.DB) {
		err := sqlDB.Close()
		if err != nil {
			panic(err)
		}
	}(sqlDB)

	// Initialize repositories
	assetRepo := gorm_repo.NewAssetRepository(database)

	// Initialize local storage
	// Get storage path from environment variable or use default
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = "/app/data/photos" // Default path for Docker container
	}
	log.Printf("Using storage path: %s", storagePath)

	localStorage, err := storage.NewLocalStorage(storagePath)
	if err != nil {
		log.Fatalf("Failed to initialize local storage: %v", err)
	}

	// Initialize services
	assetService := service.NewAssetService(assetRepo, localStorage)

	// Initialize image processor
	imageProcessor := utils.NewImageProcessor(assetService, localStorage, storagePath)

	// Initialize controllers
	assetController := handler.NewAssetHandler(assetService, imageProcessor)

	// Set up router with new asset endpoints
	router := api.NewRouter(assetController)

	// Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}

	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
