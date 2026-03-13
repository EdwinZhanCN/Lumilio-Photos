package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"server/config"
	"server/docs" // Import docs for swaggo
	"server/internal/agent/core"
	"server/internal/agent/tools"
	"server/internal/api"
	"server/internal/api/handler"
	"server/internal/db"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/processors"
	"server/internal/queue"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/storage/monitor"
	"server/internal/storage/repocfg"

	lumenconfig "github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

const primaryRepositoryFolderName = "primary"

type primaryStorageRepositoryManager interface {
	GetRepositoryByPath(path string) (*repo.Repository, error)
	AddRepository(path string) (*repo.Repository, error)
	InitializeRepository(path string, config repocfg.RepositoryConfig) (*repo.Repository, error)
}

func init() {
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
	logRuntime, err := logging.NewLogger(logging.LoadConfig(appConfig.ServerConfig.LogLevel))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logRuntime.Sync()
	restoreStdLog := logging.RedirectStandardLog(logRuntime.Named("stdlib"))
	defer restoreStdLog()

	appLogger := logRuntime.Named("app")
	lumenLogger := logRuntime.Named("lumen")
	repositoryLogger := logRuntime.Named("repository")
	processorLogger := logRuntime.Named("processor")
	indexingLogger := logRuntime.Named("indexing")
	watchmanLogger := logRuntime.Named("watchman")
	repoAuditProvider := logging.NewRepositoryAuditProvider(logRuntime.Named("repo_audit"))

	appLogger.Info("starting Lumilio Photos API",
		zap.String("operation", "server.start"),
		zap.String("db_host", dbConfig.Host),
		zap.String("db_port", dbConfig.Port),
		zap.String("db_name", dbConfig.DBName),
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run database migrations
	if err := db.AutoMigrate(ctx, dbConfig); err != nil {
		appLogger.Warn("failed to run migrations automatically",
			zap.String("operation", "database.migrate"),
			zap.Error(err),
		)
		appLogger.Warn("please run migrations manually using migrate -path server/migrations -database \"$DATABASE_URL\" up",
			zap.String("operation", "database.migrate"),
		)
	}

	// Connect to the database
	database, err := db.New(dbConfig)
	if err != nil {
		appLogger.Fatal("failed to connect to database", zap.String("operation", "database.connect"), zap.Error(err))
	}
	defer database.Close()
	pgxPool := database.Pool
	queries := database.Queries

	settingsService := service.NewSettingsService(queries)
	if err := settingsService.EnsureInitialized(ctx); err != nil {
		appLogger.Fatal("failed to initialize system settings", zap.String("operation", "settings.init"), zap.Error(err))
	}

	currentMLConfig, err := settingsService.GetMLConfig(ctx)
	if err != nil {
		appLogger.Fatal("failed to load ML settings", zap.String("operation", "settings.ml"), zap.Error(err))
	}
	if currentMLConfig.IsAutoEnabled() {
		appLogger.Info("ML auto mode enabled; discovery mode will accept all discovered ML tasks",
			zap.String("operation", "settings.ml"),
		)
	}

	// Initialize new repository-based storage system
	repoManager, err := storage.NewRepositoryManager(queries, repositoryLogger, repoAuditProvider)
	if err != nil {
		appLogger.Fatal("failed to initialize repository manager", zap.String("operation", "repository.init"), zap.Error(err))
	}
	stagingManager := storage.NewStagingManager()
	appLogger.Info("repository storage system initialized", zap.String("operation", "repository.init"))
	// Initialize primary storage repository
	appLogger.Info("initializing primary storage repository", zap.String("operation", "repository.primary"))
	if err := initPrimaryStorage(repoManager, repositoryLogger); err != nil {
		appLogger.Fatal("failed to initialize primary storage", zap.String("operation", "repository.primary"), zap.Error(err))
	}

	workers := river.NewWorkers()
	queueClient, err := queue.New(pgxPool, workers, logRuntime.RiverLogger())
	if err != nil {
		appLogger.Fatal("failed to initialize queue", zap.String("operation", "queue.init"), zap.Error(err))
	}
	faceService := service.NewFaceService(queries, repoManager)

	lumenService, embeddingService, err := initMLServices(ctx, pgxPool, queries, workers, appLogger, lumenLogger, settingsService, faceService)
	if err != nil {
		appLogger.Fatal("failed to initialize ML services", zap.String("operation", "ml.init"), zap.Error(err))
	}

	if lumenService != nil {
		warmupTasks := []string{"clip_image_embed", "clip_classify", "clip_scene_classify", "ocr", "vlm_generate", "face_detect_and_embed"}
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
				lumenLogger.Warn("failed to close lumen service", zap.String("operation", "lumen.close"), zap.Error(err))
			}
		}
	}()

	assetService, err := service.NewAssetService(queries, pgxPool, lumenService, &repoManager, embeddingService)
	if err != nil {
		appLogger.Fatal("failed to initialize asset service", zap.String("operation", "asset.init"), zap.Error(err))
	}
	indexingService := service.NewAssetIndexingService(queries, settingsService, lumenService, queueClient, pgxPool, indexingLogger, repoAuditProvider)
	authService := service.NewAuthService(queries, pgxPool)
	albumService := service.NewAlbumService(queries)
	userService := service.NewUserService(queries, pgxPool)

	// Initialize Agent Service
	agentService := core.NewAgentService(queries, settingsService)
	appLogger.Info("agent service initialized", zap.String("operation", "agent.init"))

	// Register agent tools
	tools.RegisterFilterAsset()
	tools.RegisterBulkLikeTool()
	appLogger.Info("agent tools registered", zap.String("operation", "agent.tools"))

	assetProcessor := processors.NewAssetProcessor(assetService, queries, repoManager, stagingManager, queueClient, settingsService, lumenService, processorLogger, repoAuditProvider)
	river.AddWorker[queue.IngestAssetArgs](workers, &queue.IngestAssetWorker{Processor: assetProcessor})
	river.AddWorker[queue.DiscoverAssetArgs](workers, &queue.DiscoverAssetWorker{ProcessDiscover: assetProcessor.ProcessDiscoveredAsset})
	river.AddWorker[queue.MetadataArgs](workers, &queue.MetadataWorker{Process: assetProcessor.ProcessMetadataTask})
	river.AddWorker[queue.ThumbnailArgs](workers, &queue.ThumbnailWorker{Process: assetProcessor.ProcessThumbnailTask})
	river.AddWorker[queue.TranscodeArgs](workers, &queue.TranscodeWorker{Process: assetProcessor.ProcessTranscodeTask})
	river.AddWorker[queue.AssetRetryArgs](workers, &queue.AssetRetryWorker{ProcessRetry: assetProcessor.ProcessRetryTask})
	river.AddWorker[queue.ReindexAssetsArgs](workers, &queue.ReindexAssetsWorker{IndexingService: indexingService})

	go func() {
		if err := queueClient.Start(context.Background()); err != nil {
			appLogger.Fatal("queue client stopped unexpectedly", zap.String("operation", "queue.start"), zap.Error(err))
		}
	}()

	appLogger.Info("queues initialized successfully", zap.String("operation", "queue.init"))

	repoMonitor := monitor.NewWatchmanMonitor(queries, queueClient, appConfig.WatchmanConfig, watchmanLogger)
	if err := repoMonitor.Start(ctx); err != nil {
		appLogger.Fatal("failed to start watchman monitor", zap.String("operation", "watchman.start"), zap.Error(err))
	}
	defer repoMonitor.Stop()

	// Initialize controllers with new storage system
	assetController := handler.NewAssetHandler(assetService, authService, indexingService, queries, repoManager, stagingManager, queueClient)
	authController := handler.NewAuthHandler(authService)
	albumController := handler.NewAlbumHandler(&albumService, queries)
	peopleController := handler.NewPeopleHandler(assetService, faceService, authService, repoManager)
	userController := handler.NewUserHandler(userService)
	queueController := handler.NewQueueHandler(queueClient, pgxPool)
	statsController := handler.NewStatsHandler(queries)
	agentController := handler.NewAgentHandler(agentService)
	capabilitiesController := handler.NewCapabilitiesHandler(settingsService, lumenService)
	settingsController := handler.NewSettingsHandler(settingsService)

	// Initialize Swagger docs
	docs.SwaggerInfo.Title = "Lumilio-Photos API"
	docs.SwaggerInfo.Description = "Photo management system API with asset upload, processing, and organization features"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = "localhost:" + appConfig.ServerConfig.Port
	docs.SwaggerInfo.BasePath = "/api/v1"

	// Set up router with new asset, album, auth, stats and agent endpoints
	router := api.NewRouter(
		assetController,
		authController,
		albumController,
		peopleController,
		queueController,
		statsController,
		agentController,
		capabilitiesController,
		settingsController,
		userController,
		handler.RequireLLMAgentEnabled(settingsService),
	)

	// Add Swagger documentation endpoint
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	appLogger.Info("server starting",
		zap.String("operation", "server.listen"),
		zap.String("port", appConfig.ServerConfig.Port),
		zap.String("swagger_url", fmt.Sprintf("http://localhost:%s/swagger/index.html", appConfig.ServerConfig.Port)),
		zap.String("health_url", fmt.Sprintf("http://localhost:%s/api/v1/health", appConfig.ServerConfig.Port)),
	)

	if err := http.ListenAndServe(":"+appConfig.ServerConfig.Port, router); err != nil {
		appLogger.Fatal("failed to start server", zap.String("operation", "server.listen"), zap.Error(err))
	}
}

func initMLServices(
	ctx context.Context,
	pgxPool *pgxpool.Pool,
	queries *repo.Queries,
	workers *river.Workers,
	appLogger *zap.Logger,
	lumenLogger *zap.Logger,
	settingsService service.SettingsService,
	faceService service.FaceService,
) (service.LumenService, service.EmbeddingService, error) {
	appLogger.Info("initializing ML services", zap.String("operation", "ml.init"))

	lumenCfg, err := lumenconfig.LoadConfig("")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load lumen sdk config: %w", err)
	}

	lumenService, err := service.NewLumenService(lumenCfg, lumenLogger)
	if err != nil {
		return nil, nil, err
	}

	err = lumenService.Start(ctx)
	if err != nil {
		return nil, nil, err
	}
	appLogger.Info("lumen service initialized", zap.String("operation", "ml.init"))

	embeddingService := service.NewEmbeddingService(queries, pgxPool)
	tagService := service.NewAIGeneratedTagService(queries)
	captionService := service.NewCaptionService(queries, lumenService)
	ocrService := service.NewOCRService(queries)

	river.AddWorker[queue.ProcessClipArgs](workers, &queue.ProcessClipWorker{
		LumenService:     lumenService,
		EmbeddingService: embeddingService,
		TagService:       tagService,
		ConfigProvider:   settingsService,
	})
	appLogger.Info("CLIP service and worker registered", zap.String("operation", "ml.init"))

	river.AddWorker[queue.ProcessCaptionArgs](workers, &queue.ProcessCaptionWorker{
		CaptionService: captionService,
		LumenService:   lumenService,
		ConfigProvider: settingsService,
	})
	appLogger.Info("caption service and worker registered", zap.String("operation", "ml.init"))

	river.AddWorker[queue.ProcessOcrArgs](workers, &queue.ProcessOcrWorker{
		OCRService:     ocrService,
		LumenService:   lumenService,
		ConfigProvider: settingsService,
	})
	appLogger.Info("OCR service and worker registered", zap.String("operation", "ml.init"))

	river.AddWorker[queue.ProcessFaceArgs](workers, &queue.ProcessFaceWorker{
		FaceService:    faceService,
		LumenService:   lumenService,
		ConfigProvider: settingsService,
	})
	appLogger.Info("face service and worker registered", zap.String("operation", "ml.init"))

	appLogger.Info("ML services initialization complete", zap.String("operation", "ml.init"))
	return lumenService, embeddingService, nil
}

func initPrimaryStorage(repoManager primaryStorageRepositoryManager, logger *zap.Logger) error {
	if logger == nil {
		logger = zap.NewNop()
	}
	// Load environment variables
	storagePath := os.Getenv("STORAGE_PATH")
	storageRootPath, primaryRepoPath, err := resolvePrimaryStoragePaths(storagePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(storageRootPath, 0755); err != nil {
		return fmt.Errorf("failed to create storage root path %s: %w", storageRootPath, err)
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

	// Strict mode: repository root must not be STORAGE_PATH itself.
	// Primary repository is always STORAGE_PATH/primary.
	if repocfg.IsRepositoryRoot(storageRootPath) {
		return fmt.Errorf("legacy repository detected at STORAGE_PATH root (%s); move repository to %s", storageRootPath, primaryRepoPath)
	}

	// If a repository already exists at the primary path, register it if needed.
	if repocfg.IsRepositoryRoot(primaryRepoPath) {
		// If it's already registered in DB, we're done
		if existing, err := repoManager.GetRepositoryByPath(primaryRepoPath); err == nil {
			logger.Info("primary storage already initialized",
				zap.String("operation", "repository.primary"),
				zap.String("repository_path", primaryRepoPath),
				zap.String("repository_id", repoUUIDString(existing.RepoID)),
			)
			return nil
		}

		// Otherwise, register the existing repository
		existingRepo, err := repoManager.AddRepository(primaryRepoPath)
		if err != nil {
			return fmt.Errorf("failed to register existing primary storage repository: %w", err)
		}

		logger.Info("primary storage registered",
			zap.String("operation", "repository.primary"),
			zap.String("repository_path", primaryRepoPath),
			zap.String("repository_id", repoUUIDString(existingRepo.RepoID)),
		)
		return nil
	}

	// Create repository configuration
	cfg := repocfg.NewRepositoryConfig(
		"Primary Storage",
		repocfg.WithStorageStrategy(storageStrategy),
		repocfg.WithLocalSettings(preserve, duplicateHandling, 0, false, false),
	)

	// Initialize a new repository with the configuration
	repo, err := repoManager.InitializeRepository(primaryRepoPath, *cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize primary storage repository: %w", err)
	}

	logger.Info("primary storage initialized",
		zap.String("operation", "repository.primary"),
		zap.String("repository_path", primaryRepoPath),
		zap.String("repository_id", repoUUIDString(repo.RepoID)),
		zap.String("storage_strategy", storageStrategy),
		zap.String("duplicate_handling", duplicateHandling),
		zap.Bool("preserve_filename", preserve),
	)

	return nil
}

func resolvePrimaryStoragePaths(storagePath string) (string, string, error) {
	trimmed := strings.TrimSpace(storagePath)
	if trimmed == "" {
		return "", "", fmt.Errorf("STORAGE_PATH environment variable is required")
	}

	storageRootPath, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return "", "", fmt.Errorf("invalid STORAGE_PATH %q: %w", storagePath, err)
	}

	primaryRepoPath := filepath.Join(storageRootPath, primaryRepositoryFolderName)
	return storageRootPath, primaryRepoPath, nil
}

func repoUUIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}
