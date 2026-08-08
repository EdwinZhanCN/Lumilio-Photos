package processors

import (
	"database/sql"

	"server/config"
	"server/internal/db/repo"
	"server/internal/logging"
	"server/internal/service"
	"server/internal/sourcing"
	"server/internal/storage"

	"github.com/riverqueue/river"
	"go.uber.org/zap"
)

// AssetProcessor holds shared dependencies for per-task processors.
type AssetProcessor struct {
	assetService     service.AssetService
	queries          *repo.Queries
	files            *storage.RepositoryFSFactory
	materializer     *sourcing.SourceMaterializer
	queueClient      *river.Client[*sql.Tx]
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
	materializer *sourcing.SourceMaterializer,
	queueClient *river.Client[*sql.Tx],
	settingsService service.SettingsService,
	embeddingService service.EmbeddingService,
	lumenService service.LumenService,
	transcodeConfig config.TranscodeConfig,
	toolsConfig config.ToolsConfig,
	logger *zap.Logger,
	auditProvider logging.RepositoryAuditProvider,
	files *storage.RepositoryFSFactory,
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
		files:            files,
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
	if ap == nil || ap.auditProvider == nil {
		return logging.NoopRepositoryAuditLogger()
	}
	return ap.auditProvider.ForPath(repoPath)
}
