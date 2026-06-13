// Package app contains the server bootstrap: it wires configuration, logging,
// storage, the job queue, ML services, and the HTTP router, then serves until
// the provided context is cancelled. It is invoked by the CLI entrypoint
// (server/cmd) and imported in-process by the desktop supervisor, so it must
// own its full lifecycle (startup and graceful shutdown) without calling
// os.Exit. Fatal startup conditions are returned as errors for the caller to
// handle.
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"server/config"
	"server/docs" // Import docs for swaggo
	"server/internal/agent/core"
	"server/internal/agent/pins"
	"server/internal/agent/ref"
	"server/internal/agent/tools"
	"server/internal/api"
	"server/internal/api/handler"
	"server/internal/cloud"
	"server/internal/db"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/processors"
	"server/internal/queue"
	"server/internal/queue/jobs"
	"server/internal/service"
	"server/internal/sourcing"
	"server/internal/storage"
	"server/internal/storage/scanner"
	"server/internal/utils/imaging"

	lumenconfig "github.com/edwinzhancn/lumen-sdk/pkg/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// shutdownTimeout bounds how long graceful shutdown (HTTP drain + queue drain)
// may take before the process exits anyway. Mirrors the desktop shutdown budget
// in docs/agent/exec-plans/active/desktop-wails-v3.md.
const shutdownTimeout = 10 * time.Second

// Run boots the API server from an already-resolved configuration and blocks
// until ctx is cancelled (e.g. SIGINT/SIGTERM for the CLI, or the desktop
// supervisor cancelling on app quit), then performs a graceful shutdown. It
// returns a non-nil error only on a fatal startup failure or an unexpected
// server error; a clean shutdown returns nil.
func Run(ctx context.Context, appConfig config.AppConfig) error {
	config.ApplyRuntimeEnvDefaults(appConfig)
	dbConfig := appConfig.DatabaseConfig

	return run(ctx, appConfig, dbConfig)
}

// RunFromEnvironment is a compatibility helper for hosts that still want the
// legacy behavior of loading .env/TOML/env inside the app package. New hosts
// should load config themselves and call Run with a typed AppConfig.
func RunFromEnvironment(ctx context.Context) error {
	config.LoadEnvironment()

	appConfig, err := config.LoadAppConfigWithError()
	if err != nil {
		return fmt.Errorf("load server configuration: %w", err)
	}
	config.ApplyRuntimeEnvDefaults(appConfig)
	dbConfig := appConfig.DatabaseConfig

	return run(ctx, appConfig, dbConfig)
}

func run(ctx context.Context, appConfig config.AppConfig, dbConfig config.DatabaseConfig) error {
	logRuntime, err := logging.NewLogger(logging.Config{
		Level:         appConfig.LoggingConfig.Level,
		LogDir:        appConfig.LoggingConfig.LogDir,
		ConsoleFormat: appConfig.LoggingConfig.ConsoleFormat,
		FileFormat:    appConfig.LoggingConfig.FileFormat,
		Development:   strings.EqualFold(appConfig.Environment, "development"),
	})
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	defer logRuntime.Sync()
	restoreStdLog := logging.RedirectStandardLog(logRuntime.Named("stdlib"))
	defer restoreStdLog()

	appLogger := logRuntime.Named("app")
	lumenLogger := logRuntime.Named("lumen")
	repositoryLogger := logRuntime.Named("repository")
	processorLogger := logRuntime.Named("processor")
	indexingLogger := logRuntime.Named("indexing")
	scannerLogger := logRuntime.Named("repository_scanner")
	repoAuditProvider := logging.NewRepositoryAuditProvider(logRuntime.Named("repo_audit"))

	appLogger.Info("starting Lumilio Photos API",
		zap.String("operation", "server.start"),
		zap.String("db_host", dbConfig.Host),
		zap.String("db_port", dbConfig.Port),
		zap.String("db_name", dbConfig.DBName),
	)

	// Derive a cancelable context from the caller's so shutdown can be triggered
	// both externally (ctx cancelled) and internally (defer cancel on return).
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Initialize libvips runtime once. ConcurrencyLevel=1 keeps libvips internal
	// thread pool disabled; outer parallelism is governed by River worker counts.
	imaging.StartVips()
	defer imaging.ShutdownVips()

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
		return fmt.Errorf("connect to database: %w", err)
	}
	defer database.Close()
	pgxPool := database.Pool
	queries := database.Queries

	settingsService := service.NewSettingsService(queries)
	if err := settingsService.EnsureInitialized(ctx); err != nil {
		return fmt.Errorf("initialize system settings: %w", err)
	}

	currentMLConfig, err := settingsService.GetMLConfig(ctx)
	if err != nil {
		return fmt.Errorf("load ML settings: %w", err)
	}
	if currentMLConfig.HasRuntimeDemand() {
		appLogger.Info("ML task processing enabled",
			zap.String("operation", "settings.ml"),
			zap.Bool("semantic_enabled", currentMLConfig.SemanticEnabled),
			zap.Bool("bioclip_enabled", currentMLConfig.BioCLIPEnabled),
			zap.Bool("ocr_enabled", currentMLConfig.OCREnabled),
			zap.Bool("face_enabled", currentMLConfig.FaceEnabled),
		)
	}

	// Initialize new repository-based storage system
	repoManager, err := storage.NewRepositoryManager(queries, repositoryLogger, repoAuditProvider)
	if err != nil {
		return fmt.Errorf("initialize repository manager: %w", err)
	}
	stagingManager := storage.NewStagingManager()
	appLogger.Info("repository storage system initialized", zap.String("operation", "repository.init"))
	// Initialize primary storage repository
	appLogger.Info("initializing primary storage repository", zap.String("operation", "repository.primary"))
	if err := initPrimaryStorage(repoManager, repositoryLogger, appConfig.StorageConfig); err != nil {
		return fmt.Errorf("initialize primary storage: %w", err)
	}

	workers := river.NewWorkers()
	queueClient, err := queue.New(pgxPool, workers, logRuntime.RiverLogger())
	if err != nil {
		return fmt.Errorf("initialize queue: %w", err)
	}
	faceService := service.NewFaceService(queries, repoManager, pgxPool)

	lumenService, embeddingService, classifierService, err := initMLServices(ctx, pgxPool, queries, workers, appLogger, lumenLogger, settingsService, faceService)
	if err != nil {
		return fmt.Errorf("initialize ML services: %w", err)
	}

	defer func() {
		if lumenService != nil {
			if err := lumenService.Close(); err != nil {
				lumenLogger.Warn("failed to close lumen service", zap.String("operation", "lumen.close"), zap.Error(err))
			}
		}
	}()

	assetService, err := service.NewAssetService(queries, pgxPool, lumenService, &repoManager, embeddingService, appLogger.Named("asset_service"))
	if err != nil {
		return fmt.Errorf("initialize asset service: %w", err)
	}
	locationService := service.NewLocationService(queries, pgxPool)
	speciesReferenceService := service.NewSpeciesReferenceService()
	indexingService := service.NewAssetIndexingService(queries, settingsService, lumenService, queueClient, pgxPool, indexingLogger, repoAuditProvider)
	stackService := service.NewStackService(queries, pgxPool, appLogger.Named("stack"), repoAuditProvider)
	duplicateService := service.NewDuplicateService(queries, pgxPool, appLogger.Named("duplicate"), assetService)
	authService := service.NewAuthService(queries, pgxPool)
	albumService := service.NewAlbumService(queries)
	userService := service.NewUserService(queries, pgxPool)

	// Break-glass recovery: when LUMILIO_BREAK_GLASS is set, reset a locked-out
	// admin (oldest active admin, or LUMILIO_BREAK_GLASS_USERNAME) to a random
	// temporary password and clear all MFA factors, printing the password once.
	runBreakGlassIfRequested(ctx, userService, appLogger)

	// Initialize Agent Service. The ref store is shared between the agent
	// tool chain and the hydration API handler; its janitor bounds memory
	// for abandoned sessions.
	refStore := ref.NewMemoryStore(ref.DefaultTTL, ref.DefaultMaxRefsPerScope)
	go refStore.RunJanitor(ctx, 10*time.Minute)
	conversations := core.NewConversationStore(core.DefaultConversationTTL)
	go conversations.RunJanitor(ctx, 10*time.Minute)
	agentService := core.NewAgentService(queries, settingsService, refStore, assetService, conversations)
	agentPins := pins.NewService(queries, refStore, assetService)
	appLogger.Info("agent service initialized", zap.String("operation", "agent.init"))

	// Register agent tools
	tools.RegisterAll()
	appLogger.Info("agent tools registered", zap.String("operation", "agent.tools"))

	// Initialize SourceMaterializer (unified ingest entry point for upload, scan, cloud sync)
	sourceMaterializer := sourcing.NewSourceMaterializer(queries, stagingManager, queueClient, assetService, processorLogger, repoAuditProvider)

	assetProcessor := processors.NewAssetProcessor(assetService, queries, repoManager, stagingManager, sourceMaterializer, queueClient, settingsService, embeddingService, lumenService, processorLogger, repoAuditProvider)
	repositoryScanner := scanner.NewScanner(queries, queueClient, appConfig.RepositoryScan, scannerLogger)
	river.AddWorker[queue.IngestAssetArgs](workers, &queue.IngestAssetWorker{Processor: assetProcessor})
	river.AddWorker[queue.DiscoverAssetArgs](workers, &queue.DiscoverAssetWorker{ProcessDiscover: assetProcessor.ProcessDiscoveredAsset})
	river.AddWorker[queue.MetadataArgs](workers, &queue.MetadataWorker{Process: assetProcessor.ProcessMetadataTask})
	river.AddWorker[queue.ThumbnailArgs](workers, &queue.ThumbnailWorker{Process: assetProcessor.ProcessThumbnailTask})
	river.AddWorker[queue.TranscodeArgs](workers, &queue.TranscodeWorker{Process: assetProcessor.ProcessTranscodeTask})
	river.AddWorker[queue.AssetRetryArgs](workers, &queue.AssetRetryWorker{ProcessRetry: assetProcessor.ProcessRetryTask})
	river.AddWorker[queue.ReindexAssetsArgs](workers, &queue.ReindexAssetsWorker{IndexingService: indexingService})
	river.AddWorker[queue.RebuildLocationClustersArgs](workers, &queue.RebuildLocationClustersWorker{LocationService: locationService})
	river.AddWorker[queue.ScanRepositoryArgs](workers, &queue.ScanRepositoryWorker{ProcessScan: repositoryScanner.ProcessScanRepository})
	river.AddWorker[queue.DetectStacksArgs](workers, &queue.DetectStacksWorker{StackService: stackService})
	river.AddWorker[queue.LivePhotoMatchArgs](workers, &queue.LivePhotoMatchWorker{StackService: stackService})
	river.AddWorker[queue.ProcessPHashArgs](workers, &queue.ProcessPHashWorker{
		Queries:          queries,
		EmbeddingService: embeddingService,
	})
	river.AddWorker[queue.ScheduleRepositoryScansArgs](workers, &queue.ScheduleRepositoryScansWorker{
		EnqueueAll: repositoryScanner.EnqueueAllPeriodicScans,
	})

	// River's Start runs the client in a background goroutine until Stop is
	// called; it returns once startup completes. context.Background is used (not
	// the run context) so a shutdown signal triggers a graceful drain via Stop
	// rather than an abrupt cancellation of in-flight jobs.
	if err := queueClient.Start(context.Background()); err != nil {
		return fmt.Errorf("start queue client: %w", err)
	}
	appLogger.Info("queues initialized successfully", zap.String("operation", "queue.init"))

	// --- Periodic Jobs (River PeriodicJobs) ---
	// Must be registered after Start() — the periodic job enqueuer is
	// initialized during Start.
	if appConfig.RepositoryScan.Enabled {
		queueClient.PeriodicJobs().Add(river.NewPeriodicJob(
			river.PeriodicInterval(time.Duration(appConfig.RepositoryScan.IntervalSeconds)*time.Second),
			func() (river.JobArgs, *river.InsertOpts) {
				return jobs.ScheduleRepositoryScansArgs{}, nil
			},
			&river.PeriodicJobOpts{ID: "repository_scan", RunOnStart: true},
		))
	}

	// Initialize controllers with new storage system
	assetController := handler.NewAssetHandler(assetService, authService, indexingService, stackService, queries, repoManager, stagingManager, queueClient, settingsService, lumenService)
	assetController.StartCleanupTasks(ctx)
	authController := handler.NewAuthHandler(authService)
	setupController := handler.NewSetupHandler(service.NewSetupService(dbConfig))
	albumController := handler.NewAlbumHandler(&albumService, queries, queueClient, settingsService, lumenService)
	peopleController := handler.NewPeopleHandler(assetService, faceService, authService, repoManager)
	locationController := handler.NewLocationHandler(locationService, queueClient)
	speciesController := handler.NewSpeciesHandler(speciesReferenceService)
	userController := handler.NewUserHandler(userService)
	queueController := handler.NewQueueHandler(pgxPool)
	statsController := handler.NewStatsHandler(queries)
	agentController := handler.NewAgentHandler(agentService, refStore, queries, agentPins)
	capabilitiesController := handler.NewCapabilitiesHandler(settingsService, lumenService)
	settingsController := handler.NewSettingsHandler(settingsService)
	classifierController := handler.NewClassifierHandler(classifierService)
	// Initialize Cloud Sync service and handler
	cloudSyncService := cloud.NewCloudSyncService(queries, sourceMaterializer, appLogger.Named("cloud_sync"))
	// Reconcile import runs left "running"/"queued" by a previous crash/restart
	// so repositories are not stuck with an import that never finishes.
	if err := cloudSyncService.RecoverInterruptedRuns(ctx); err != nil {
		appLogger.Warn("failed to recover interrupted cloud import runs", zap.Error(err))
	}
	cloudController := handler.NewCloudHandler(cloudSyncService)
	repositoryScanController := handler.NewRepositoryScanHandler(repositoryScanner, repoManager, appConfig.StorageConfig.Path, cloudSyncService)
	duplicateController := handler.NewDuplicateHandler(duplicateService, queries)

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
		setupController,
		albumController,
		peopleController,
		locationController,
		speciesController,
		queueController,
		statsController,
		agentController,
		capabilitiesController,
		settingsController,
		classifierController,
		userController,
		repositoryScanController,
		duplicateController,
		cloudController,
		handler.RequireLLMAgentEnabled(settingsService),
		appLogger.Named("http"),
	)

	// Add Swagger documentation endpoint
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Optionally serve the SPA bundle (desktop sets server.web_root; docker/web
	// leave it empty and serve the bundle from a separate static server).
	api.RegisterSPA(router, appConfig.ServerConfig.WebRoot)

	srv := &http.Server{
		Addr:    ":" + appConfig.ServerConfig.Port,
		Handler: router,
	}

	appLogger.Info("server starting",
		zap.String("operation", "server.listen"),
		zap.String("port", appConfig.ServerConfig.Port),
		zap.String("swagger_url", fmt.Sprintf("http://localhost:%s/swagger/index.html", appConfig.ServerConfig.Port)),
		zap.String("health_url", fmt.Sprintf("http://localhost:%s/api/v1/health", appConfig.ServerConfig.Port)),
	)

	// Serve in a goroutine so this function can block on ctx and drive a
	// graceful shutdown when it is cancelled.
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		appLogger.Info("shutdown signal received, draining", zap.String("operation", "server.shutdown"))
	}

	// Graceful shutdown: stop accepting HTTP connections and drain in-flight
	// jobs, both bounded by shutdownTimeout. Remaining resources (scheduler,
	// lumen, database, libvips, logger) are released by the deferred cleanups.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		appLogger.Warn("http server shutdown error", zap.String("operation", "server.shutdown"), zap.Error(err))
	}
	if err := queueClient.Stop(shutdownCtx); err != nil {
		appLogger.Warn("queue client shutdown error", zap.String("operation", "queue.shutdown"), zap.Error(err))
	}
	appLogger.Info("shutdown complete", zap.String("operation", "server.shutdown"))
	return nil
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
) (service.LumenService, service.EmbeddingService, service.ClassifierService, error) {
	appLogger.Info("initializing ML services", zap.String("operation", "ml.init"))

	lumenCfg, err := lumenconfig.LoadConfig("")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load lumen sdk config: %w", err)
	}

	lumenService, err := service.NewLumenService(lumenCfg, lumenLogger)
	if err != nil {
		return nil, nil, nil, err
	}

	err = lumenService.Start(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	appLogger.Info("lumen service initialized", zap.String("operation", "ml.init"))

	embeddingService := service.NewEmbeddingService(queries, pgxPool)
	speciesService := service.NewSpeciesService(queries)
	ocrService := service.NewOCRService(queries)
	imageLoader := queue.NewDBMLImageLoader(queries)

	river.AddWorker[queue.ProcessSemanticArgs](workers, &queue.ProcessSemanticWorker{
		LumenService:     lumenService,
		EmbeddingService: embeddingService,
		ConfigProvider:   settingsService,
		ImageLoader:      imageLoader,
	})
	appLogger.Info("semantic service and worker registered", zap.String("operation", "ml.init"))

	river.AddWorker[queue.ProcessBioClipArgs](workers, &queue.ProcessBioClipWorker{
		LumenService:   lumenService,
		SpeciesService: speciesService,
		ConfigProvider: settingsService,
		ImageLoader:    imageLoader,
	})
	appLogger.Info("BioCLIP service and worker registered", zap.String("operation", "ml.init"))

	river.AddWorker[queue.ProcessOcrArgs](workers, &queue.ProcessOcrWorker{
		OCRService:     ocrService,
		LumenService:   lumenService,
		ConfigProvider: settingsService,
		ImageLoader:    imageLoader,
	})
	appLogger.Info("OCR service and worker registered", zap.String("operation", "ml.init"))

	river.AddWorker[queue.ProcessFaceArgs](workers, &queue.ProcessFaceWorker{
		FaceService:    faceService,
		LumenService:   lumenService,
		ConfigProvider: settingsService,
		ImageLoader:    imageLoader,
	})
	appLogger.Info("face service and worker registered", zap.String("operation", "ml.init"))

	aiTagService := service.NewAIGeneratedTagService(queries)
	classifierService := service.NewClassifierService(pgxPool, lumenService, embeddingService, appLogger.Named("classifier"))
	river.AddWorker[queue.ZeroshotClassifyArgs](workers, &queue.ZeroshotClassifyWorker{
		EmbeddingService:  embeddingService,
		ClassifierService: classifierService,
		AITagService:      aiTagService,
		ConfigProvider:    settingsService,
	})
	appLogger.Info("zero-shot classifier service and worker registered", zap.String("operation", "ml.init"))

	// Build classifier prototypes in the background once the semantic text-embed
	// task is reachable, so startup never blocks on ML node availability.
	go buildClassifierPrototypes(lumenService, classifierService, appLogger.Named("classifier"))

	appLogger.Info("ML services initialization complete", zap.String("operation", "ml.init"))
	return lumenService, embeddingService, classifierService, nil
}

// buildClassifierPrototypes waits (bounded) for the semantic text-embed task to
// become available, then builds/refreshes classifier prototypes. It is a no-op
// when semantic classification is disabled (no enabled definitions are scored).
func buildClassifierPrototypes(lumenService service.LumenService, classifierService service.ClassifierService, logger *zap.Logger) {
	for i := 0; i < 30; i++ {
		if lumenService != nil && lumenService.IsTaskAvailable("semantic_text_embed") {
			break
		}
		time.Sleep(10 * time.Second)
	}
	buildCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := classifierService.EnsurePrototypes(buildCtx); err != nil {
		logger.Warn("failed to build classifier prototypes", zap.Error(err))
	}
}
