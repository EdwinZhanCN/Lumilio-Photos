package main

import (
	"database/sql"
	"log"
	"os"
	"os/signal"
	"server/config"
	"server/internal/models"
	"server/internal/queue"
	"server/internal/repository/gorm_repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/utils"
	"syscall"
	"time"

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
	log.Println("Starting worker service...")

	// Load configuration
	dbConfig := config.LoadDBConfig()

	// Connect to the database
	database := gorm_repo.InitDB(dbConfig)

	// Auto-migrate database models if needed
	err := database.AutoMigrate(
		&models.Asset{},
		&models.Thumbnail{},
		&models.Tag{},
		&models.Album{},
	)
	if err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

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

	// Initialize asset processor
	assetProcessor := utils.NewAssetProcessor(assetService, localStorage, storagePath)

	// Initialize task queue
	queueDir := os.Getenv("QUEUE_DIR")
	if queueDir == "" {
		queueDir = "/app/queue"
	}
	log.Printf("Using queue directory: %s", queueDir)

	taskQueue, err := queue.NewTaskQueue(queueDir, 100)
	if err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}
	defer taskQueue.Close()

	// Initialize task queue and start processing
	if err := taskQueue.Initialize(); err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}

	// Start the task processor
	stopChan := make(chan struct{})
	go processTasksLoop(taskQueue, &assetService, assetProcessor, storagePath, stopChan)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping worker...")
	close(stopChan)
	// Give tasks some time to complete
	time.Sleep(2 * time.Second)
	log.Println("Worker service stopped")
}

// processTasksLoop continuously processes tasks from the queue
func processTasksLoop(taskQueue *queue.TaskQueue, assetService *service.AssetService,
	assetProcessor *utils.AssetProcessor, storagePath string, stopChan chan struct{}) {
	log.Println("Task processor started, waiting for tasks...")

	// Create cleanup ticker
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-stopChan:
			log.Println("Task processor stopping...")
			return

		case <-cleanupTicker.C:
			log.Println("Running task queue cleanup...")
			if err := taskQueue.CleanupProcessedTasks(); err != nil {
				log.Printf("Error cleaning up processed tasks: %v", err)
			}

		default:
			task, ok := taskQueue.GetTask()
			if !ok {
				// Channel closed or no tasks available
				time.Sleep(100 * time.Millisecond)
				continue
			}

			log.Printf("Processing task %s for file: %s", task.TaskID, task.StagedPath)

			// Check if file exists
			if _, err := os.Stat(task.StagedPath); os.IsNotExist(err) {
				log.Printf("Error: staged file not found for task %s: %v", task.TaskID, err)
				continue
			}

			// Verify hash (this would use the same Blake3 hash function from utils)
			calculatedHash, err := utils.CalculateFileHash(task.StagedPath)
			if err != nil {
				log.Printf("Error calculating hash for task %s: %v", task.TaskID, err)
				continue
			}

			if calculatedHash != task.ClientHash {
				log.Printf("Hash mismatch for task %s: expected %s, got %s",
					task.TaskID, task.ClientHash, calculatedHash)
				continue
			}

			// Process the asset (extract metadata, create thumbnails, etc.)
			asset, err := assetProcessor.ProcessNewAsset(task.StagedPath, task.UserID, task.FileName)
			if err != nil {
				log.Printf("Error processing asset for task %s: %v", task.TaskID, err)
				continue
			}

			// Move file to final storage location (CAS)
			destinationDir := assetProcessor.GetPathForHash(asset.Hash)
			if err := os.MkdirAll(destinationDir, 0755); err != nil {
				log.Printf("Error creating directory for asset: %v", err)
				continue
			}

			destinationPath := assetProcessor.GetFullPathForHash(asset.Hash)
			if err := os.Rename(task.StagedPath, destinationPath); err != nil {
				log.Printf("Error moving file to final location: %v", err)
				continue
			}

			// Mark task as complete
			if err := taskQueue.MarkTaskComplete(task.TaskID); err != nil {
				log.Printf("Error marking task %s as complete: %v", task.TaskID, err)
			} else {
				log.Printf("Task %s completed successfully", task.TaskID)
			}
		}
	}
}
