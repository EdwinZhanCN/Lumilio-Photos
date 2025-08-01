package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"server/config"
	"server/docs" // Import docs for swaggo
	"server/internal/api"
	"server/internal/api/handler"
	"server/internal/processors"
	"server/internal/queue"
	queuesetup "server/internal/queue/queue_setup"
	"server/internal/repository/gorm_repo"
	"server/internal/service"
	"server/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func init() {
	log.SetOutput(os.Stdout)

	// Load environment variables using unified config function
	config.LoadEnvironment()

	if config.IsDevelopmentMode() {
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

func main() {
	// Load configurations
	dbConfig := config.LoadDBConfig()
	appConfig := config.LoadAppConfig()

	log.Println("🚀 Starting Lumilio Photos API...")
	log.Printf("📊 Database configuration: %s:%s/%s", dbConfig.Host, dbConfig.Port, dbConfig.DBName)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to the database
	database := gorm_repo.InitDB(dbConfig)

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

	log.Println("GORM database connection successful.")

	Port, err := strconv.Atoi(dbConfig.Port)
	if err != nil {
		log.Fatalf("GORM database Port Interal Erro.")
	}

	pgxDSN := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		dbConfig.User,
		dbConfig.Password,
		dbConfig.Host,
		Port,
		dbConfig.DBName,
	)

	pgxPool, err := pgxpool.New(context.Background(), pgxDSN)
	if err != nil {
		log.Fatalf("Unable to create pgx connection pool: %v\n", err)
	}
	defer func() {
		log.Println("Closing pgx connection pool...")
		pgxPool.Close()
	}()

	log.Println("PGX connection pool for queue created successfully.")

	// Initialize repositories
	assetRepo := gorm_repo.NewAssetRepository(database)
	tagRepo := gorm_repo.NewTagRepository(database)
	userRepo := gorm_repo.NewUserRepository(database)
	embedRepo := gorm_repo.NewEmbedRepository(database)
	refreshTokenRepo := gorm_repo.NewRefreshTokenRepository(database)

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

	assetService, err := service.NewAssetService(assetRepo, tagRepo, embedRepo, storageService)
	if err != nil {
		log.Fatalf("Failed to initialize asset service: %v", err)
	}
	authService := service.NewAuthService(userRepo, refreshTokenRepo)

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
