package processors

import (
	"time"

	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/service"
	"server/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

// AssetPayload matches the ingest-stage payload fields (kept for compatibility with task workers).
type AssetPayload struct {
	ClientHash   string    `json:"clientHash" river:"unique"`
	StagedPath   string    `json:"stagedPath"`
	UserID       string    `json:"userId" river:"unique"`
	Timestamp    time.Time `json:"timestamp"`
	ContentType  string    `json:"contentType,omitempty"`
	FileName     string    `json:"fileName,omitempty"`
	RepositoryID string    `json:"repositoryId,omitempty"` // Repository UUID
}

// AssetProcessor holds shared dependencies for per-task processors.
type AssetProcessor struct {
	assetService    service.AssetService
	queries         *repo.Queries
	repoManager     storage.RepositoryManager
	stagingManager  storage.StagingManager
	queueClient     *river.Client[pgx.Tx]
	settingsService service.SettingsService
	lumenService    service.LumenService
	logger          *zap.Logger
	auditProvider   logging.RepositoryAuditProvider
}

// NewAssetProcessor constructs the processor with required dependencies.
func NewAssetProcessor(
	assetService service.AssetService,
	queries *repo.Queries,
	repoManager storage.RepositoryManager,
	stagingManager storage.StagingManager,
	queueClient *river.Client[pgx.Tx],
	settingsService service.SettingsService,
	lumenService service.LumenService,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
) *AssetProcessor {
	if logger == nil {
		logger = zap.NewNop()
	}
	if auditProvider == nil {
		auditProvider = logging.NewRepositoryAuditProvider(logger)
	}
	return &AssetProcessor{
		assetService:    assetService,
		queries:         queries,
		repoManager:     repoManager,
		stagingManager:  stagingManager,
		queueClient:     queueClient,
		settingsService: settingsService,
		lumenService:    lumenService,
		logger:          logger.With(zap.String("component", "processor")),
		auditProvider:   auditProvider,
	}
}

func (ap *AssetProcessor) runtimeIndexingTaskAvailable(task service.AssetIndexingTask) bool {
	return service.IsIndexingTaskRuntimeAvailable(ap.lumenService, task)
}

func (ap *AssetProcessor) repoAudit(repoPath string) logging.RepositoryAuditLogger {
	if ap == nil {
		return logging.NewRepositoryAuditProvider(zap.NewNop()).ForPath(repoPath)
	}
	if ap.auditProvider == nil {
		return logging.NewRepositoryAuditProvider(ap.logger).ForPath(repoPath)
	}
	return ap.auditProvider.ForPath(repoPath)
}
