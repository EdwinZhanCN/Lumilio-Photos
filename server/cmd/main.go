package main

import (
	"database/sql"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
	"server/cmd/web"
	"server/db"
	"server/internal/controller"
	"server/internal/repository"
	"server/internal/service"
	"server/internal/storage"
)

func init() {
	// load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

func main() {
	// Connect to the database
	database := db.Connect("lumina-photos")

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
	
	localStorage, err := storage.NewLocalStorage(storagePath)
	if err != nil {
		log.Fatalf("Failed to initialize local storage: %v", err)
	}
	
	// Initialize services
	photoService := service.NewPhotoService(photoRepo, localStorage)
	
	// Initialize controllers
	photoController := controller.NewPhotoController(photoService)
	
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
