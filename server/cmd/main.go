package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"server/config"
	"server/docs" // Import docs for swaggo
	"server/internal/agent/core"
	"server/internal/agent/tools"
	"server/internal/api"
	"server/internal/api/handler"
	"server/internal/db"
	"server/internal/db/repo"
	"server/internal/processors"
	"server/internal/queue"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/storage/monitor"
	"server/internal/storage/repocfg"

	"github.com/riverqueue/river"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

func init() {
	log.SetOutput(os.Stdout)

	// Load environment variables using unified config function
	config.LoadEnvironment()
}

// @title Lumilio-Photos API
// @version 1.0
// @description Media management system API with asset upload, processing, and organization features

// @contact.name API Support
// @contact.url http://www.github.com/EdwinZhanCN/Lumilio-Photos

// @license.name GPLv3.0
// @license.url https://opensource.org/licenses/GPL-3.0

// @host localhost:8080
// @BasePath /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
// @openapi 3.0.0
func main() {
	// Load configurations
	dbConfig := config.LoadDBConfig()
	appConfig := config.LoadAppConfig()
	llmConfig := config.LoadLLMConfig()

	log.Println("🚀 Starting Lumilio Photos API...")
	log.Printf("📊 Database configuration: %s:%s/%s", dbConfig.Host, dbConfig.Port, dbConfig.DBName)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run database migrations
	if err := db.AutoMigrate(ctx, dbConfig); err != nil {
		log.Printf("Warning: Failed to run migrations automatically: %v", err)
		log.Println("Please run migrations manually using: migrate -path server/migrations -database \"$DATABASE_URL\" up")
	}

	// Connect to the database
	database, err := db.New(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	pgxPool := database.Pool
	queries := database.Queries

	// Initialize new repository-based storage system
	repoManager, err := storage.NewRepositoryManager(queries, pgxPool)
	if err != nil {
		log.Fatalf("Failed to initialize repository manager: %v", err)
	}
	stagingManager := storage.NewStagingManager()
	log.Println("✅ Repository Storage System Initialized")
	// Initialize primary storage repository
	log.Println("📁 Initializing primary storage repository...")
	if err := initPrimaryStorage(repoManager); err != nil {
		log.Printf("Warning: Failed to initialize primary storage: %v", err)
		log.Println("Please ensure STORAGE_PATH environment variable is set")
	}

	zapLogger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize zap logger: %v", err)
	}

	workers := river.NewWorkers()
	queueClient, err := queue.New(pgxPool, workers, appConfig.MLConfig)
	if err != nil {
		log.Fatalf("Failed to Initialize Queue: %v", err)
	}

	lumenService, embeddingService, err := initMLServices(ctx, appConfig.MLConfig, queries, workers, zapLogger)
	if err != nil {
		log.Fatalf("Failed to initialize ML services: %v", err)
	}

	if lumenService != nil {
		warmupTasks := []string{"clip_image_embed", "ocr", "vlm_generate", "face_detect_and_embed"}
		lumenService.WarmupTasks(ctx, warmupTasks)
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					lumenService.WarmupTasks(context.Background(), warmupTasks)
				}
			}
		}()
	}

	defer func() {
		if lumenService != nil {
			if err := lumenService.Close(); err != nil {
				log.Printf("⚠️  Failed to close lumen service: %v", err)
			}
		}
	}()

	assetService, err := service.NewAssetService(queries, lumenService, &repoManager, embeddingService)
	if err != nil {
		log.Fatalf("Failed to initialize asset service: %v", err)
	}
	authService := service.NewAuthService(queries)
	albumService := service.NewAlbumService(queries)

	// Initialize Agent Service
	agentService := core.NewAgentService(queries, llmConfig)
	log.Println("✅ Agent Service Initialized")

	// Register agent tools
	tools.RegisterFilterAsset()
	tools.RegisterBulkLikeTool()
	log.Println("✅ Agent Tools Registered")

	assetProcessor := processors.NewAssetProcessor(assetService, queries, repoManager, stagingManager, queueClient, appConfig, lumenService)
	river.AddWorker[queue.IngestAssetArgs](workers, &queue.IngestAssetWorker{Processor: assetProcessor})
	river.AddWorker[queue.DiscoverAssetArgs](workers, &queue.DiscoverAssetWorker{ProcessDiscover: assetProcessor.ProcessDiscoveredAsset})
	river.AddWorker[queue.MetadataArgs](workers, &queue.MetadataWorker{Process: assetProcessor.ProcessMetadataTask})
	river.AddWorker[queue.ThumbnailArgs](workers, &queue.ThumbnailWorker{Process: assetProcessor.ProcessThumbnailTask})
	river.AddWorker[queue.TranscodeArgs](workers, &queue.TranscodeWorker{Process: assetProcessor.ProcessTranscodeTask})
	river.AddWorker[queue.AssetRetryArgs](workers, &queue.AssetRetryWorker{ProcessRetry: assetProcessor.ProcessRetryTask})

	go func() {
		if err := queueClient.Start(context.Background()); err != nil {
			panic(err)
		}
	}()

	log.Println("✅ Queues initialized successfully")

	repoMonitor := monitor.NewWatchmanMonitor(queries, queueClient, appConfig.WatchmanConfig)
	if err := repoMonitor.Start(ctx); err != nil {
		log.Fatalf("Failed to start watchman monitor: %v", err)
	}
	defer repoMonitor.Stop()

	// Initialize controllers with new storage system
	assetController := handler.NewAssetHandler(assetService, queries, repoManager, stagingManager, queueClient)
	authController := handler.NewAuthHandler(authService)
	albumController := handler.NewAlbumHandler(&albumService, queries)
	queueController := handler.NewQueueHandler(queueClient, pgxPool)
	statsController := handler.NewStatsHandler(queries)
	agentController := handler.NewAgentHandler(agentService)

	// Initialize Swagger docs
	docs.SwaggerInfo.Title = "Lumilio-Photos API"
	docs.SwaggerInfo.Description = "Photo management system API with asset upload, processing, and organization features"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = "localhost:" + appConfig.ServerConfig.Port
	docs.SwaggerInfo.BasePath = "/api/v1"

	// Set up router with new asset, album, auth, stats and agent endpoints
	router := api.NewRouter(assetController, authController, albumController, queueController, statsController, agentController)

	// Add Swagger documentation endpoint
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	log.Printf("🌐 Server starting on port %s...", appConfig.ServerConfig.Port)
	log.Printf("📖 API Documentation: http://localhost:%s/swagger/index.html", appConfig.ServerConfig.Port)
	log.Printf("🔗 Health Check: http://localhost:%s/api/v1/health", appConfig.ServerConfig.Port)

	if err := http.ListenAndServe(":"+appConfig.ServerConfig.Port, router); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initMLServices(
	ctx context.Context,
	mlConfig config.MLConfig,
	queries *repo.Queries,
	workers *river.Workers,
	zapLogger *zap.Logger,
) (service.LumenService, service.EmbeddingService, error) {
	mlEnabled := mlConfig.CLIPEnabled || mlConfig.OCREnabled || mlConfig.CaptionEnabled || mlConfig.FaceEnabled

	if !mlEnabled {
		log.Println("ℹ️  ML features disabled, skipping Lumen and ML service initialization")
		return nil, nil, nil
	}

	log.Println("🤖 Initializing ML services...")

	lumenService, err := service.NewLumenService(nil, zapLogger)
	if err != nil {
		return nil, nil, err
	}

	err = lumenService.Start(ctx)
	if err != nil {
		return nil, nil, err
	}
	log.Println("✅ Lumen Service Initialized")

	var embeddingService service.EmbeddingService
	var speciesService service.SpeciesService
	var aiDescriptionService service.AIDescriptionService
	var ocrService service.OCRService
	var faceService service.FaceService

	if mlConfig.CLIPEnabled {
		embeddingService = service.NewEmbeddingService(queries)
		speciesService = service.NewSpeciesService(queries)

		river.AddWorker[queue.ProcessClipArgs](workers, &queue.ProcessClipWorker{
			LumenService:     lumenService,
			EmbeddingService: embeddingService,
			SpeciesService:   speciesService,
		})
		log.Println("✅ CLIP Service and Worker Registered")
	}

	if mlConfig.CaptionEnabled {
		aiDescriptionService = service.NewAIDescriptionService(queries, lumenService)

		river.AddWorker[queue.ProcessCaptionArgs](workers, &queue.ProcessCaptionWorker{
			AIDescriptionService: aiDescriptionService,
			LumenService:         lumenService,
		})
		log.Println("✅ Caption Service and Worker Registered")
	}

	if mlConfig.OCREnabled {
		ocrService = service.NewOCRService(queries)

		river.AddWorker[queue.ProcessOcrArgs](workers, &queue.ProcessOcrWorker{
			OCRService:   ocrService,
			LumenService: lumenService,
		})
		log.Println("✅ OCR Service and Worker Registered")
	}

	if mlConfig.FaceEnabled {
		faceService = service.NewFaceService(queries)

		river.AddWorker[queue.ProcessFaceArgs](workers, &queue.ProcessFaceWorker{
			FaceService:  faceService,
			LumenService: lumenService,
		})
		log.Println("✅ Face Service and Worker Registered")
	}

	log.Println("✅ ML Services Initialization Complete")
	return lumenService, embeddingService, nil
}

func initPrimaryStorage(repoManager storage.RepositoryManager) error {
	// Load environment variables
	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		return fmt.Errorf("STORAGE_PATH environment variable is required")
	}

	storageStrategy := os.Getenv("STORAGE_STRATEGY")
	if storageStrategy == "" {
		storageStrategy = "date" // Default to date strategy
	}

	preserveFilename := os.Getenv("STORAGE_PRESERVE_FILENAME")
	preserve := preserveFilename != "false" // Default to true

	duplicateHandling := os.Getenv("STORAGE_DUPLICATE_HANDLING")
	if duplicateHandling == "" {
		duplicateHandling = "rename" // Default to rename
	}

	// If a repository already exists at the storage path, register it if needed
	if repocfg.IsRepositoryRoot(storagePath) {
		// If it's already registered in DB, we're done
		if existing, err := repoManager.GetRepositoryByPath(storagePath); err == nil {
			log.Printf("✅ Primary storage already initialized at: %s", storagePath)
			log.Printf("   Repository ID: %s", existing.RepoID)
			return nil
		}

		// Otherwise, register the existing repository
		existingRepo, err := repoManager.AddRepository(storagePath)
		if err != nil {
			return fmt.Errorf("failed to register existing primary storage repository: %w", err)
		}

		log.Printf("✅ Primary storage registered at: %s", storagePath)
		log.Printf("   Repository ID: %s", existingRepo.RepoID)
		return nil
	}

	// Create repository configuration
	cfg := repocfg.NewRepositoryConfig(
		"Primary Storage",
		repocfg.WithStorageStrategy(storageStrategy),
		repocfg.WithLocalSettings(preserve, duplicateHandling, 0, false, false),
	)

	// Initialize a new repository with the configuration
	repo, err := repoManager.InitializeRepository(storagePath, *cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize primary storage repository: %w", err)
	}

	log.Printf("✅ Primary storage initialized at: %s", storagePath)
	log.Printf("   Repository ID: %s", repo.RepoID)
	log.Printf("   Storage Strategy: %s", storageStrategy)
	log.Printf("   Duplicate Handling: %s", duplicateHandling)
	log.Printf("   Preserve Filename: %v", preserve)

	return nil
}
