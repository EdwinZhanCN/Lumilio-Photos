package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"

	"server/config"
	"server/docs" // Import docs for swaggo
	"server/internal/api"
	"server/internal/api/handler"
	"server/internal/models"
	"server/internal/queue"
	"server/internal/repository/gorm_repo"
	"server/internal/service"
	"server/internal/storage"

	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func init() {
	log.SetOutput(os.Stdout)

	// Check if we're in development mode
	isDev := strings.ToLower(os.Getenv("ENV")) == "development" ||
		strings.ToLower(os.Getenv("ENVIRONMENT")) == "development" ||
		os.Getenv("DEV_MODE") == "true"

	// Try to load .env file but continue if it's not found
	envFile := ".env"
	if isDev {
		// Try development-specific env file first
		if _, err := os.Stat(".env.development"); err == nil {
			envFile = ".env.development"
		}
	}

	if err := godotenv.Load(envFile); err != nil {
		log.Printf("Running without %s file, using environment variables", envFile)
	} else {
		log.Printf("Environment variables loaded from %s file", envFile)
	}

	if isDev {
		log.Println("🔧 Running in DEVELOPMENT mode")
		log.Println("📋 Development checklist:")
		log.Println("   1. Database: Make sure PostgreSQL is running on localhost:5432")
		log.Println("   2. Database: Use 'docker-compose up db' to start only the database")
		log.Println("   3. Storage: Local directories will be created automatically")
		log.Println("   4. Access: API will be available at http://localhost:8080")
	}
}

// @title Lumilio-Photos Manager API
// @version 1.0
// @description Photo management system API with asset upload, processing, and organization features

// @contact.name API Support
// @contact.url http://www.github.com/EdwinZhanCN/Lumilio-Photos

// @license.name GPLv3.0
// @license.url https://opensource.org/licenses/GPL-3.0

// @host localhost:3001
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

// 该主程序启动API层，API层只且仅只处理认证，接受文件，任务入队
// 只在除了文件上传之外的任务调用AssetsService及其方法
func main() {
	dbConfig := config.LoadDBConfig()
	log.Println("🚀 Starting Lumilio Photos API...")
	log.Printf("📊 Database configuration: %s:%s/%s", dbConfig.Host, dbConfig.Port, dbConfig.DBName)

	// Connect to the database
	database := gorm_repo.InitDB(dbConfig)

	// Auto-migrate database models
	log.Println("Running database migrations...")
	err := database.AutoMigrate(
		&models.RefreshToken{}, // Refresh token model
		&models.User{},         // User model for authentication
		&models.Asset{},        // New unified asset model
		&models.Thumbnail{},    // Updated thumbnail model
		&models.Tag{},
		&models.Album{},
		&models.AssetTag{}, // Join table for assets and tags with metadata
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
	tagRepo := gorm_repo.NewTagRepository(database)
	userRepo := gorm_repo.NewUserRepository(database)
	refreshTokenRepo := gorm_repo.NewRefreshTokenRepository(database)

	// Storage will be initialized through AssetService with configuration

	// Initialize staging area for temporary file storage
	stagingPath := os.Getenv("STAGING_PATH")
	if stagingPath == "" {
		if strings.ToLower(os.Getenv("ENV")) == "development" {
			stagingPath = "./staging" // Local development path
		} else {
			stagingPath = "/app/staging" // Container path
		}
	}
	log.Printf("📁 Using staging path: %s", stagingPath)

	// Ensure staging directory exists
	if err := os.MkdirAll(stagingPath, 0755); err != nil {
		log.Fatalf("Failed to create staging directory: %v", err)
	}

	// Initialize task queue
	queueDir := os.Getenv("QUEUE_DIR")
	if queueDir == "" {
		if strings.ToLower(os.Getenv("ENV")) == "development" {
			queueDir = "./queue" // Local development path
		} else {
			queueDir = "/app/queue" // Container path
		}
	}
	log.Printf("📋 Using queue directory: %s", queueDir)

	taskQueue, err := queue.NewTaskQueue(queueDir, 100)
	if err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}

	// Initialize the queue
	if err := taskQueue.Initialize(); err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}
	defer taskQueue.Close()

	// Load storage configuration
	storageConfig := storage.LoadStorageConfigFromEnv()
	log.Printf("💾 Storage strategy: %s (%s)", storageConfig.Strategy, storageConfig.Strategy.GetDescription())
	log.Printf("💾 Storage path: %s", storageConfig.BasePath)
	log.Printf("💾 Preserve filenames: %t", storageConfig.Options.PreserveOriginalFilename)
	log.Printf("💾 Duplicate handling: %s", storageConfig.Options.HandleDuplicateFilenames)

	// Initialize services, service layer inside the api layer only responsible for non-upload logic
	assetService, err := service.NewAssetServiceWithConfig(assetRepo, tagRepo, storageConfig)
	if err != nil {
		log.Fatalf("Failed to initialize asset service: %v", err)
	}

	// Initialize authentication service
	authService := service.NewAuthService(userRepo, refreshTokenRepo)

	// Initialize controllers - pass the staging path and task queue to the handler
	assetController := handler.NewAssetHandler(assetService, stagingPath, taskQueue)
	authController := handler.NewAuthHandler(authService)

	// Start HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}

	// Initialize Swagger docs
	docs.SwaggerInfo.Title = "Lumilio-Photos API"
	docs.SwaggerInfo.Description = "Photo management system API with asset upload, processing, and organization features"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = "localhost:" + port
	docs.SwaggerInfo.BasePath = "/api/v1"

	// Set up router with new asset and auth endpoints
	router := api.NewRouter(assetController, authController)

	// Add Swagger documentation endpoint
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	log.Printf("🌐 Server starting on port %s...", port)
	log.Printf("📖 API Documentation: http://localhost:%s/swagger/index.html", port)
	log.Printf("🔗 Health Check: http://localhost:%s/api/v1/health", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
