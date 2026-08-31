package jobs

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const QueueMacro = "catalog_macro"

func macroOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueMacro, MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: activeUniqueStates()}}
}

// IngestAssetArgs resumes one catalog-owned ingest receipt. The payload carries
// only its immutable commit/receipt fence; staging paths and product facts stay
// in catalog.db.
type IngestAssetArgs struct {
	CommitID  uuid.UUID `json:"commitId" river:"unique"`
	ReceiptID uuid.UUID `json:"receiptId" river:"unique"`
	Admission string    `json:"admissionClass" river:"unique"`
}

func (IngestAssetArgs) Kind() string                 { return "ingest_asset" }
func (IngestAssetArgs) InsertOpts() river.InsertOpts { return macroOpts() }

type AnalyzeAssetArgs struct {
	AssetID         uuid.UUID `json:"assetId" river:"unique"`
	SourceFence     uuid.UUID `json:"sourceFence" river:"unique"`
	DesiredVersion  uint64    `json:"desiredVersion" river:"unique"`
	PipelineVersion string    `json:"pipelineVersion" river:"unique"`
	Admission       string    `json:"admissionClass" river:"unique"`
}

func (AnalyzeAssetArgs) Kind() string                 { return "analyze_asset" }
func (AnalyzeAssetArgs) InsertOpts() river.InsertOpts { return macroOpts() }

type GenerateAssetDerivativesArgs struct {
	AssetID         uuid.UUID `json:"assetId" river:"unique"`
	SourceFence     uuid.UUID `json:"sourceFence" river:"unique"`
	DesiredVersion  uint64    `json:"desiredVersion" river:"unique"`
	PipelineVersion string    `json:"pipelineVersion" river:"unique"`
	Admission       string    `json:"admissionClass" river:"unique"`
}

func (GenerateAssetDerivativesArgs) Kind() string                 { return "generate_asset_derivatives" }
func (GenerateAssetDerivativesArgs) InsertOpts() river.InsertOpts { return macroOpts() }

type TranscodeMediaArgs struct {
	AssetID         uuid.UUID `json:"assetId" river:"unique"`
	SourceFence     uuid.UUID `json:"sourceFence" river:"unique"`
	DesiredVersion  uint64    `json:"desiredVersion" river:"unique"`
	PipelineVersion string    `json:"pipelineVersion" river:"unique"`
	Admission       string    `json:"admissionClass" river:"unique"`
}

func (TranscodeMediaArgs) Kind() string                 { return "transcode_media" }
func (TranscodeMediaArgs) InsertOpts() river.InsertOpts { return macroOpts() }

type EnrichAssetArgs struct {
	AssetID         uuid.UUID `json:"assetId" river:"unique"`
	SourceFence     uuid.UUID `json:"sourceFence" river:"unique"`
	DesiredVersion  uint64    `json:"desiredVersion" river:"unique"`
	PipelineVersion string    `json:"pipelineVersion" river:"unique"`
	Admission       string    `json:"admissionClass" river:"unique"`
}

func (EnrichAssetArgs) Kind() string                 { return "enrich_asset" }
func (EnrichAssetArgs) InsertOpts() river.InsertOpts { return macroOpts() }

type ScanRepositoryBatchArgs struct {
	RepositoryID   uuid.UUID `json:"repositoryId" river:"unique"`
	RequestedEpoch uint64    `json:"requestedEpoch" river:"unique"`
	DesiredVersion uint64    `json:"desiredVersion" river:"unique"`
	Frontier       string    `json:"frontier,omitempty" river:"unique"`
	Admission      string    `json:"admissionClass" river:"unique"`
}

func (ScanRepositoryBatchArgs) Kind() string                 { return "scan_repository_batch" }
func (ScanRepositoryBatchArgs) InsertOpts() river.InsertOpts { return macroOpts() }

type RebuildProjectionBatchArgs struct {
	ProjectionKind    string `json:"projectionKind" river:"unique"`
	Scope             string `json:"scope" river:"unique"`
	SourceRevision    uint64 `json:"sourceRevision" river:"unique"`
	ProjectionVersion uint64 `json:"projectionVersion" river:"unique"`
	Cursor            string `json:"cursor,omitempty" river:"unique"`
	Admission         string `json:"admissionClass" river:"unique"`
}

func (RebuildProjectionBatchArgs) Kind() string                 { return "rebuild_projection_batch" }
func (RebuildProjectionBatchArgs) InsertOpts() river.InsertOpts { return macroOpts() }

type BackupCatalogArgs struct {
	RequestID uuid.UUID `json:"requestId" river:"unique"`
	Force     bool      `json:"force"`
	Admission string    `json:"admissionClass" river:"unique"`
}

func (BackupCatalogArgs) Kind() string                 { return "backup_catalog" }
func (BackupCatalogArgs) InsertOpts() river.InsertOpts { return macroOpts() }
