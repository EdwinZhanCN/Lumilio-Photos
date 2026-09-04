package processors

import (
	"context"
	"database/sql"
	"errors"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/google/uuid"

	"server/config"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/execution"
	"server/internal/logging"
	"server/internal/settings"
	"server/internal/storage"
	"server/internal/storage/roe/locations"
	"server/internal/utils/imagesource"

	"go.uber.org/zap"
)

// AssetProcessor holds shared dependencies for per-task processors.
type AssetProcessor struct {
	readerDatabase   QueryRower
	reader           AssetReader
	files            *storage.RepositoryFSFactory
	locationResolver AssetLocationResolver
	materializer     SourceCommitMaterializer
	settingsService  MLSettingsReader
	lumenService     ImageEmbedder
	transcodeConfig  config.TranscodeConfig
	toolsConfig      config.ToolsConfig
	toolSession      execution.ToolSession
	logger           *zap.Logger
	auditProvider    logging.RepositoryAuditProvider
}

type QueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type AssetReader interface {
	GetAssetByID(context.Context, uuid.UUID) (repo.Asset, error)
	GetContentObjectByID(context.Context, uuid.UUID) (repo.ContentObject, error)
	GetRepository(context.Context, uuid.UUID) (repo.Repository, error)
}

type AssetLocationResolver interface {
	LocalAssetPath(context.Context, uuid.UUID) (*locations.OpenedMedia, string, error)
}

type SourceCommitMaterializer interface {
	MaterializeCommit(context.Context, uuid.UUID) (*repo.Asset, error)
}

type MLSettingsReader interface {
	GetEffectiveMLConfig(context.Context) (settings.ML, error)
}

type ImageEmbedder interface {
	SemanticImageEmbed(context.Context, *imagesource.MLImage) (*types.EmbeddingV1, error)
}

func (ap *AssetProcessor) SetLocationResolver(resolver AssetLocationResolver) {
	ap.locationResolver = resolver
}

// NewAssetProcessor constructs the processor with required dependencies.
func NewAssetProcessor(
	reader AssetReader,
	readerDatabase QueryRower,
	materializer SourceCommitMaterializer,
	settingsService MLSettingsReader,
	lumenService ImageEmbedder,
	transcodeConfig config.TranscodeConfig,
	toolsConfig config.ToolsConfig,
	toolSession execution.ToolSession,
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
		readerDatabase:  readerDatabase,
		reader:          reader,
		files:           files,
		materializer:    materializer,
		settingsService: settingsService,
		lumenService:    lumenService,
		transcodeConfig: transcodeConfig,
		toolsConfig:     toolsConfig,
		toolSession:     toolSession,
		logger:          logger.With(zap.String("component", "processor")),
		auditProvider:   auditProvider,
	}
}

// AssetMediaType returns the typed media category for an asset.
func (ap *AssetProcessor) AssetMediaType(ctx context.Context, assetID uuid.UUID) (dbtypes.AssetType, error) {
	if ap == nil || ap.reader == nil {
		return "", errors.New("asset processor reader is not configured")
	}
	asset, err := ap.reader.GetAssetByID(ctx, assetID)
	if err != nil {
		return "", err
	}
	return dbtypes.AssetType(asset.Type), nil
}

func (ap *AssetProcessor) repoAudit(repoPath string) logging.RepositoryAuditLogger {
	if ap == nil || ap.auditProvider == nil {
		return logging.NoopRepositoryAuditLogger()
	}
	return ap.auditProvider.ForPath(repoPath)
}
