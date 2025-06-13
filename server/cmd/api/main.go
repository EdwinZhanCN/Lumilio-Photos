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
	"server/internal/queue"
	"server/internal/repository/gorm_repo"
	"server/internal/service"
	"server/internal/storage"

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

// 该主程序启动API层，API层只且仅只处理认证，接受文件，任务入队
// 只在除了文件上传之外的任务调用AssetsService及其方法
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
		storagePath = "/app/data/photos" // 最终储存区，required by AssetsService
	}
	log.Printf("Using storage path: %s", storagePath)

	localStorage, err := storage.NewLocalStorage(storagePath)
	if err != nil {
		log.Fatalf("Failed to initialize local storage: %v", err)
	}

	// Initialize staging area for temporary file storage
	stagingPath := os.Getenv("STAGING_PATH")
	if stagingPath == "" {
		stagingPath = "/app/staging" // 临时暂存区，用于初次存储用户上传的文件
	}
	log.Printf("Using staging path: %s", stagingPath)

	// Ensure staging directory exists
	if err := os.MkdirAll(stagingPath, 0755); err != nil {
		log.Fatalf("Failed to create staging directory: %v", err)
	}

	// Initialize task queue
	queueDir := os.Getenv("QUEUE_DIR")
	if queueDir == "" {
		queueDir = "/app/queue" // 持久化队列
	}
	log.Printf("Using queue directory: %s", queueDir)

	taskQueue, err := queue.NewTaskQueue(queueDir, 100)
	if err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}

	// Initialize the queue
	if err := taskQueue.Initialize(); err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}
	defer taskQueue.Close()

	// Initialize services, service layer inside the api layer only responsible for non-upload logic
	assetService := service.NewAssetService(assetRepo, localStorage)

	// Initialize controllers - pass the staging path and task queue to the handler
	assetController := handler.NewAssetHandler(assetService, stagingPath, taskQueue)

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
