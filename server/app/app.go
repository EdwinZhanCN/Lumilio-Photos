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
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"server/config"
	"server/docs" // Import docs for swaggo
	"server/internal/agent/core"
	"server/internal/agent/pins"
	"server/internal/agent/ref"
	"server/internal/agent/tools"
	"server/internal/api"
	"server/internal/api/dto"
	"server/internal/api/handler"
	"server/internal/cloud"
	"server/internal/db"
	dbbackup "server/internal/db/backup"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/event"
	"server/internal/httporigin"
	"server/internal/logging"
	"server/internal/processors"
	"server/internal/queue"
	"server/internal/queue/jobs"
	"server/internal/search/bleveocr"
	"server/internal/servertransport"
	"server/internal/service"
	"server/internal/settings"
	"server/internal/sourcing"
	"server/internal/storage"
	"server/internal/storage/scanner"
	"server/internal/utils/imaging"
	"server/internal/version"

	"github.com/riverqueue/river"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

// Each subsystem owns an independent shutdown budget. In particular, time used
// by HTTP draining never reduces River's opportunity to relinquish SQLite.
const (
	shutdownTimeout  = 10 * time.Second
	queueStopTimeout = 10 * time.Second
	queueKillTimeout = 10 * time.Second
)

var errSQLiteOwnershipRetained = errors.New("SQLite ownership was not relinquished")

// OperatorControls are explicit, single-run host controls. They do not modify
// AppConfig and are never read from the environment inside the application.
type OperatorControls struct {
	PprofAddr                    string
	AgentAuditLogPath            string
	AgentRefUserHotBudgetBytes   int64
	AgentRefGlobalHotBudgetBytes int64
	BreakGlass                   bool
	BreakGlassUsername           string
	// RepositoryManagerReady exposes the in-process repository control plane to
	// the Desktop host. Standalone leaves it nil; no HTTP path or secret is
	// created by this hook.
	RepositoryManagerReady func(RepositoryControl)
	// RuntimeReady is called once, after the listener, migrations, and core
	// workers are available. Desktop uses this typed handoff instead of probing
	// its own loopback listener to establish generation ownership.
	RuntimeReady func(RuntimeInfo)
}

// RuntimeInfo describes the listener published by one server generation. It
// intentionally contains no mutable server handles; RepositoryManagerReady is
// the separate handoff for the in-process storage control plane.
type RuntimeInfo struct {
	Listen     string
	ProductURL string
}

// Run boots the API server from an already-resolved configuration and blocks
// until ctx is cancelled (e.g. SIGINT/SIGTERM for the CLI, or the desktop
// supervisor cancelling on app quit), then performs a graceful shutdown. It
// returns a non-nil error only on a fatal startup failure or an unexpected
// server error; a clean shutdown returns nil.
func Run(ctx context.Context, appConfig config.AppConfig, controls OperatorControls) error {
	if !appConfig.LoadedFromManifest() {
		return errors.New("app config was not produced by the strict manifest loader")
	}
	dbConfig := appConfig.DatabaseConfig
	originPolicy, err := httporigin.New(appConfig.ServerConfig, appConfig.Auth.Passkey)
	if err != nil {
		return fmt.Errorf("initialize origin policy: %w", err)
	}

	pprofHost, err := startPprofHost(strings.TrimSpace(controls.PprofAddr))
	if err != nil {
		return err
	}
	if pprofHost != nil {
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()
			_ = pprofHost.shutdown(shutdownCtx)
		}()
	}

	// govips owns process-global libvips state and cannot be restarted after a
	// shutdown. Keep it alive across in-process SQLite restore generations and
	// repeated embedded Run calls; process exit releases the native runtime.
	imaging.StartVips()

	forceOCRRebuild := false
	for {
		if _, err := dbbackup.ApplyPendingRestore(context.WithoutCancel(ctx), dbConfig.Path, nil); err != nil {
			_ = dbbackup.FailPendingRestoreOperation(
				dbConfig.Path,
				"restore_install_failed",
				"Restore could not be installed. Review server logs and restart to retry.",
			)
			return fmt.Errorf("apply pending SQLite restore: %w", err)
		}
		appliedRestore := dbbackup.HasAppliedRestore(dbConfig.Path)

		generationCtx, cancelGeneration := context.WithCancel(ctx)
		var restartRequested atomic.Bool
		requestRestart := func() {
			restartRequested.Store(true)
			cancelGeneration()
		}
		err := run(
			generationCtx,
			appConfig,
			dbConfig,
			originPolicy,
			controls,
			requestRestart,
			appliedRestore || forceOCRRebuild,
		)
		forceOCRRebuild = false
		cancelGeneration()
		if err != nil {
			if dbbackup.HasAppliedRestore(dbConfig.Path) && !errors.Is(err, errSQLiteOwnershipRetained) {
				rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), shutdownTimeout)
				rollbackErr := dbbackup.RollbackPendingRestoreWithCause(
					rollbackCtx,
					dbConfig.Path,
					nil,
					"restore_runtime_failed",
					"The restored database did not pass runtime verification. The previous database was restored.",
				)
				rollbackCancel()
				if rollbackErr != nil {
					_ = dbbackup.FailPendingRestoreOperation(
						dbConfig.Path,
						"restore_rollback_failed",
						"Restore failed and automatic rollback could not be completed. Review server logs before restarting.",
					)
					return errors.Join(err, fmt.Errorf("rollback failed SQLite restore: %w", rollbackErr))
				}
				if ctx.Err() == nil {
					forceOCRRebuild = true
					continue
				}
			}
			return err
		}
		if restartRequested.Load() && ctx.Err() == nil {
			continue
		}
		return nil
	}
}

func run(
	ctx context.Context,
	appConfig config.AppConfig,
	dbConfig config.DatabaseConfig,
	originPolicy *httporigin.Policy,
	controls OperatorControls,
	requestRestart func(),
	forceOCRRebuild bool,
) (runErr error) {
	agentRefUserBudget := controls.AgentRefUserHotBudgetBytes
	if agentRefUserBudget <= 0 {
		agentRefUserBudget = ref.DefaultUserHotBudget
	}
	agentRefGlobalBudget := controls.AgentRefGlobalHotBudgetBytes
	if agentRefGlobalBudget <= 0 {
		agentRefGlobalBudget = ref.DefaultGlobalHotBudget
	}
	if agentRefGlobalBudget < agentRefUserBudget {
		return errors.New("global Agent ref hot-memory budget must be greater than or equal to the per-user budget")
	}

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
	defer logRuntime.Close()
	restoreStdLog := logging.RedirectStandardLog(logRuntime.Named("stdlib"))
	defer restoreStdLog()

	appLogger := logRuntime.Named("app")
	securityLogger := logRuntime.Security()
	lumenLogger := logRuntime.Named("lumen")
	repositoryLogger := logRuntime.Named("repository")
	processorLogger := logRuntime.Named("processor")
	indexingLogger := logRuntime.Named("indexing")
	scannerLogger := logRuntime.Named("repository_scanner")
	repoAuditProvider := logging.NewRepositoryAuditProvider(logRuntime.Named("repo_audit"), appConfig.LoggingConfig.RepositoryAuditVerbose)
	defer func() {
		if err := repoAuditProvider.Close(); err != nil {
			appLogger.Warn("failed to close repository audit logs", zap.Error(err))
		}
	}()

	appLogger.Info("starting Lumilio Photos API",
		zap.String("operation", "server.start"),
		zap.String("db_path", dbConfig.Path),
		zap.String("config_path", appConfig.ManifestPath),
		zap.Int("config_schema_version", appConfig.SchemaVersion),
		zap.String("config_sha256", appConfig.ManifestSHA256),
	)

	// Derive a cancelable context from the caller's so shutdown can be triggered
	// both externally (ctx cancelled) and internally (defer cancel on return).
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Ensure the default media root and explicitly separate private cloud/backup
	// directories exist before any service reads them.
	if err := storage.EnsureRootLayout(appConfig.StorageConfig); err != nil {
		return fmt.Errorf("ensure storage layout: %w", err)
	}

	database, err := db.Open(ctx, dbConfig)
	if err != nil {
		return fmt.Errorf("open SQLite database: %w", err)
	}
	databaseCloseAllowed := true
	defer func() {
		if !databaseCloseAllowed {
			runErr = errors.Join(
				runErr,
				fmt.Errorf("%w: database left open and restore swap blocked", errSQLiteOwnershipRetained),
			)
			return
		}
		closeCtx, closeCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer closeCancel()
		if err := database.Close(closeCtx); err != nil {
			appLogger.Warn("SQLite close maintenance failed", zap.Error(err))
			runErr = errors.Join(
				runErr,
				fmt.Errorf("%w: close SQLite database: %v", errSQLiteOwnershipRetained, err),
			)
		}
	}()

	if err := database.Migrate(ctx); err != nil {
		appLogger.Error("failed to run migrations automatically",
			zap.String("operation", "database.migrate"),
			zap.Error(err),
		)
		return fmt.Errorf("run database migrations: %w", err)
	}
	sqlDB := database.SQL
	queries := database.Queries
	ocrIndex, err := bleveocr.Open(
		ctx,
		database.Path,
		queries,
		forceOCRRebuild,
		appLogger.Named("ocr_index"),
	)
	if err != nil {
		return fmt.Errorf("initialize OCR Bleve index: %w", err)
	}
	defer func() {
		if err := ocrIndex.Close(); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}()
	ocrIndexWriter := bleveocr.NewWriter(sqlDB, queries, ocrIndex)
	ocrIndexTrigger := bleveocr.NewOutboxTrigger()

	// River must exist before settings construction: geocoding settings writes
	// reset location projections and insert the revisioned resolver job through
	// one SQLite transaction.
	workers := river.NewWorkers()
	queueClient, err := queue.New(sqlDB, workers, logRuntime.RiverLogger())
	if err != nil {
		return fmt.Errorf("initialize queue: %w", err)
	}
	settingsService := service.NewSettingsServiceWithRuntime(
		queries,
		settings.Default(appConfig.Environment),
		appConfig.Auth.SecretKeyFile,
		service.SettingsRuntime{DB: sqlDB, Queue: queueClient},
	)
	if err := settingsService.EnsureInitialized(ctx); err != nil {
		return fmt.Errorf("initialize system settings: %w", err)
	}

	// Single source of truth for first-run bootstrap progress. Reconcile at boot
	// so the cached phase reflects the current gates.
	bootstrapService := service.NewBootstrapService(queries)
	if phase, err := bootstrapService.Reconcile(ctx); err != nil {
		appLogger.Warn("failed to reconcile bootstrap phase", zap.Error(err))
	} else {
		appLogger.Info("bootstrap phase", zap.String("operation", "bootstrap.reconcile"), zap.String("phase", phase))
	}

	currentMLConfig, err := settingsService.GetMLConfig(ctx)
	if err != nil {
		return fmt.Errorf("load ML settings: %w", err)
	}
	if currentMLConfig.HasRuntimeDemand() {
		appLogger.Info("ML task processing enabled",
			zap.String("operation", "settings.ml"),
			zap.Bool("semantic_enabled", currentMLConfig.SemanticEnabled),
			zap.Bool("video_semantic_enabled", currentMLConfig.VideoSemanticEnabled),
			zap.Bool("bioclip_enabled", currentMLConfig.BioCLIPEnabled),
			zap.Bool("ocr_enabled", currentMLConfig.OCREnabled),
			zap.Bool("face_enabled", currentMLConfig.FaceEnabled),
		)
	}

	// Initialize new repository-based storage system
	repositoryAccess := storage.NewRepositoryAccessCoordinator()
	repositoryFiles := storage.NewRepositoryFSFactory(repositoryAccess, queries)
	repoManager, err := storage.NewRepositoryManager(sqlDB, queries, repositoryLogger, repoAuditProvider, repositoryFiles)
	if err != nil {
		return fmt.Errorf("initialize repository manager: %w", err)
	}
	// Claim every currently reachable portable identity before lifecycle/host
	// recovery or reconciliation can inspect and mutate its filesystem state.
	releaseStorageOwnership, err := repoManager.AcquireRuntimeStorageOwnership(ctx)
	if err != nil {
		return fmt.Errorf("claim portable storage ownership: %w", err)
	}
	defer releaseStorageOwnership()
	if err := repoManager.RecoverLifecycleOperations(ctx); err != nil {
		return fmt.Errorf("recover storage lifecycle operations: %w", err)
	}
	if err := repoManager.RecoverHostActions(ctx); err != nil {
		return fmt.Errorf("recover native host actions: %w", err)
	}
	defaultRoot, degradedStorage, err := ensureDefaultStorageForRuntime(ctx, repoManager, appConfig.StorageConfig.Path)
	if err != nil {
		return err
	}
	if degradedStorage {
		appLogger.Warn("default Storage Location requires recovery; continuing in degraded mode",
			zap.String("operation", "repository_root.recovery_required"),
			zap.String("path", appConfig.StorageConfig.Path))
	} else {
		appLogger.Info("default storage location initialized",
			zap.String("operation", "repository_root.init"),
			zap.String("path", defaultRoot.Path),
		)
	}
	stagingManager := storage.NewStagingManager(repositoryFiles)
	appLogger.Info("repository storage system initialized", zap.String("operation", "repository.init"))

	// Drives get unplugged, remounted, and replaced while the server is down.
	// Re-check every repository's recorded path before anything schedules work
	// against it. Unreachable repositories become offline rather than failing
	// mid-scan.
	if err := repoManager.ReconcileRepositoryRoots(ctx); err != nil {
		appLogger.Warn("failed to reconcile Storage Locations", zap.Error(err))
	}
	if err := repoManager.ReconcileAll(ctx); err != nil {
		appLogger.Warn("failed to reconcile repositories", zap.Error(err))
	}
	startStorageReconciler(ctx, repoManager, time.Minute, appLogger)
	if err := repoManager.ReconcileRepositoryCapacity(ctx); err != nil {
		appLogger.Warn("failed to reconcile repository capacity", zap.Error(err))
	}
	go monitorRepositoryCapacity(ctx, repoManager, appLogger.Named("storage_capacity"))
	eventService := event.NewService(sqlDB, queueClient)
	river.AddWorker[queue.EventRebuildArgs](workers, &queue.EventRebuildWorker{
		DB: sqlDB, Service: eventService,
	})
	river.AddWorker[queue.ScheduleEventRebuildsArgs](workers, &queue.ScheduleEventRebuildsWorker{
		DB: sqlDB,
	})
	river.AddWorker[queue.ProcessOCROutboxArgs](workers, &queue.ProcessOCROutboxWorker{
		Writer: ocrIndexWriter,
	})
	faceService := service.NewFaceService(queries, repositoryFiles, sqlDB)

	lumenService, embeddingService, classifierService, err := initMLServices(ctx, appConfig, sqlDB, queries, workers, appLogger, lumenLogger, settingsService, faceService, repositoryFiles, ocrIndexTrigger)
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

	assetService, err := service.NewAssetServiceWithQueue(
		queries,
		sqlDB,
		lumenService,
		embeddingService,
		ocrIndex,
		queueClient,
		ocrIndexTrigger,
		appLogger.Named("asset_service"),
	)
	if err != nil {
		return fmt.Errorf("initialize asset service: %w", err)
	}
	locationService := service.NewLocationService(queries, sqlDB, queueClient)
	speciesReferenceService := service.NewSpeciesReferenceService()
	indexingService := service.NewAssetIndexingService(queries, settingsService, lumenService, queueClient, sqlDB, indexingLogger, repoAuditProvider, repositoryFiles)
	stackService := service.NewStackServiceWithQueue(queries, sqlDB, appLogger.Named("stack"), repoAuditProvider, queueClient)
	duplicateService := service.NewDuplicateService(queries, sqlDB, appLogger.Named("duplicate"), assetService)
	authService, err := service.NewAuthService(queries, sqlDB, appConfig.Auth, appLogger.Named("auth"), securityLogger)
	if err != nil {
		return fmt.Errorf("initialize auth service: %w", err)
	}
	authRateLimiter, err := handler.NewAuthRateLimiter(appConfig.Auth.RateLimit, securityLogger)
	if err != nil {
		return fmt.Errorf("initialize auth rate limiter: %w", err)
	}
	albumService := service.NewAlbumService(queries)
	userService := service.NewUserService(queries, sqlDB)

	// Break-glass recovery is an explicit single-run host control, separate from
	// immutable AppConfig.
	runBreakGlassIfRequested(ctx, userService, controls.BreakGlass, controls.BreakGlassUsername, securityLogger)

	// Initialize Agent Service. The ref store is shared between the agent
	// tool chain and the hydration API handler; its janitor bounds memory
	// for abandoned sessions.
	authorizedLibraries := core.NewAuthorizedLibraryFactory(queries, assetService, sqlDB)
	refStore := ref.NewPersistentStore(
		queries,
		authorizedLibraries,
		ref.DefaultTTL,
		ref.DefaultMaxRefsPerScope,
		agentRefUserBudget,
		agentRefGlobalBudget,
	)
	go refStore.RunJanitor(ctx, 10*time.Minute)
	conversations := core.NewConversationStore(core.DefaultConversationTTL)
	go conversations.RunJanitor(ctx, 10*time.Minute)
	agentService := core.NewAgentService(queries, sqlDB, settingsService, refStore, authorizedLibraries, conversations, controls.AgentAuditLogPath)
	agentPins := pins.NewService(queries, refStore, authorizedLibraries)
	appLogger.Info("agent service initialized", zap.String("operation", "agent.init"))

	// Share links reuse the same asset-set-source query path pins use
	// (resolveSourceAssetIDs -> AssetService.QueryAssets / agentPins.AssetIDs),
	// so it's constructed here once both dependencies exist.
	shareLinkService := service.NewShareLinkService(queries, assetService, agentPins, appConfig.Auth.SecretKeyFile)

	// Register agent tools
	tools.RegisterAll()
	appLogger.Info("agent tools registered", zap.String("operation", "agent.tools"))

	// Initialize SourceMaterializer (unified ingest entry point for upload, scan, cloud sync)
	sourceMaterializer := sourcing.NewSourceMaterializer(database, stagingManager, queueClient, processorLogger, repoAuditProvider, repositoryFiles)
	sourceMaterializer.SetCapacityGuard(repoManager)

	assetProcessor := processors.NewAssetProcessor(sqlDB, assetService, queries, sourceMaterializer, queueClient, settingsService, embeddingService, lumenService, appConfig.Transcode, appConfig.Tools, processorLogger, repoAuditProvider, repositoryFiles, repoManager)
	repositoryScanner := scanner.NewScanner(database, queueClient, repositoryFiles, repoManager, appConfig.RepositoryScan, scannerLogger)
	repoManager.SetInitialScanEnqueuer(func(ctx context.Context, repositoryID string) error {
		_, err := repositoryScanner.EnqueueManualScan(ctx, repositoryID, "storage_lifecycle", true)
		return err
	})
	if err := repoManager.RetryPendingInitialRepositoryScans(ctx); err != nil {
		return fmt.Errorf("resume pending initial repository scans: %w", err)
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if retryErr := repoManager.RetryPendingInitialRepositoryScans(ctx); retryErr != nil {
					appLogger.Warn("failed to retry pending initial repository scans", zap.Error(retryErr))
				}
			}
		}
	}()
	if controls.RepositoryManagerReady != nil {
		controls.RepositoryManagerReady(newRepositoryControl(repoManager))
		defer controls.RepositoryManagerReady(nil)
	}
	river.AddWorker[queue.IngestAssetArgs](workers, &queue.IngestAssetWorker{Processor: assetProcessor})
	river.AddWorker[queue.DiscoverAssetArgs](workers, &queue.DiscoverAssetWorker{ProcessDiscover: assetProcessor.ProcessDiscoveredAsset})
	river.AddWorker[queue.MetadataArgs](workers, &queue.MetadataWorker{Process: assetProcessor.ProcessMetadataTask})
	river.AddWorker[queue.ThumbnailArgs](workers, &queue.ThumbnailWorker{Process: assetProcessor.ProcessThumbnailTask})
	river.AddWorker[queue.TranscodeArgs](workers, &queue.TranscodeWorker{Process: assetProcessor.ProcessTranscodeTask})
	river.AddWorker[queue.ProcessVideoFramesArgs](workers, &queue.ProcessVideoFramesWorker{Process: assetProcessor.ProcessVideoFramesTask})
	river.AddWorker[queue.AssetRetryArgs](workers, &queue.AssetRetryWorker{ProcessRetry: assetProcessor.ProcessRetryTask})
	river.AddWorker[queue.ReindexAssetsArgs](workers, &queue.ReindexAssetsWorker{IndexingService: indexingService})
	river.AddWorker[queue.RebuildLocationClustersArgs](workers, &queue.RebuildLocationClustersWorker{LocationService: locationService})
	river.AddWorker[queue.ResolveLocationClustersArgs](workers, &queue.ResolveLocationClustersWorker{LocationService: locationService})
	river.AddWorker[queue.ScanRepositoryArgs](workers, &queue.ScanRepositoryWorker{ProcessScan: repositoryScanner.ProcessScanRepository})
	river.AddWorker[queue.DetectStacksArgs](workers, &queue.DetectStacksWorker{StackService: stackService})
	river.AddWorker[queue.LivePhotoMatchArgs](workers, &queue.LivePhotoMatchWorker{StackService: stackService})
	river.AddWorker[queue.ProcessPHashArgs](workers, &queue.ProcessPHashWorker{
		Queries:          queries,
		EmbeddingService: embeddingService,
		Files:            repositoryFiles,
	})
	river.AddWorker[queue.ScheduleRepositoryScansArgs](workers, &queue.ScheduleRepositoryScansWorker{
		EnqueueAll: repositoryScanner.EnqueueAllPeriodicScans,
	})

	// Automatic database backups use their explicit private destination rather
	// than following any removable repository root. Policy
	// (enabled/interval/retention) is read from runtime settings on every tick,
	// so the periodic job below can stay a fixed hourly heartbeat.
	backupLogger := appLogger.Named("db_backup").Sugar()
	catalogInfo, err := db.InspectCatalog(ctx, database.Path)
	if err != nil {
		return fmt.Errorf("inspect SQLite catalog for backup runtime: %w", err)
	}
	snapshotMetadata := dbbackup.SnapshotMetadata{
		AppVersion:          version.Version,
		ConfigSchemaVersion: appConfig.SchemaVersion,
	}
	snapshotCompatibility := dbbackup.Compatibility{
		LibraryID:               catalogInfo.LibraryID,
		ConfigSchemaVersion:     appConfig.SchemaVersion,
		MaxApplicationMigration: catalogInfo.ApplicationMigration,
		MaxRiverMigration:       catalogInfo.RiverMigration,
	}
	backupScheduler := &dbbackup.Scheduler{
		Source:   sqlDB,
		Dir:      appConfig.StorageConfig.BackupsDir(),
		Metadata: snapshotMetadata,
		Ready:    bootstrapService.IsReady,
		Settings: settingsService.GetBackupConfig,
		Logf:     func(format string, args ...any) { backupLogger.Infof(format, args...) },
	}
	river.AddWorker[queue.DatabaseBackupArgs](workers, &queue.DatabaseBackupWorker{Run: backupScheduler.Run})

	// Admin backup surface (list/trigger/download/delete/restore). Restore only
	// stages a validated snapshot here; the generation loop drains and closes
	// this runtime before replacing the active database.
	backupService := service.NewBackupService(service.BackupRuntime{
		ActivePath:     database.Path,
		Dir:            appConfig.StorageConfig.BackupsDir(),
		Metadata:       snapshotMetadata,
		Compatibility:  snapshotCompatibility,
		RequestRestart: requestRestart,
		Logf:           backupScheduler.Logf,
	}, queueClient)

	// River's Start runs the client in a background goroutine until Stop is
	// called; it returns once startup completes. context.Background is used (not
	// the run context) so a shutdown signal triggers a graceful drain via Stop
	// rather than an abrupt cancellation of in-flight jobs.
	if err := queueClient.Start(context.Background()); err != nil {
		return fmt.Errorf("start queue client: %w", err)
	}
	queueStopped := false
	defer func() {
		if queueStopped {
			return
		}
		if err := stopRiverQueue(queueClient, queueStopTimeout, queueKillTimeout); err != nil {
			databaseCloseAllowed = false
			appLogger.Warn("queue client cleanup failed", zap.Error(err))
			runErr = errors.Join(runErr, fmt.Errorf("stop River during cleanup: %w", err))
			return
		}
		queueStopped = true
		databaseCloseAllowed = true
	}()
	appLogger.Info("queues initialized successfully", zap.String("operation", "queue.init"))
	backfillOwners, err := eventService.InitializeBackfill(ctx)
	if err != nil {
		return fmt.Errorf("initialize Event backfill: %w", err)
	}
	for _, ownerID := range backfillOwners {
		args := jobs.EventRebuildArgs{OwnerID: ownerID}
		opts := args.InsertOpts()
		if _, err := queueClient.Insert(ctx, args, &opts); err != nil {
			return fmt.Errorf("enqueue Event backfill for owner %d: %w", ownerID, err)
		}
	}

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

	// Hourly database-backup heartbeat. RunOnStart also gives a fresh install
	// its first dump immediately; the worker's due-check (interval from
	// runtime settings, storage reachability) turns superfluous ticks into
	// silent no-ops.
	queueClient.PeriodicJobs().Add(river.NewPeriodicJob(
		river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) {
			return jobs.DatabaseBackupArgs{}, nil
		},
		&river.PeriodicJobOpts{ID: "database_backup", RunOnStart: true},
	))
	queueClient.PeriodicJobs().Add(river.NewPeriodicJob(
		river.PeriodicInterval(time.Minute),
		func() (river.JobArgs, *river.InsertOpts) {
			args := jobs.ScheduleEventRebuildsArgs{}
			opts := args.InsertOpts()
			return args, &opts
		},
		&river.PeriodicJobOpts{ID: "schedule_event_rebuilds", RunOnStart: true},
	))
	queueClient.PeriodicJobs().Add(river.NewPeriodicJob(
		river.PeriodicInterval(bleveocr.DefaultOutboxWakeInterval),
		func() (river.JobArgs, *river.InsertOpts) {
			if !ocrIndexTrigger.ShouldSchedule(time.Now(), bleveocr.DefaultOutboxRecoveryInterval) {
				return nil, nil
			}
			return jobs.ProcessOCROutboxArgs{}, nil
		},
		&river.PeriodicJobOpts{ID: "ocr_index_outbox", RunOnStart: true},
	))

	// Initialize controllers with new storage system
	assetController := handler.NewAssetHandler(assetService, authService, indexingService, stackService, queries, sqlDB, repoManager, stagingManager, queueClient, settingsService, lumenService, repositoryFiles)
	assetController.StartCleanupTasks(ctx)
	authController := handler.NewAuthHandler(authService, authRateLimiter, appConfig.Auth.RefreshTokenTTL, originPolicy)
	setupController := handler.NewSetupHandler(service.NewSetupService(bootstrapService, repoManager, appConfig.StorageConfig.Path))
	albumController := handler.NewAlbumHandler(&albumService, queries, queueClient, settingsService, lumenService)
	peopleController := handler.NewPeopleHandler(assetService, faceService, authService, repoManager, repositoryFiles)
	locationController := handler.NewLocationHandler(locationService, queueClient)
	speciesController := handler.NewSpeciesHandler(speciesReferenceService)
	userController := handler.NewUserHandler(userService, securityLogger)
	queueController := handler.NewQueueHandler(sqlDB)
	statsController := handler.NewStatsHandler(queries)
	agentController := handler.NewAgentHandler(agentService, refStore, authorizedLibraries, agentPins, assetService)
	capabilitiesController := handler.NewCapabilitiesHandler(settingsService, lumenService)
	settingsController := handler.NewSettingsHandler(settingsService, backupService, dto.NewRuntimeInfoDTO(appConfig))
	classifierController := handler.NewClassifierHandler(classifierService)
	// Initialize Cloud Sync service and handler
	cloudSyncService := cloud.NewCloudSyncService(queries, sourceMaterializer, stagingManager, repoManager, appConfig.Auth.SecretKeyFile, appConfig.StorageConfig.CloudDir(), appLogger.Named("cloud_sync"))
	// Reconcile import runs left "running"/"queued" by a previous crash/restart
	// so repositories are not stuck with an import that never finishes.
	if err := cloudSyncService.RecoverInterruptedRuns(ctx); err != nil {
		appLogger.Warn("failed to recover interrupted cloud import runs", zap.Error(err))
	}
	// Reclaim scan runs left "running" by a previous crash/restart so the
	// one-running-per-repository index does not permanently block scans.
	if err := repositoryScanner.ReclaimInterruptedRuns(ctx); err != nil {
		appLogger.Warn("failed to reclaim interrupted repository scan runs", zap.Error(err))
	}
	cloudController := handler.NewCloudHandler(cloudSyncService)
	repositoryScanController := handler.NewRepositoryScanHandler(repositoryScanner, repoManager)
	hostActionController := handler.NewHostActionHandler(repoManager, controls.RepositoryManagerReady != nil)
	duplicateController := handler.NewDuplicateHandler(duplicateService, queries)
	eventController := handler.NewEventHandler(eventService, sqlDB, shareLinkService, queueClient)
	shareLinkController := handler.NewShareLinkHandler(shareLinkService, assetService, queries, repositoryFiles)

	// Initialize Swagger docs
	docs.SwaggerInfo.Title = "Lumilio-Photos API"
	docs.SwaggerInfo.Description = "Photo management system API with asset upload, processing, and organization features"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.Host = ""
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
		hostActionController,
		duplicateController,
		eventController,
		cloudController,
		shareLinkController,
		handler.RequireLLMAgentEnabled(settingsService),
		handler.RequireAppInitialized(bootstrapService),
		originPolicy,
		appLogger.Named("http"),
	)

	// Add Swagger documentation endpoint
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Optionally serve the SPA bundle (desktop sets server.web_root; docker/web
	// leave it empty and serve the bundle from a separate static server).
	api.RegisterSPA(router, appConfig.ServerConfig.WebRoot)

	transport, err := servertransport.Start(
		ctx,
		appConfig.ServerConfig,
		router,
		appLogger.Named("transport"),
	)
	if err != nil {
		return fmt.Errorf("start server transport: %w", err)
	}
	settingsController.SetCertificateInfoProvider(func() dto.CertificateRuntimeInfo {
		status := transport.CertificateStatus()
		return dto.CertificateRuntimeInfo{
			Hostname:      status.Hostname,
			Status:        status.Status,
			ExpiresAt:     status.ExpiresAt,
			LastManagedAt: status.LastManagedAt,
		}
	})
	shutdownTransport := func() error {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer shutdownCancel()
		return transport.Shutdown(shutdownCtx)
	}

	if dbbackup.HasAppliedRestore(database.Path) {
		if _, err := settingsService.GetSystemSettings(ctx); err != nil {
			return errors.Join(
				fmt.Errorf("settings unreadable after SQLite restore: %w", err),
				shutdownTransport(),
			)
		}
		var users int
		if err := sqlDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&users); err != nil {
			return errors.Join(
				fmt.Errorf("users table unreadable after SQLite restore: %w", err),
				shutdownTransport(),
			)
		}
		if users == 0 {
			return errors.Join(errors.New("no users present after SQLite restore"), shutdownTransport())
		}
		if err := dbbackup.CompletePendingRestore(ctx, database.Path); err != nil {
			return errors.Join(
				fmt.Errorf("complete SQLite restore: %w", err),
				shutdownTransport(),
			)
		}
		appLogger.Info("restored SQLite runtime generation is healthy", zap.String("operation", "database.restore"))
	}
	if controls.RuntimeReady != nil {
		controls.RuntimeReady(RuntimeInfo{
			Listen:     appConfig.ServerConfig.Listen,
			ProductURL: productURL(appConfig.ServerConfig.Listen),
		})
	}

	appLogger.Info("server starting",
		zap.String("operation", "server.listen"),
		zap.String("listen", appConfig.ServerConfig.Listen),
		zap.String("origin_policy", "request-derived"),
		zap.String("tls_mode", string(appConfig.ServerConfig.TLS.Mode)),
		zap.Int("trusted_proxy_cidr_count", len(appConfig.ServerConfig.Proxy.TrustedCIDRs)),
		zap.String("acme_hostname", appConfig.ServerConfig.TLS.Hostname),
		zap.String("acme_storage_path", appConfig.ServerConfig.TLS.StoragePath),
	)

	// Serve in a goroutine so this function can block on ctx and drive a
	// graceful shutdown when it is cancelled.
	transportErr := make(chan error, 1)
	go func() {
		transportErr <- transport.Wait()
	}()

	select {
	case err := <-transportErr:
		if err != nil {
			return errors.Join(fmt.Errorf("server transport: %w", err), shutdownTransport())
		}
		return errors.Join(errors.New("server transport stopped unexpectedly"), shutdownTransport())
	case <-ctx.Done():
		appLogger.Info("shutdown signal received, draining", zap.String("operation", "server.shutdown"))
	}

	// HTTP and River have independent budgets. A subsystem that cannot prove it
	// has stopped blocks SQLite close and therefore blocks an in-process restore
	// swap; the process host may still choose to terminate normally.
	httpErr := shutdownTransport()
	if httpErr != nil {
		databaseCloseAllowed = false
		appLogger.Warn("http server shutdown error", zap.String("operation", "server.shutdown"), zap.Error(httpErr))
	}
	queueErr := stopRiverQueue(queueClient, queueStopTimeout, queueKillTimeout)
	if queueErr != nil {
		databaseCloseAllowed = false
		appLogger.Warn("queue client shutdown error", zap.String("operation", "queue.shutdown"), zap.Error(queueErr))
	} else {
		queueStopped = true
	}
	if httpErr != nil || queueErr != nil {
		return errors.Join(
			wrapOptionalError("drain HTTP server", httpErr),
			wrapOptionalError("stop River queue", queueErr),
		)
	}
	appLogger.Info("shutdown complete", zap.String("operation", "server.shutdown"))
	return nil
}

type defaultStorageRuntimeManager interface {
	EnsureDefaultRepositoryRoot(context.Context, string, ...storage.LifecycleRequest) (*repo.RepositoryRoot, error)
	ListRepositoryRoots(context.Context) ([]repo.RepositoryRoot, error)
}

// ensureDefaultStorageForRuntime distinguishes a first-run initialization
// failure from a previously registered portable identity that needs recovery.
// The latter must not prevent the HTTP runtime and unrelated repositories from
// starting in degraded mode.
func ensureDefaultStorageForRuntime(ctx context.Context, manager defaultStorageRuntimeManager, path string) (*repo.RepositoryRoot, bool, error) {
	hostInstanceID, _ := os.Hostname()
	root, err := manager.EnsureDefaultRepositoryRoot(ctx, path, storage.LifecycleRequest{
		Actor: "server:config", HostInstanceID: hostInstanceID, ConfirmationType: "portable_identity_match",
	})
	if err == nil {
		return root, false, nil
	}
	if !errors.Is(err, storage.ErrRepositoryRootOffline) && !errors.Is(err, storage.ErrRepositoryRootInvalid) {
		return nil, false, fmt.Errorf("initialize default storage location: %w", err)
	}
	roots, listErr := manager.ListRepositoryRoots(ctx)
	if listErr != nil {
		return nil, false, fmt.Errorf("verify degraded default Storage Location: %w", listErr)
	}
	for i := range roots {
		if roots[i].Kind == dbtypes.RepositoryRootKindDefault {
			// A registered default failing at its unchanged configured path is a
			// recoverable offline/missing-marker condition. A different configured
			// path is a migration attempt; failure to prove its portable identity
			// must fail startup so Desktop rolls the runtime intent back.
			registeredPath, registeredPathErr := storage.CanonicalizeRepositoryPath(roots[i].Path)
			configuredPath, configuredPathErr := storage.CanonicalizeRepositoryPath(path)
			if registeredPathErr != nil || configuredPathErr != nil || registeredPath != configuredPath {
				return nil, false, fmt.Errorf("validate default storage location migration: %w", err)
			}
			return &roots[i], true, nil
		}
	}
	return nil, false, fmt.Errorf("initialize default storage location: %w", err)
}

func monitorRepositoryCapacity(ctx context.Context, manager *storage.DefaultRepositoryManager, logger *zap.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := manager.ReconcileRepositoryCapacity(ctx); err != nil && !errors.Is(err, context.Canceled) {
				logger.Warn("repository capacity monitor failed", zap.Error(err))
			}
		}
	}
}

func productURL(listen string) string {
	listen = strings.TrimSpace(listen)
	if strings.HasPrefix(listen, ":") {
		listen = "127.0.0.1" + listen
	}
	if strings.HasPrefix(listen, "0.0.0.0:") {
		listen = "127.0.0.1:" + strings.TrimPrefix(listen, "0.0.0.0:")
	}
	return "http://" + listen
}

type riverStopper interface {
	Stop(context.Context) error
	StopAndCancel(context.Context) error
	Stopped() <-chan struct{}
}

func stopRiverQueue(client riverStopper, gracefulBudget, forcedBudget time.Duration) error {
	stopped := client.Stopped()
	gracefulCtx, gracefulCancel := context.WithTimeout(context.Background(), gracefulBudget)
	gracefulErr := client.Stop(gracefulCtx)
	if gracefulErr == nil {
		select {
		case <-stopped:
		case <-gracefulCtx.Done():
			gracefulErr = gracefulCtx.Err()
		}
	}
	gracefulCancel()
	if channelClosed(stopped) {
		return nil
	}
	if gracefulErr == nil {
		gracefulErr = errors.New("River Stop returned without closing Stopped")
	}

	forcedCtx, forcedCancel := context.WithTimeout(context.Background(), forcedBudget)
	forcedErr := client.StopAndCancel(forcedCtx)
	if forcedErr == nil {
		select {
		case <-stopped:
		case <-forcedCtx.Done():
			forcedErr = forcedCtx.Err()
		}
	}
	forcedCancel()
	if channelClosed(stopped) {
		return nil
	}
	if forcedErr == nil {
		forcedErr = errors.New("River StopAndCancel returned without closing Stopped")
	}
	return errors.Join(
		fmt.Errorf("graceful River drain: %w", gracefulErr),
		fmt.Errorf("forced River cancellation: %w", forcedErr),
	)
}

func channelClosed(channel <-chan struct{}) bool {
	select {
	case <-channel:
		return true
	default:
		return false
	}
}

func wrapOptionalError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

type pprofServerHost struct {
	server *http.Server
	done   chan error
}

func startPprofHost(addr string) (*pprofServerHost, error) {
	if addr == "" {
		return nil, nil
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen for pprof on %s: %w", addr, err)
	}
	server := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	host := &pprofServerHost{
		server: server,
		done:   make(chan error, 1),
	}
	go func() {
		host.done <- server.Serve(listener)
	}()
	return host, nil
}

func (host *pprofServerHost) shutdown(ctx context.Context) error {
	if host == nil {
		return nil
	}
	shutdownErr := host.server.Shutdown(ctx)
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, host.server.Close())
	}
	select {
	case serveErr := <-host.done:
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(shutdownErr, serveErr)
	case <-ctx.Done():
		return errors.Join(shutdownErr, ctx.Err())
	}
}

func initMLServices(
	ctx context.Context,
	appConfig config.AppConfig,
	sqlDB *sql.DB,
	queries *repo.Queries,
	workers *river.Workers,
	appLogger *zap.Logger,
	lumenLogger *zap.Logger,
	settingsService service.SettingsService,
	faceService service.FaceService,
	repositoryFiles *storage.RepositoryFSFactory,
	ocrIndexNotifier service.OCRIndexNotifier,
) (service.LumenService, service.EmbeddingService, service.ClassifierService, error) {
	appLogger.Info("initializing ML services", zap.String("operation", "ml.init"))

	lumenService, err := service.NewLumenServiceFromAppConfig(appConfig.Lumen, lumenLogger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("configure lumen service: %w", err)
	}

	err = lumenService.Start(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	appLogger.Info("lumen service initialized",
		zap.String("operation", "ml.init"),
		zap.Bool("enabled", appConfig.Lumen.Enabled()),
	)

	embeddingService := service.NewEmbeddingService(queries, sqlDB)
	speciesService := service.NewSpeciesService(queries)
	ocrService := service.NewOCRServiceWithNotifier(queries, sqlDB, ocrIndexNotifier)
	imageLoader := queue.NewDBMLImageLoader(queries, repositoryFiles)

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
	classifierService := service.NewClassifierService(sqlDB, lumenService, embeddingService, appLogger.Named("classifier"))
	river.AddWorker[queue.ZeroshotClassifyArgs](workers, &queue.ZeroshotClassifyWorker{
		EmbeddingService:  embeddingService,
		ClassifierService: classifierService,
		AITagService:      aiTagService,
		ConfigProvider:    settingsService,
	})
	appLogger.Info("zero-shot classifier service and worker registered", zap.String("operation", "ml.init"))

	// Build classifier prototypes in the background once the semantic text-embed
	// task is reachable, so startup never blocks on ML node availability. With
	// the Lumen integration disabled the task can never appear, so skip the wait
	// entirely.
	if appConfig.Lumen.Enabled() {
		go buildClassifierPrototypes(lumenService, classifierService, appLogger.Named("classifier"))
	}

	appLogger.Info("ML services initialization complete", zap.String("operation", "ml.init"))
	return lumenService, embeddingService, classifierService, nil
}

// buildClassifierPrototypes waits (bounded) for the semantic text-embed task and
// its capability contract to become available, then builds/refreshes classifier
// prototypes. Task availability can become visible just before the full
// capability snapshot, so failed builds are retried within the startup window.
func buildClassifierPrototypes(lumenService service.LumenService, classifierService service.ClassifierService, logger *zap.Logger) {
	buildCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		if lumenService != nil && lumenService.IsTaskAvailable("semantic_text_embed") {
			if err := classifierService.EnsurePrototypes(buildCtx); err == nil {
				return
			} else {
				logger.Warn("failed to build classifier prototypes; retrying", zap.Error(err))
			}
		}

		select {
		case <-buildCtx.Done():
			logger.Warn("timed out building classifier prototypes", zap.Error(buildCtx.Err()))
			return
		case <-ticker.C:
		}
	}
}
