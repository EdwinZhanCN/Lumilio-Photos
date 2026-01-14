package processors

import (
	"time"

	"server/config"
	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
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
	assetService   service.AssetService
	queries        *repo.Queries
	repoManager    storage.RepositoryManager
	stagingManager storage.StagingManager
	queueClient    *river.Client[pgx.Tx]
	appConfig      config.AppConfig
	lumenService   service.LumenService
}

// NewAssetProcessor constructs the processor with required dependencies.
func NewAssetProcessor(
	assetService service.AssetService,
	queries *repo.Queries,
	repoManager storage.RepositoryManager,
	stagingManager storage.StagingManager,
	queueClient *river.Client[pgx.Tx],
	appConfig config.AppConfig,
	lumenService service.LumenService,
) *AssetProcessor {
	return &AssetProcessor{
		assetService:   assetService,
		queries:        queries,
		repoManager:    repoManager,
		stagingManager: stagingManager,
		queueClient:    queueClient,
		appConfig:      appConfig,
		lumenService:   lumenService,
	}
}
