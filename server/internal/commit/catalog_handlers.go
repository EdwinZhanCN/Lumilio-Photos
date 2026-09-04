package commit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/google/uuid"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/event"
	"server/internal/pipeline"
	"server/internal/service"
	roematerializer "server/internal/storage/roe/materializer"
)

type AssetStageApplied struct {
	AssetID, SourceFence   uuid.UUID
	Stage, PipelineVersion string
	DesiredVersion         uint64
	// TerminalError is a stable public-safe failure code. It is set only by an
	// explicit Catalog transition; retry/reprocess creates a newer desired
	// generation and clears it.
	TerminalError string
}

// AssetMetadataApplied carries the immutable result of metadata extraction.
// The commit handler validates the asset fence before applying it in the same
// transaction that advances the analyze stage.
type AssetMetadataApplied struct {
	AssetID, SourceFence uuid.UUID
	PipelineVersion      string
	DesiredVersion       uint64
	Metadata             dbtypes.SpecificMetadata
	Common               dbtypes.CommonMetadata
	ExifRaw              dbtypes.JSON
	ComponentRelation    string
}

type ThumbnailArtifact struct {
	RepositoryID uuid.UUID
	Size         string
	StoragePath  string
	MimeType     string
}

type AssetDerivativesApplied struct {
	AssetID, SourceFence uuid.UUID
	PipelineVersion      string
	DesiredVersion       uint64
	Artifacts            []ThumbnailArtifact
}

// AssetStackApplied requests the deterministic stack/live-photo projection for
// the repository containing the fenced asset. The projection itself is applied
// in this transaction; no stack writer is exposed to the macro worker.
type AssetStackApplied struct {
	AssetID, SourceFence uuid.UUID
	PipelineVersion      string
	DesiredVersion       uint64
}

// RepositoryAssetApplied activates a repository observation's immutable asset
// through the same coordinator used by asset-stage commits.
type RepositoryAssetApplied struct {
	RepositoryID, NodeID, AssetID, ContentID uuid.UUID
	// ObservationRevision is the immutable ROE revision that was inspected and
	// bound. The coordinator rechecks it before activation so a result that
	// races a newer observation cannot publish an old occurrence.
	ObservationRevision int64
}

// RepositoryKnownContentApplied is a fully hashed, immutable source fact.
// The coordinator alone turns it into nodes, content, an Asset, and its
// Location; upload and cloud macros hold no catalog write capability.
type RepositoryKnownContentApplied struct {
	Fact roematerializer.KnownContent
}

// RepositoryHashApplied carries the immutable filesystem observation prepared
// outside the catalog transaction for one ROE node revision.
type RepositoryHashApplied struct {
	Prepared roematerializer.HashPreparation
}

type VideoFrameEmbedding struct {
	FrameTsMs int32
	Vector    []float32
}

type VideoFrameEmbeddingsApplied struct {
	AssetID, SourceFence uuid.UUID
	PipelineVersion      string
	DesiredVersion       uint64
	ModelID              string
	Frames               []VideoFrameEmbedding
}

type EnrichmentEmbedding struct {
	Type      service.EmbeddingType
	Model     string
	Vector    []float32
	IsPrimary bool
}

type EnrichmentAesthetic struct {
	Score float32
	Model string
}

type EnrichmentSpecies struct {
	Predictions []dbtypes.SpeciesPredictionMeta
}

type EnrichmentFace struct {
	Payload          *types.FaceV1
	ImageData        []byte
	ProcessingTimeMs int
}

type EnrichmentTags struct {
	Tags []service.AIGeneratedTag
}

// EnrichmentApplied carries all optional enrichment facts computed for one
// asset generation. The commit handler applies the immutable snapshot in one
// fenced catalog transaction before the stage acknowledgement is submitted.
type EnrichmentApplied struct {
	AssetID, SourceFence uuid.UUID
	PipelineVersion      string
	DesiredVersion       uint64
	PHash                *EnrichmentEmbedding
	Semantic             *EnrichmentEmbedding
	Aesthetic            *EnrichmentAesthetic
	Species              *EnrichmentSpecies
	OCR                  *types.OCRV1
	Face                 *EnrichmentFace
	AITags               *EnrichmentTags
}
type IngestReceiptApplied struct {
	ReceiptID     uuid.UUID
	TerminalError string
}
type OperationReceiptApplied struct {
	ReceiptID     uuid.UUID
	Kind          string
	TerminalError string
}

// EventProjectionApplied carries the complete deterministic Event replacement
// computed from one reader snapshot. The coordinator applies it atomically and
// advances the Event projection ledger in the same catalog transaction.
type EventProjectionApplied struct {
	Prepared          event.PreparedRebuild
	ProjectionVersion uint64
}

// LocationProjectionApplied carries one bounded topology mutation prefix.
// Compute happens against a reader snapshot; the coordinator owns the catalog
// transaction that applies the prefix and, when complete, advances the
// published revision.
type LocationProjectionApplied struct {
	Prepared          service.PreparedLocationRebuild
	ProjectionVersion uint64
}

// LocationResolutionApplied carries provider/cache results that were computed
// outside the catalog transaction. The coordinator applies them and advances
// the resolution ledger only after all eligible work for the batch is done.
type LocationResolutionApplied struct {
	Prepared          service.PreparedLocationResolution
	ProjectionVersion uint64
}

type OCRIndexEntry struct {
	AssetID  uuid.UUID
	Revision int64
}

// OCRProjectionApplied acknowledges external Bleve mutations only after the
// coordinator verifies the current projection fence. Replaying the same
// batch is harmless because the source-revision acknowledgement is conditional.
type OCRProjectionApplied struct {
	Entries           []OCRIndexEntry
	SourceRevision    uint64
	ProjectionVersion uint64
	Complete          bool
}

// ReindexProjectionApplied carries a bounded reindex page. Candidate stage
// requests, cursor advancement, and receipt completion are committed together
// by the coordinator.
type ReindexProjectionApplied struct {
	Prepared          service.PreparedReindex
	ProjectionVersion uint64
}
type RepositoryEpochApplied struct {
	RepositoryID   uuid.UUID
	RequestedEpoch uint64
	TerminalError  string
}

// ProjectionTerminalFailure is the terminal counterpart of a bounded
// projection macro. It records only a stable code, never a worker error or a
// filesystem path.
type ProjectionTerminalFailure struct {
	Kind, Scope                       string
	SourceRevision, ProjectionVersion uint64
	TerminalError                     string
}

// CatalogDependencies are the typed capabilities needed by Catalog committers.
// They are captured once during runtime composition; processors never receive
// these write-side capabilities.
type CatalogDependencies struct {
	Face         service.FaceService
	Event        *event.Service
	Location     service.LocationService
	Indexing     service.AssetIndexingService
	Materializer *roematerializer.HashApplier
}

func applyRepositoryAssets(ctx context.Context, tx *sql.Tx, payload RepositoryAssetApplied) (Outcome, error) {
	queries := repo.New(tx)
	if payload.RepositoryID == uuid.Nil || payload.NodeID == uuid.Nil || payload.AssetID == uuid.Nil || payload.ContentID == uuid.Nil || payload.ObservationRevision <= 0 {
		return 0, errors.New("invalid repository asset result")
	}
	asset, err := queries.GetAssetByIDAny(ctx, payload.AssetID)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if asset.IsDeleted || asset.ContentID != payload.ContentID {
		return OutcomeStale, nil
	}
	node, err := queries.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{RepositoryID: payload.RepositoryID, NodeID: payload.NodeID})
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if node.Lifecycle != "active" || node.ObservationRevision != payload.ObservationRevision {
		return OutcomeStale, nil
	}
	location, err := queries.GetActiveAssetLocationByNode(ctx, payload.NodeID)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if location.AssetID != payload.AssetID || location.BoundObservationRevision != payload.ObservationRevision {
		return OutcomeStale, nil
	}
	if err := service.ApplyAssetActivationTx(ctx, tx, queries, payload.RepositoryID, payload.NodeID, payload.AssetID, payload.ContentID); err != nil {
		return 0, err
	}
	return OutcomeApplied, nil
}

func applyAssetStack(ctx context.Context, tx *sql.Tx, payload AssetStackApplied) (Outcome, error) {
	queries := repo.New(tx)
	if payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 {
		return 0, errors.New("invalid asset stack result")
	}
	var fence, pipelineVersion string
	var desired uint64
	if err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version FROM asset_pipeline_state WHERE asset_id=? AND stage='analyze'`, payload.AssetID.String()).Scan(&fence, &pipelineVersion, &desired); errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	} else if err != nil {
		return 0, err
	} else if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
		return OutcomeStale, nil
	}
	occurrence, err := queries.GetPreferredActiveAssetOccurrence(ctx, payload.AssetID)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if _, err := service.ApplyAutoDetectStacksTx(ctx, tx, queries, occurrence.RepositoryID); err != nil {
		return 0, err
	}
	return OutcomeApplied, nil
}

func applyEnrichment(ctx context.Context, tx *sql.Tx, payload EnrichmentApplied, face service.FaceService) (Outcome, error) {
	queries := repo.New(tx)
	if payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 {
		return 0, errors.New("invalid enrichment result")
	}
	var fence, pipelineVersion string
	var desired, applied uint64
	err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='enrich'`, payload.AssetID.String()).Scan(&fence, &pipelineVersion, &desired, &applied)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
		return OutcomeStale, nil
	}
	if applied >= desired {
		return OutcomeDuplicate, nil
	}
	if payload.PHash != nil {
		if err := service.ApplyEmbeddingTx(ctx, queries, payload.AssetID, payload.PHash.Type, payload.PHash.Model, payload.PHash.Vector, payload.PHash.IsPrimary); err != nil {
			return 0, err
		}
	}
	if payload.Semantic != nil {
		if err := service.ApplyEmbeddingTx(ctx, queries, payload.AssetID, payload.Semantic.Type, payload.Semantic.Model, payload.Semantic.Vector, payload.Semantic.IsPrimary); err != nil {
			return 0, err
		}
	}
	if payload.Aesthetic != nil {
		if err := service.ApplyAestheticScoreTx(ctx, queries, payload.AssetID, payload.Aesthetic.Score, payload.Aesthetic.Model); err != nil {
			return 0, err
		}
	}
	if payload.Species != nil {
		if err := service.ApplySpeciesPredictionsTx(ctx, queries, payload.AssetID, payload.Species.Predictions); err != nil {
			return 0, err
		}
	}
	if payload.OCR != nil {
		if err := service.ApplyOCRResultsTx(ctx, tx, queries, payload.AssetID, payload.OCR, 0); err != nil {
			return 0, err
		}
	}
	if payload.AITags != nil {
		if err := service.ApplyAIGeneratedTagsTx(ctx, queries, payload.AssetID, payload.AITags.Tags, []string{service.AssetTagSourceZeroshot}); err != nil {
			return 0, err
		}
	}
	if payload.Face != nil {
		if face == nil {
			return 0, errors.New("face enrichment commit is not configured")
		}
		if err := face.ApplyFaceResultsTx(ctx, tx, payload.AssetID, payload.Face.Payload, payload.Face.ImageData, payload.Face.ProcessingTimeMs); err != nil {
			return 0, err
		}
	}
	return OutcomeApplied, nil
}

func applyVideoFrameEmbeddings(ctx context.Context, tx *sql.Tx, payload VideoFrameEmbeddingsApplied) (Outcome, error) {
	queries := repo.New(tx)
	if payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 || payload.ModelID == "" || len(payload.Frames) == 0 {
		return 0, errors.New("invalid video frame embedding result")
	}
	var fence, pipelineVersion string
	var desired, applied uint64
	err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='enrich'`, payload.AssetID.String()).Scan(&fence, &pipelineVersion, &desired, &applied)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
		return OutcomeStale, nil
	}
	if applied >= desired {
		return OutcomeDuplicate, nil
	}
	frames := make([]service.VideoFrameEmbedding, 0, len(payload.Frames))
	for _, frame := range payload.Frames {
		frames = append(frames, service.VideoFrameEmbedding{FrameTsMs: frame.FrameTsMs, Vector: frame.Vector})
	}
	if err := service.ApplyVideoFrameEmbeddingsTx(ctx, queries, payload.AssetID, payload.ModelID, frames); err != nil {
		return 0, err
	}
	return OutcomeApplied, nil
}

func applyAssetDerivatives(ctx context.Context, tx *sql.Tx, payload AssetDerivativesApplied) (Outcome, error) {
	queries := repo.New(tx)
	if payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 {
		return 0, errors.New("invalid asset derivatives result")
	}
	var fence, pipelineVersion string
	var desired, applied uint64
	err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='derivatives'`, payload.AssetID.String()).Scan(&fence, &pipelineVersion, &desired, &applied)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
		return OutcomeStale, nil
	}
	if applied >= desired {
		return OutcomeDuplicate, nil
	}
	for _, artifact := range payload.Artifacts {
		if artifact.RepositoryID == uuid.Nil || artifact.Size == "" || artifact.StoragePath == "" || artifact.MimeType == "" {
			return 0, errors.New("invalid thumbnail artifact")
		}
		existing, lookupErr := queries.GetThumbnailByAssetAndSize(ctx, repo.GetThumbnailByAssetAndSizeParams{AssetID: payload.AssetID, Size: artifact.Size})
		if lookupErr == nil {
			if existing.RepositoryID != artifact.RepositoryID || existing.StoragePath != artifact.StoragePath || existing.MimeType != artifact.MimeType {
				return 0, fmt.Errorf("thumbnail artifact %s does not match the immutable catalog path", artifact.Size)
			}
			continue
		}
		if !errors.Is(lookupErr, sql.ErrNoRows) {
			return 0, lookupErr
		}
		if _, err := queries.CreateThumbnail(ctx, repo.CreateThumbnailParams{AssetID: payload.AssetID, Size: artifact.Size, StoragePath: artifact.StoragePath, MimeType: artifact.MimeType, RepositoryID: artifact.RepositoryID}); err != nil {
			return 0, err
		}
	}
	return OutcomeApplied, nil
}

func applyAssetMetadata(ctx context.Context, tx *sql.Tx, payload AssetMetadataApplied) (Outcome, error) {
	if payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 {
		return 0, errors.New("invalid asset metadata result")
	}
	var fence, pipelineVersion string
	var desired, applied uint64
	err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='analyze'`, payload.AssetID.String()).Scan(&fence, &pipelineVersion, &desired, &applied)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
		return OutcomeStale, nil
	}
	if applied >= desired {
		return OutcomeDuplicate, nil
	}
	if err := service.ApplyAssetExtractedMetadataTx(ctx, tx, repo.New(tx), payload.AssetID, payload.Metadata, payload.Common, json.RawMessage(payload.ExifRaw), payload.ComponentRelation); err != nil {
		return 0, err
	}
	return OutcomeApplied, nil
}

func applyRepositoryEpochs(ctx context.Context, tx *sql.Tx, payload RepositoryEpochApplied) (Outcome, error) {
	if payload.RepositoryID == uuid.Nil || payload.RequestedEpoch == 0 {
		return 0, errors.New("invalid repository epoch result")
	}
	var desired, applied uint64
	err := tx.QueryRowContext(ctx, `SELECT desired_epoch,applied_epoch FROM repository_observation_state WHERE repository_id=?`, payload.RepositoryID.String()).Scan(&desired, &applied)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if desired != payload.RequestedEpoch {
		return OutcomeStale, nil
	}
	if payload.TerminalError != "" {
		result, err := tx.ExecContext(ctx, `UPDATE repository_observation_state SET terminal_error=?,updated_at=? WHERE repository_id=? AND desired_epoch=? AND applied_epoch<?`, payload.TerminalError, time.Now().UTC().UnixMicro(), payload.RepositoryID.String(), payload.RequestedEpoch, payload.RequestedEpoch)
		if err != nil {
			return 0, err
		}
		changed, _ := result.RowsAffected()
		if changed == 0 {
			return OutcomeDuplicate, nil
		}
		return OutcomeApplied, nil
	}
	if applied < payload.RequestedEpoch {
		return OutcomeStale, nil
	}
	return OutcomeApplied, nil
}

func applyProjectionTerminalFailures(ctx context.Context, tx *sql.Tx, payload ProjectionTerminalFailure) (Outcome, error) {
	now := time.Now().UTC().UnixMicro()
	if payload.Kind == "" || payload.Scope == "" || payload.SourceRevision == 0 || payload.ProjectionVersion == 0 || payload.TerminalError == "" {
		return 0, errors.New("invalid projection terminal failure result")
	}
	var result sql.Result
	var err error
	switch payload.Kind {
	case "event":
		result, err = tx.ExecContext(ctx, `UPDATE event_projection_pipeline_state SET terminal_error=?,updated_at=? WHERE owner_id=? AND source_revision=? AND projection_version=? AND applied_revision<source_revision`, payload.TerminalError, now, payload.Scope, payload.SourceRevision, payload.ProjectionVersion)
	case "location_resolution":
		result, err = tx.ExecContext(ctx, `UPDATE location_resolution_pipeline_state SET terminal_error=?,updated_at=? WHERE scope=? AND source_revision=? AND projection_version=? AND applied_revision<projection_version`, payload.TerminalError, now, payload.Scope, payload.SourceRevision, payload.ProjectionVersion)
	case "ocr":
		result, err = tx.ExecContext(ctx, `UPDATE ocr_projection_pipeline_state SET terminal_error=?,updated_at=? WHERE scope=? AND source_revision=? AND projection_version=? AND applied_revision<projection_version`, payload.TerminalError, now, payload.Scope, payload.SourceRevision, payload.ProjectionVersion)
	case "asset_reindex":
		result, err = tx.ExecContext(ctx, `UPDATE catalog_operation_receipts SET state='failed',terminal_error=?,updated_at=? WHERE receipt_id=? AND kind='reindex' AND state='pending'`, payload.TerminalError, now, payload.Scope)
	case "location":
		parts := strings.SplitN(payload.Scope, ":", 2)
		if len(parts) != 2 {
			return 0, errors.New("invalid location projection terminal scope")
		}
		result, err = tx.ExecContext(ctx, `UPDATE location_projection_state SET terminal_error=?,updated_at=? WHERE repository_id=? AND owner_id=? AND source_revision=? AND published_revision<source_revision`, payload.TerminalError, now, parts[0], parts[1], payload.SourceRevision)
	default:
		return 0, fmt.Errorf("unsupported projection terminal kind %q", payload.Kind)
	}
	if err != nil {
		return 0, err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return OutcomeDuplicate, nil
	}
	return OutcomeApplied, nil
}

// projectionCommit is the typed union of projection results accepted by the
// Catalog committer. Exactly one field is populated by each typed Apply method.
// It is intentionally private: processors submit concrete results, never a
// generic payload plus an independently constructed identity key.
type projectionCommit struct {
	Location           *LocationProjectionApplied
	LocationResolution *LocationResolutionApplied
	OCR                *OCRProjectionApplied
	Reindex            *ReindexProjectionApplied
	Event              *EventProjectionApplied
}

func applyProjections(ctx context.Context, tx *sql.Tx, payload projectionCommit, eventService *event.Service, locationService service.LocationService, indexingService service.AssetIndexingService) (Outcome, error) {
	now := time.Now().UTC().UnixMicro()
	if payload.Location != nil {
		value := payload.Location
		if locationService == nil || value.Prepared.RepositoryID == uuid.Nil || value.Prepared.OwnerID <= 0 || value.Prepared.SourceRevision <= 0 || value.ProjectionVersion == 0 {
			return 0, errors.New("location projection commit is not configured")
		}
		var source, published int64
		err := tx.QueryRowContext(ctx, `SELECT source_revision,published_revision FROM location_projection_state WHERE repository_id=? AND owner_id=?`, value.Prepared.RepositoryID.String(), value.Prepared.OwnerID).Scan(&source, &published)
		if errors.Is(err, sql.ErrNoRows) {
			return OutcomeStale, nil
		}
		if err != nil {
			return 0, err
		}
		if source != value.Prepared.SourceRevision || uint64(source) != value.ProjectionVersion {
			return OutcomeStale, nil
		}
		if value.Prepared.Complete && published >= source {
			return OutcomeDuplicate, nil
		}
		if err := locationService.ApplyPreparedLocationRebuildTx(ctx, tx, value.Prepared); err != nil {
			if errors.Is(err, service.ErrLocationProjectionStale) {
				return OutcomeStale, nil
			}
			return 0, err
		}
		if value.Prepared.Complete {
			if _, err := tx.ExecContext(ctx, `UPDATE catalog_operation_receipts AS receipt SET state='completed',applied_version=desired_version,terminal_error=NULL,updated_at=? WHERE receipt.kind='rebuild' AND receipt.state='pending' AND EXISTS(SELECT 1 FROM location_projection_receipt_scopes link WHERE link.receipt_id=receipt.receipt_id AND link.repository_id=? AND link.owner_id=?) AND NOT EXISTS(SELECT 1 FROM location_projection_receipt_scopes link JOIN location_projection_state state ON state.repository_id=link.repository_id AND state.owner_id=link.owner_id WHERE link.receipt_id=receipt.receipt_id AND state.published_revision<link.desired_revision)`, now, value.Prepared.RepositoryID.String(), value.Prepared.OwnerID); err != nil {
				return 0, err
			}
		}
		return OutcomeApplied, nil
	}

	if payload.LocationResolution != nil {
		value := payload.LocationResolution
		if locationService == nil || value.Prepared.Revision <= 0 || value.ProjectionVersion == 0 {
			return 0, errors.New("location resolution commit is not configured")
		}
		var source, version, applied uint64
		err := tx.QueryRowContext(ctx, `SELECT source_revision,projection_version,applied_revision FROM location_resolution_pipeline_state WHERE scope='all'`).Scan(&source, &version, &applied)
		if errors.Is(err, sql.ErrNoRows) {
			return OutcomeStale, nil
		}
		if err != nil {
			return 0, err
		}
		if source != uint64(value.Prepared.Revision) || version != value.ProjectionVersion {
			return OutcomeStale, nil
		}
		if applied >= version {
			return OutcomeDuplicate, nil
		}
		if err := locationService.ApplyPreparedLocationResolutionTx(ctx, tx, value.Prepared, value.ProjectionVersion); err != nil {
			if errors.Is(err, service.ErrLocationProjectionStale) {
				return OutcomeStale, nil
			}
			return 0, err
		}
		return OutcomeApplied, nil
	}

	if payload.OCR != nil {
		value := payload.OCR
		if value.SourceRevision == 0 || value.ProjectionVersion == 0 {
			return 0, errors.New("OCR projection commit has no revision")
		}
		var source, version, applied uint64
		err := tx.QueryRowContext(ctx, `SELECT source_revision,projection_version,applied_revision FROM ocr_projection_pipeline_state WHERE scope='all'`).Scan(&source, &version, &applied)
		if errors.Is(err, sql.ErrNoRows) {
			return OutcomeStale, nil
		}
		if err != nil {
			return 0, err
		}
		if source != value.SourceRevision || version != value.ProjectionVersion {
			return OutcomeStale, nil
		}
		if applied >= version {
			return OutcomeDuplicate, nil
		}
		queries := repo.New(tx)
		for _, entry := range value.Entries {
			if entry.AssetID == uuid.Nil || entry.Revision <= 0 {
				return 0, errors.New("invalid OCR projection entry")
			}
			if _, err := queries.AcknowledgeOCRIndexOutbox(ctx, repo.AcknowledgeOCRIndexOutboxParams{AssetID: entry.AssetID, Revision: entry.Revision}); err != nil {
				return 0, fmt.Errorf("acknowledge OCR index revision %s@%d: %w", entry.AssetID, entry.Revision, err)
			}
		}
		if value.Complete {
			if _, err := tx.ExecContext(ctx, `UPDATE ocr_projection_pipeline_state SET applied_revision=projection_version,terminal_error=NULL,updated_at=? WHERE scope='all' AND source_revision=? AND projection_version=? AND applied_revision<projection_version`, now, source, version); err != nil {
				return 0, err
			}
		}
		return OutcomeApplied, nil
	}

	if payload.Reindex != nil {
		value := payload.Reindex
		if indexingService == nil || value.Prepared.ReceiptID == uuid.Nil || value.Prepared.RequestedRevision == 0 || value.ProjectionVersion == 0 {
			return 0, errors.New("reindex projection commit is not configured")
		}
		var requested, applied uint64
		err := tx.QueryRowContext(ctx, `SELECT requested_revision,applied_revision FROM asset_reindex_requests WHERE receipt_id=?`, value.Prepared.ReceiptID.String()).Scan(&requested, &applied)
		if errors.Is(err, sql.ErrNoRows) {
			return OutcomeStale, nil
		}
		if err != nil {
			return 0, err
		}
		if requested != value.Prepared.RequestedRevision || requested != value.ProjectionVersion {
			return OutcomeStale, nil
		}
		if applied >= requested {
			return OutcomeDuplicate, nil
		}
		if err := indexingService.ApplyPreparedReindexTx(ctx, tx, value.Prepared); err != nil {
			if errors.Is(err, service.ErrReindexProjectionStale) {
				return OutcomeStale, nil
			}
			return 0, err
		}
		return OutcomeApplied, nil
	}

	if payload.Event != nil {
		value := payload.Event
		if eventService == nil || value.Prepared.OwnerID <= 0 || value.Prepared.SourceRevision <= 0 || value.ProjectionVersion == 0 {
			return 0, errors.New("event projection commit is not configured")
		}
		ownerID := value.Prepared.OwnerID
		var source, version, applied uint64
		if err := tx.QueryRowContext(ctx, `SELECT source_revision,projection_version,applied_revision FROM event_projection_pipeline_state WHERE owner_id=?`, ownerID).Scan(&source, &version, &applied); errors.Is(err, sql.ErrNoRows) {
			return OutcomeStale, nil
		} else if err != nil {
			return 0, err
		} else if source != uint64(value.Prepared.SourceRevision) || version != value.ProjectionVersion {
			return OutcomeStale, nil
		} else if applied >= source {
			return OutcomeDuplicate, nil
		}
		if err := eventService.ApplyPreparedRebuildTx(ctx, tx, value.Prepared); err != nil {
			if errors.Is(err, event.ErrStaleRevision) {
				return OutcomeStale, nil
			}
			return 0, err
		}
		result, err := tx.ExecContext(ctx, `UPDATE event_projection_pipeline_state SET applied_revision=?,cursor=NULL,terminal_error=NULL,updated_at=? WHERE owner_id=? AND source_revision=? AND projection_version=?`, source, now, ownerID, source, version)
		if err != nil {
			return 0, err
		}
		changed, _ := result.RowsAffected()
		if changed == 0 {
			return OutcomeStale, nil
		}
		return OutcomeApplied, nil
	}

	return 0, errors.New("projection commit has no typed payload")
}

func applyOperationReceipts(ctx context.Context, tx *sql.Tx, payload OperationReceiptApplied) (Outcome, error) {
	now := time.Now().UTC().UnixMicro()
	if payload.ReceiptID == uuid.Nil || payload.Kind == "" {
		return 0, errors.New("invalid operation receipt result")
	}
	var kind string
	err := tx.QueryRowContext(ctx, `SELECT kind FROM catalog_operation_receipts WHERE receipt_id=?`, payload.ReceiptID.String()).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if kind != payload.Kind {
		return 0, errors.New("operation receipt kind does not match receipt")
	}
	state, applied := "completed", 1
	if payload.TerminalError != "" {
		state, applied = "failed", 0
	}
	result, err := tx.ExecContext(ctx, `UPDATE catalog_operation_receipts SET state=?,applied_version=?,terminal_error=NULLIF(?,''),updated_at=? WHERE receipt_id=? AND kind=? AND state='pending'`, state, applied, payload.TerminalError, now, payload.ReceiptID.String(), payload.Kind)
	if err != nil {
		return 0, err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return OutcomeDuplicate, nil
	}
	return OutcomeApplied, nil
}

func applyAssetStages(ctx context.Context, tx *sql.Tx, payload AssetStageApplied) (Outcome, error) {
	now := time.Now().UTC().UnixMicro()
	if payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 {
		return 0, errors.New("invalid asset stage result")
	}
	switch payload.Stage {
	case string(pipeline.StageAnalyze), string(pipeline.StageDerivatives), string(pipeline.StageTranscode), string(pipeline.StageEnrich):
	default:
		return 0, fmt.Errorf("invalid asset stage %q", payload.Stage)
	}
	var fence, pipelineVersion string
	var desired, applied uint64
	err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage=?`, payload.AssetID.String(), payload.Stage).Scan(&fence, &pipelineVersion, &desired, &applied)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
		return OutcomeStale, nil
	}
	if payload.TerminalError != "" {
		if applied >= desired {
			return OutcomeDuplicate, nil
		}
		result, err := tx.ExecContext(ctx, `UPDATE asset_pipeline_state SET terminal_error=?,updated_at=? WHERE asset_id=? AND stage=? AND source_content_id=? AND pipeline_version=? AND desired_version=? AND applied_version<?`, payload.TerminalError, now, payload.AssetID.String(), payload.Stage, payload.SourceFence.String(), payload.PipelineVersion, desired, desired)
		if err != nil {
			return 0, err
		}
		changed, _ := result.RowsAffected()
		if changed == 0 {
			return OutcomeStale, nil
		}
		if err := refreshAssetProductStatus(ctx, tx, payload.AssetID, now); err != nil {
			return 0, err
		}
		if err := settleAssetPipelineReceipts(ctx, tx, payload.AssetID, now); err != nil {
			return 0, err
		}
		return OutcomeApplied, nil
	}
	if applied >= desired {
		if err := settleAssetPipelineReceipts(ctx, tx, payload.AssetID, now); err != nil {
			return 0, err
		}
		return OutcomeDuplicate, nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE asset_pipeline_state SET applied_version=?,terminal_error=NULL,updated_at=? WHERE asset_id=? AND stage=? AND source_content_id=? AND pipeline_version=? AND desired_version=? AND applied_version<?`, desired, now, payload.AssetID.String(), payload.Stage, payload.SourceFence.String(), payload.PipelineVersion, desired, desired)
	if err != nil {
		return 0, err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return OutcomeStale, nil
	}
	if err := refreshAssetProductStatus(ctx, tx, payload.AssetID, now); err != nil {
		return 0, err
	}
	if err := settleAssetPipelineReceipts(ctx, tx, payload.AssetID, now); err != nil {
		return 0, err
	}
	return OutcomeApplied, nil
}

func settleAssetPipelineReceipts(ctx context.Context, tx *sql.Tx, assetID uuid.UUID, now int64) error {
	if _, err := tx.ExecContext(ctx, `
		UPDATE catalog_operation_receipts AS receipt
		SET state='failed',terminal_error=(
			SELECT stage.terminal_error
			FROM asset_pipeline_receipt_stages link
			JOIN asset_pipeline_state stage ON stage.asset_id=link.asset_id AND stage.stage=link.stage
			WHERE link.receipt_id=receipt.receipt_id AND link.asset_id=? AND stage.terminal_error IS NOT NULL
			ORDER BY stage.updated_at DESC LIMIT 1
		),updated_at=?
		WHERE receipt.state='pending' AND receipt.kind IN ('reprocess','retry','reindex')
		  AND EXISTS (
			SELECT 1 FROM asset_pipeline_receipt_stages link
			JOIN asset_pipeline_state stage ON stage.asset_id=link.asset_id AND stage.stage=link.stage
			WHERE link.receipt_id=receipt.receipt_id AND link.asset_id=? AND stage.terminal_error IS NOT NULL
		  )`, assetID.String(), now, assetID.String()); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE catalog_operation_receipts AS receipt
		SET state='completed',applied_version=desired_version,terminal_error=NULL,updated_at=?
		WHERE receipt.state='pending' AND receipt.kind IN ('reprocess','retry','reindex')
		  AND EXISTS (
			SELECT 1 FROM asset_pipeline_receipt_stages link
			WHERE link.receipt_id=receipt.receipt_id AND link.asset_id=?
		  )
		  AND NOT EXISTS (
			SELECT 1
			FROM asset_pipeline_receipt_stages link
			JOIN asset_pipeline_state stage
			  ON stage.asset_id=link.asset_id AND stage.stage=link.stage
			WHERE link.receipt_id=receipt.receipt_id
			  AND stage.applied_version<link.desired_version
		  )`, now, assetID.String())
	return err
}

func refreshAssetProductStatus(ctx context.Context, tx *sql.Tx, assetID uuid.UUID, now int64) error {
	var failed, pending bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM asset_pipeline_state WHERE asset_id=? AND desired_version>applied_version AND terminal_error IS NOT NULL),EXISTS(SELECT 1 FROM asset_pipeline_state WHERE asset_id=? AND desired_version>applied_version AND terminal_error IS NULL)`, assetID.String(), assetID.String()).Scan(&failed, &pending); err != nil {
		return err
	}
	status := `{"state":"completed"}`
	if failed {
		status = `{"state":"failed"}`
	} else if pending {
		status = `{"state":"processing"}`
	}
	_, err := tx.ExecContext(ctx, `UPDATE assets SET status=?,updated_at=? WHERE asset_id=?`, status, now, assetID.String())
	return err
}

func applyIngestReceipts(ctx context.Context, tx *sql.Tx, payload IngestReceiptApplied, commitID uuid.UUID) (Outcome, error) {
	now := time.Now().UTC().UnixMicro()
	if payload.ReceiptID == uuid.Nil || commitID == uuid.Nil {
		return 0, errors.New("invalid ingest receipt result")
	}
	var subject string
	err := tx.QueryRowContext(ctx, `SELECT subject_id FROM catalog_operation_receipts WHERE receipt_id=? AND kind='ingest'`, payload.ReceiptID.String()).Scan(&subject)
	if errors.Is(err, sql.ErrNoRows) {
		return OutcomeStale, nil
	}
	if err != nil {
		return 0, err
	}
	if subject != commitID.String() {
		return 0, errors.New("ingest receipt fence does not match receipt subject")
	}
	state := "completed"
	applied := 1
	if payload.TerminalError != "" {
		state = "failed"
		applied = 0
	}
	result, err := tx.ExecContext(ctx, `UPDATE catalog_operation_receipts SET state=?,applied_version=?,terminal_error=NULLIF(?,''),updated_at=? WHERE receipt_id=? AND kind='ingest' AND state='pending'`, state, applied, payload.TerminalError, now, payload.ReceiptID.String())
	if err != nil {
		return 0, err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return OutcomeDuplicate, nil
	}
	return OutcomeApplied, nil
}
