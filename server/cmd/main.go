package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"server/config"
	"server/docs" // Import docs for swaggo
	"server/internal/api"
	"server/internal/api/handler"
	"server/internal/db"
	"server/internal/processors"
	"server/internal/queue"
	queuesetup "server/internal/queue/queue_setup"
	"server/internal/service"
	"server/internal/storage"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func init() {
	log.SetOutput(os.Stdout)

	// Load environment variables using unified config function
	config.LoadEnvironment()
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

func main() {
	// Load configurations
	dbConfig := config.LoadDBConfig()
	appConfig := config.LoadAppConfig()

	log.Println("🚀 Starting Lumilio Photos API...")
	log.Printf("📊 Database configuration: %s:%s/%s", dbConfig.Host, dbConfig.Port, dbConfig.DBName)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run database migrations
	if err := db.AutoMigrate(ctx, dbConfig); err != nil {
		log.Printf("Warning: Failed to run migrations automatically: %v", err)
		log.Println("Please run migrations manually using: atlas migrate apply")
	}

	// Connect to the database
	database, err := db.New(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Use the same pool for both SQLC queries and pgx operations
	pgxPool := database.Pool

	// Initialize repositories using SQLC queries directly
	queries := database.Queries

	// Storage will be initialized through AssetService with configuration

	// Initialize staging area for temporary file storage
	log.Printf("📁 Using staging path: %s", appConfig.StagingPath)

	// Ensure staging directory exists
	if err := os.MkdirAll(appConfig.StagingPath, 0755); err != nil {
		log.Fatalf("Failed to create staging directory: %v", err)
	}

	// Load storage configuration
	storageConfig := storage.LoadStorageConfigFromEnv()
	log.Printf("💾 Storage strategy: %s (%s)", storageConfig.Strategy, storageConfig.Strategy.GetDescription())
	log.Printf("💾 Storage path: %s", storageConfig.BasePath)
	log.Printf("💾 Preserve filenames: %t", storageConfig.Options.PreserveOriginalFilename)
	log.Printf("💾 Duplicate handling: %s", storageConfig.Options.HandleDuplicateFilenames)
	storageService, err := storage.NewStorageWithConfig(storageConfig)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	assetService, err := service.NewAssetService(queries, storageService)
	if err != nil {
		log.Fatalf("Failed to initialize asset service: %v", err)
	}
	authService := service.NewAuthService(queries)

	mlService, err := service.NewMLClient(appConfig.MLServiceAddr)
	if err != nil {
		log.Fatalf("Failed to connect to ML gRPC server: %v", err)
	}

	clipQueue := queuesetup.SetupCLIPQueue(ctx, pgxPool, mlService, assetService)
	assetProcessor := processors.NewAssetProcessor(assetService, mlService, storageService, clipQueue)
	assetQueue := queuesetup.SetupAssetQueue(ctx, pgxPool, assetProcessor)

	go runQueue(ctx, assetQueue, "assetQueue")
	go runQueue(ctx, clipQueue, "clipQueue")

	// Initialize controllers - pass the staging path and task queue to the handler
	assetController := handler.NewAssetHandler(assetService, appConfig.StagingPath, assetQueue)
	authController := handler.NewAuthHandler(authService)

	// Initialize Swagger docs
	docs.SwaggerInfo.Title = "Lumilio-Photos API"
	docs.SwaggerInfo.Description = "Photo management system API with asset upload, processing, and organization features"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = "localhost:" + appConfig.Port
	docs.SwaggerInfo.BasePath = "/api/v1"

	// Set up router with new asset and auth endpoints
	router := api.NewRouter(assetController, authController)

	// Add Swagger documentation endpoint
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	log.Printf("🌐 Server starting on port %s...", appConfig.Port)
	log.Printf("📖 API Documentation: http://localhost:%s/swagger/index.html", appConfig.Port)
	log.Printf("🔗 Health Check: http://localhost:%s/api/v1/health", appConfig.Port)
	if err := http.ListenAndServe(":"+appConfig.Port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func runQueue[T any](ctx context.Context, q queue.Queue[T], name string) {
	log.Printf("[%s] Starting queue workers...", name)
	if err := q.Start(ctx); err != nil {
		log.Printf("[%s] Queue workers stopped with error: %v", name, err)
	}
}
