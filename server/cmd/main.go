package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"server/cmd/web"
	"server/db"
	"server/internal/controller"
	"server/internal/models"
	"server/internal/repository"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/utils"

	"github.com/joho/godotenv"
)

func init() {
	log.SetOutput(os.Stdout)

	// load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}
}

func main() {
	// 添加测试日志，验证日志配置是否生效
	log.Println("Starting application...")

	// Connect to the database
	database := db.Connect("lumina-photos")

	// Auto-migrate database models
	log.Println("Running database migrations...")
	err := database.AutoMigrate(
		&models.Photo{},
		&models.PhotoMetadata{},
		&models.Thumbnail{},
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
	photoRepo := repository.NewPhotoRepository(database)

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
	photoService := service.NewPhotoService(photoRepo, localStorage)

	// Initialize image processor
	imageProcessor := utils.NewImageProcessor(photoService, localStorage, storagePath)

	// Initialize controllers
	photoController := controller.NewPhotoController(photoService, imageProcessor)

	// Set up router
	router := web.NewRouter(photoController)

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
