package processors

import (
	"time"

	"server/config"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/service"
	"server/internal/sourcing"
	"server/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

// AssetPayload matches the ingest-stage payload fields (kept for compatibility with task workers).
type AssetPayload struct {
	ContentHash      string    `json:"contentHash" river:"unique"`
	QuickFingerprint string    `json:"quickFingerprint,omitempty"`
	StagedPath       string    `json:"stagedPath"`
	UserID           string    `json:"userId" river:"unique"`
	Timestamp        time.Time `json:"timestamp"`
	ContentType      string    `json:"contentType,omitempty"`
	FileName         string    `json:"fileName,omitempty"`
	RepositoryID     string    `json:"repositoryId,omitempty"` // Repository UUID
}

// AssetProcessor holds shared dependencies for per-task processors.
type AssetProcessor struct {
	assetService     service.AssetService
	queries          *repo.Queries
	repoManager      storage.RepositoryManager
	stagingManager   storage.StagingManager
	materializer     *sourcing.SourceMaterializer
	queueClient      *river.Client[pgx.Tx]
	settingsService  service.SettingsService
	embeddingService service.EmbeddingService
	lumenService     service.LumenService
	transcodeConfig  config.TranscodeConfig
	toolsConfig      config.ToolsConfig
	logger           *zap.Logger
	auditProvider    logging.RepositoryAuditProvider
}

// NewAssetProcessor constructs the processor with required dependencies.
func NewAssetProcessor(
	assetService service.AssetService,
	queries *repo.Queries,
	repoManager storage.RepositoryManager,
	stagingManager storage.StagingManager,
	materializer *sourcing.SourceMaterializer,
	queueClient *river.Client[pgx.Tx],
	settingsService service.SettingsService,
	embeddingService service.EmbeddingService,
	lumenService service.LumenService,
	transcodeConfig config.TranscodeConfig,
	toolsConfig config.ToolsConfig,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
) *AssetProcessor {
	if logger == nil {
		logger = zap.NewNop()
	}
	if auditProvider == nil {
		auditProvider = logging.NewRepositoryAuditProvider(logger, false)
	}
	return &AssetProcessor{
		assetService:     assetService,
		queries:          queries,
		repoManager:      repoManager,
		stagingManager:   stagingManager,
		materializer:     materializer,
		queueClient:      queueClient,
		settingsService:  settingsService,
		embeddingService: embeddingService,
		lumenService:     lumenService,
		transcodeConfig:  transcodeConfig,
		toolsConfig:      toolsConfig,
		logger:           logger.With(zap.String("component", "processor")),
		auditProvider:    auditProvider,
	}
}

func (ap *AssetProcessor) repoAudit(repoPath string) logging.RepositoryAuditLogger {
	if ap == nil {
		return logging.NewRepositoryAuditProvider(zap.NewNop(), false).ForPath(repoPath)
	}
	if ap.auditProvider == nil {
		return logging.NewRepositoryAuditProvider(ap.logger, false).ForPath(repoPath)
	}
	return ap.auditProvider.ForPath(repoPath)
}
