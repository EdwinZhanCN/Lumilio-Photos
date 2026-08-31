package commit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
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

const (
	FamilyAssetStage             = "asset_stage"
	FamilyAssetMetadata          = "asset_metadata"
	FamilyAssetDerivatives       = "asset_derivatives"
	FamilyAssetStack             = "asset_stack"
	FamilyRepositoryAsset        = "repository_asset"
	FamilyRepositoryKnownContent = "repository_known_content"
	FamilyRepositoryHash         = "repository_hash"
	FamilyVideoFrameEmbeddings   = "video_frame_embeddings"
	FamilyEnrichment             = "asset_enrichment"
	FamilyIngestReceipt          = "ingest_receipt"
	FamilyOperationReceipt       = "operation_receipt"
	FamilyProjectionStage        = "projection_stage"
	FamilyRepositoryEpoch        = "repository_epoch"
	FamilyProjectionTerminal     = "projection_terminal"
)

type AssetStageApplied struct {
	AssetID, SourceFence   uuid.UUID
	Stage, PipelineVersion string
	DesiredVersion         uint64
	// TerminalError is a stable public-safe failure code. It is set only by
	// the final macro-attempt handler; retry/reprocess creates a newer desired
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
// batch is harmless because the outbox acknowledgement is conditional.
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

func CatalogHandlers() map[string]Handler {
	return CatalogHandlersWithServices(nil, nil)
}

// CatalogHandlersWithFace wires the face persistence capability into the
// catalog handler set. It is useful for unit tests that do not exercise Event
// or sidecar projection publication; production supplies every capability.
func CatalogHandlersWithFace(face service.FaceService) map[string]Handler {
	return CatalogHandlersWithServices(face, nil)
}

// CatalogHandlersWithServices wires all asynchronous catalog capabilities into
// one handler set. Every capability receives the coordinator transaction; no
// worker or handler opens a second catalog transaction.
func CatalogHandlersWithServices(face service.FaceService, eventService *event.Service) map[string]Handler {
	return CatalogHandlersWithAllServices(face, eventService, nil)
}

// CatalogHandlersWithAllServices wires every asynchronous projection commit
// capability into the coordinator. A nil capability is valid only for tests
// that do not submit that projection family.
func CatalogHandlersWithAllServices(face service.FaceService, eventService *event.Service, locationService service.LocationService, indexingServices ...service.AssetIndexingService) map[string]Handler {
	var indexingService service.AssetIndexingService
	if len(indexingServices) > 0 {
		indexingService = indexingServices[0]
	}
	enrichment := func(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
		return applyEnrichment(ctx, tx, intents, face)
	}
	projections := func(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
		return applyProjections(ctx, tx, intents, eventService, locationService, indexingService)
	}
	return map[string]Handler{
		FamilyAssetStage:           OutcomeHandler(applyAssetStages),
		FamilyAssetMetadata:        OutcomeHandler(applyAssetMetadata),
		FamilyAssetDerivatives:     OutcomeHandler(applyAssetDerivatives),
		FamilyAssetStack:           OutcomeHandler(applyAssetStack),
		FamilyRepositoryAsset:      OutcomeHandler(applyRepositoryAssets),
		FamilyVideoFrameEmbeddings: OutcomeHandler(applyVideoFrameEmbeddings),
		FamilyEnrichment:           OutcomeHandler(enrichment),
		FamilyIngestReceipt:        OutcomeHandler(applyIngestReceipts),
		FamilyOperationReceipt:     OutcomeHandler(applyOperationReceipts),
		FamilyProjectionStage:      OutcomeHandler(projections),
		FamilyRepositoryEpoch:      OutcomeHandler(applyRepositoryEpochs),
		FamilyProjectionTerminal:   OutcomeHandler(applyProjectionTerminalFailures),
	}
}

// CatalogHandlersWithRepositoryMaterializer adds the source-publication
// boundary to the standard handler set. Keeping it explicit lets unit tests
// omit filesystem/source materialization while production has one canonical
// writer for it.
func CatalogHandlersWithRepositoryMaterializer(face service.FaceService, eventService *event.Service, locationService service.LocationService, indexingService service.AssetIndexingService, materializer *roematerializer.HashApplier) map[string]Handler {
	handlers := CatalogHandlersWithAllServices(face, eventService, locationService, indexingService)
	if materializer != nil {
		handlers[FamilyRepositoryKnownContent] = applyRepositoryKnownContent(materializer)
		handlers[FamilyRepositoryHash] = applyRepositoryHashes(materializer)
	}
	return handlers
}

func applyRepositoryHashes(materializer *roematerializer.HashApplier) OutcomeHandler {
	return OutcomeHandler(func(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
		outcomes := make([]Outcome, len(intents))
		queries := repo.New(tx)
		for index, intent := range intents {
			payload, ok := intent.Payload.(RepositoryHashApplied)
			prepared := payload.Prepared
			if !ok || prepared.Node.NodeID == uuid.Nil || prepared.Node.RepositoryID == uuid.Nil || prepared.Node.ObservationRevision <= 0 || prepared.Observation.ObservationToken == "" || intent.Key.Subject != prepared.Node.NodeID.String() || intent.Key.Fence != prepared.Observation.ObservationToken || intent.Key.Stage != "hash" || intent.Key.DesiredVersion == 0 {
				return nil, errors.New("repository hash commit key does not match payload")
			}
			result, err := materializer.ApplyHash(ctx, tx, prepared)
			if err != nil {
				return nil, err
			}
			if result.Code == roematerializer.ResultStale || result.AssetID == uuid.Nil {
				outcomes[index] = OutcomeStale
				continue
			}
			if err := service.ApplyAssetActivationTx(ctx, tx, queries, result.RepositoryID, result.NodeID, result.AssetID, result.ContentID); err != nil {
				return nil, err
			}
			if result.Code == roematerializer.ResultNoop {
				outcomes[index] = OutcomeDuplicate
			} else {
				outcomes[index] = OutcomeApplied
			}
		}
		return outcomes, nil
	})
}

func applyRepositoryKnownContent(materializer *roematerializer.HashApplier) OutcomeHandler {
	return OutcomeHandler(func(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
		outcomes := make([]Outcome, len(intents))
		queries := repo.New(tx)
		for index, intent := range intents {
			payload, ok := intent.Payload.(RepositoryKnownContentApplied)
			fact := payload.Fact
			if !ok || fact.RepositoryID == uuid.Nil || fact.SourceEventKey == "" || intent.Key.Subject != fact.RepositoryID.String() || intent.Key.Fence != fact.SourceEventKey || intent.Key.Stage != "known_content" || intent.Key.DesiredVersion != 1 {
				return nil, errors.New("repository known-content commit key does not match payload")
			}
			result, err := materializer.ApplyKnownContent(ctx, tx, fact)
			if err != nil {
				return nil, err
			}
			if result.Code == roematerializer.ResultStale || result.AssetID == uuid.Nil {
				outcomes[index] = OutcomeStale
				continue
			}
			if err := service.ApplyAssetActivationTx(ctx, tx, queries, result.RepositoryID, result.NodeID, result.AssetID, result.ContentID); err != nil {
				return nil, err
			}
			if result.Code == roematerializer.ResultNoop {
				outcomes[index] = OutcomeDuplicate
			} else {
				outcomes[index] = OutcomeApplied
			}
		}
		return outcomes, nil
	})
}

func applyRepositoryAssets(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	queries := repo.New(tx)
	for index, intent := range intents {
		payload, ok := intent.Payload.(RepositoryAssetApplied)
		if !ok || payload.RepositoryID == uuid.Nil || payload.NodeID == uuid.Nil || payload.AssetID == uuid.Nil || payload.ContentID == uuid.Nil || payload.ObservationRevision <= 0 {
			return nil, errors.New("invalid repository asset commit payload")
		}
		if intent.Key.Subject != payload.AssetID.String() || intent.Key.Fence != payload.ContentID.String() || intent.Key.Stage != "repository_asset" || intent.Key.DesiredVersion == 0 {
			return nil, errors.New("repository asset commit key does not match payload")
		}
		asset, err := queries.GetAssetByIDAny(ctx, payload.AssetID)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if asset.IsDeleted || asset.ContentID != payload.ContentID {
			outcomes[index] = OutcomeStale
			continue
		}
		node, err := queries.GetRepositoryNode(ctx, repo.GetRepositoryNodeParams{RepositoryID: payload.RepositoryID, NodeID: payload.NodeID})
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if node.Lifecycle != "active" || node.ObservationRevision != payload.ObservationRevision {
			outcomes[index] = OutcomeStale
			continue
		}
		location, err := queries.GetActiveAssetLocationByNode(ctx, payload.NodeID)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if location.AssetID != payload.AssetID || location.BoundObservationRevision != payload.ObservationRevision {
			outcomes[index] = OutcomeStale
			continue
		}
		if err := service.ApplyAssetActivationTx(ctx, tx, queries, payload.RepositoryID, payload.NodeID, payload.AssetID, payload.ContentID); err != nil {
			return nil, err
		}
		outcomes[index] = OutcomeApplied
	}
	return outcomes, nil
}

func applyAssetStack(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	queries := repo.New(tx)
	for index, intent := range intents {
		payload, ok := intent.Payload.(AssetStackApplied)
		if !ok || payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 {
			return nil, errors.New("invalid asset stack commit payload")
		}
		if !assetIntentKeyMatches(intent, payload.AssetID, payload.SourceFence, "stack", payload.DesiredVersion) {
			return nil, errors.New("asset stack commit key does not match payload")
		}
		var fence, pipelineVersion string
		var desired uint64
		if err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version FROM asset_pipeline_state WHERE asset_id=? AND stage='analyze'`, payload.AssetID.String()).Scan(&fence, &pipelineVersion, &desired); errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		} else if err != nil {
			return nil, err
		} else if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
			outcomes[index] = OutcomeStale
			continue
		}
		occurrence, err := queries.GetPreferredActiveAssetOccurrence(ctx, payload.AssetID)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if _, err := service.ApplyAutoDetectStacksTx(ctx, tx, queries, occurrence.RepositoryID); err != nil {
			return nil, err
		}
		outcomes[index] = OutcomeApplied
	}
	return outcomes, nil
}

func applyEnrichment(ctx context.Context, tx *sql.Tx, intents []Intent, face service.FaceService) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	queries := repo.New(tx)
	for index, intent := range intents {
		payload, ok := intent.Payload.(EnrichmentApplied)
		if !ok || payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 {
			return nil, errors.New("invalid enrichment commit payload")
		}
		if !assetIntentKeyMatches(intent, payload.AssetID, payload.SourceFence, "enrich", payload.DesiredVersion) {
			return nil, errors.New("enrichment commit key does not match payload")
		}
		var fence, pipelineVersion string
		var desired, applied uint64
		err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='enrich'`, payload.AssetID.String()).Scan(&fence, &pipelineVersion, &desired, &applied)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
			outcomes[index] = OutcomeStale
			continue
		}
		if applied >= desired {
			outcomes[index] = OutcomeDuplicate
			continue
		}
		if payload.PHash != nil {
			if err := service.ApplyEmbeddingTx(ctx, queries, payload.AssetID, payload.PHash.Type, payload.PHash.Model, payload.PHash.Vector, payload.PHash.IsPrimary); err != nil {
				return nil, err
			}
		}
		if payload.Semantic != nil {
			if err := service.ApplyEmbeddingTx(ctx, queries, payload.AssetID, payload.Semantic.Type, payload.Semantic.Model, payload.Semantic.Vector, payload.Semantic.IsPrimary); err != nil {
				return nil, err
			}
		}
		if payload.Aesthetic != nil {
			if err := service.ApplyAestheticScoreTx(ctx, queries, payload.AssetID, payload.Aesthetic.Score, payload.Aesthetic.Model); err != nil {
				return nil, err
			}
		}
		if payload.Species != nil {
			if err := service.ApplySpeciesPredictionsTx(ctx, queries, payload.AssetID, payload.Species.Predictions); err != nil {
				return nil, err
			}
		}
		if payload.OCR != nil {
			if err := service.ApplyOCRResultsTx(ctx, tx, queries, payload.AssetID, payload.OCR, 0); err != nil {
				return nil, err
			}
		}
		if payload.AITags != nil {
			if err := service.ApplyAIGeneratedTagsTx(ctx, queries, payload.AssetID, payload.AITags.Tags, []string{service.AssetTagSourceZeroshot}); err != nil {
				return nil, err
			}
		}
		if payload.Face != nil {
			if face == nil {
				return nil, errors.New("face enrichment commit is not configured")
			}
			if err := face.ApplyFaceResultsTx(ctx, tx, payload.AssetID, payload.Face.Payload, payload.Face.ImageData, payload.Face.ProcessingTimeMs); err != nil {
				return nil, err
			}
		}
		outcomes[index] = OutcomeApplied
	}
	return outcomes, nil
}

func applyVideoFrameEmbeddings(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	queries := repo.New(tx)
	for index, intent := range intents {
		payload, ok := intent.Payload.(VideoFrameEmbeddingsApplied)
		if !ok || payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 || payload.ModelID == "" || len(payload.Frames) == 0 {
			return nil, errors.New("invalid video frame embedding commit payload")
		}
		if !assetIntentKeyMatches(intent, payload.AssetID, payload.SourceFence, "enrich", payload.DesiredVersion) {
			return nil, errors.New("video frame embedding commit key does not match payload")
		}
		var fence, pipelineVersion string
		var desired, applied uint64
		err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='enrich'`, payload.AssetID.String()).Scan(&fence, &pipelineVersion, &desired, &applied)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
			outcomes[index] = OutcomeStale
			continue
		}
		if applied >= desired {
			outcomes[index] = OutcomeDuplicate
			continue
		}
		frames := make([]service.VideoFrameEmbedding, 0, len(payload.Frames))
		for _, frame := range payload.Frames {
			frames = append(frames, service.VideoFrameEmbedding{FrameTsMs: frame.FrameTsMs, Vector: frame.Vector})
		}
		if err := service.ApplyVideoFrameEmbeddingsTx(ctx, queries, payload.AssetID, payload.ModelID, frames); err != nil {
			return nil, err
		}
		outcomes[index] = OutcomeApplied
	}
	return outcomes, nil
}

func applyAssetDerivatives(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	queries := repo.New(tx)
	for index, intent := range intents {
		payload, ok := intent.Payload.(AssetDerivativesApplied)
		if !ok || payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 {
			return nil, errors.New("invalid asset derivatives commit payload")
		}
		if !assetIntentKeyMatches(intent, payload.AssetID, payload.SourceFence, "derivatives", payload.DesiredVersion) {
			return nil, errors.New("asset derivatives commit key does not match payload")
		}
		var fence, pipelineVersion string
		var desired, applied uint64
		err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='derivatives'`, payload.AssetID.String()).Scan(&fence, &pipelineVersion, &desired, &applied)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
			outcomes[index] = OutcomeStale
			continue
		}
		if applied >= desired {
			outcomes[index] = OutcomeDuplicate
			continue
		}
		for _, artifact := range payload.Artifacts {
			if artifact.RepositoryID == uuid.Nil || artifact.Size == "" || artifact.StoragePath == "" || artifact.MimeType == "" {
				return nil, errors.New("invalid thumbnail artifact")
			}
			existing, lookupErr := queries.GetThumbnailByAssetAndSize(ctx, repo.GetThumbnailByAssetAndSizeParams{AssetID: payload.AssetID, Size: artifact.Size})
			if lookupErr == nil {
				if existing.RepositoryID != artifact.RepositoryID || existing.StoragePath != artifact.StoragePath || existing.MimeType != artifact.MimeType {
					return nil, fmt.Errorf("thumbnail artifact %s does not match the immutable catalog path", artifact.Size)
				}
				continue
			}
			if !errors.Is(lookupErr, sql.ErrNoRows) {
				return nil, lookupErr
			}
			if _, err := queries.CreateThumbnail(ctx, repo.CreateThumbnailParams{AssetID: payload.AssetID, Size: artifact.Size, StoragePath: artifact.StoragePath, MimeType: artifact.MimeType, RepositoryID: artifact.RepositoryID}); err != nil {
				return nil, err
			}
		}
		outcomes[index] = OutcomeApplied
	}
	return outcomes, nil
}

func applyAssetMetadata(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	for index, intent := range intents {
		payload, ok := intent.Payload.(AssetMetadataApplied)
		if !ok || payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 {
			return nil, errors.New("invalid asset metadata commit payload")
		}
		if !assetIntentKeyMatches(intent, payload.AssetID, payload.SourceFence, "metadata", payload.DesiredVersion) {
			return nil, errors.New("asset metadata commit key does not match payload")
		}
		var fence, pipelineVersion string
		var desired, applied uint64
		err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage='analyze'`, payload.AssetID.String()).Scan(&fence, &pipelineVersion, &desired, &applied)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
			outcomes[index] = OutcomeStale
			continue
		}
		if applied >= desired {
			outcomes[index] = OutcomeDuplicate
			continue
		}
		if err := service.ApplyAssetExtractedMetadataTx(ctx, tx, repo.New(tx), payload.AssetID, payload.Metadata, payload.Common, json.RawMessage(payload.ExifRaw), payload.ComponentRelation); err != nil {
			return nil, err
		}
		outcomes[index] = OutcomeApplied
	}
	return outcomes, nil
}

func applyRepositoryEpochs(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	for index, intent := range intents {
		payload, ok := intent.Payload.(RepositoryEpochApplied)
		if !ok || payload.RepositoryID == uuid.Nil || payload.RequestedEpoch == 0 {
			return nil, errors.New("invalid repository epoch commit payload")
		}
		if intent.Key.Subject != payload.RepositoryID.String() || intent.Key.Fence != strconv.FormatUint(payload.RequestedEpoch, 10) || intent.Key.Stage != "repository_scan" || intent.Key.DesiredVersion != payload.RequestedEpoch {
			return nil, errors.New("repository epoch commit key does not match payload")
		}
		var desired, applied uint64
		err := tx.QueryRowContext(ctx, `SELECT desired_epoch,applied_epoch FROM repository_observation_state WHERE repository_id=?`, payload.RepositoryID.String()).Scan(&desired, &applied)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if desired != payload.RequestedEpoch {
			outcomes[index] = OutcomeStale
		} else if payload.TerminalError != "" {
			result, err := tx.ExecContext(ctx, `UPDATE repository_observation_state SET terminal_error=?,updated_at=? WHERE repository_id=? AND desired_epoch=? AND applied_epoch<?`, payload.TerminalError, time.Now().UTC().UnixMicro(), payload.RepositoryID.String(), payload.RequestedEpoch, payload.RequestedEpoch)
			if err != nil {
				return nil, err
			}
			changed, _ := result.RowsAffected()
			if changed == 0 {
				outcomes[index] = OutcomeDuplicate
			} else {
				outcomes[index] = OutcomeApplied
			}
		} else if applied < payload.RequestedEpoch {
			outcomes[index] = OutcomeStale
		} else {
			outcomes[index] = OutcomeApplied
		}
	}
	return outcomes, nil
}

func applyProjectionTerminalFailures(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	now := time.Now().UTC().UnixMicro()
	for index, intent := range intents {
		payload, ok := intent.Payload.(ProjectionTerminalFailure)
		if !ok || payload.Kind == "" || payload.Scope == "" || payload.SourceRevision == 0 || payload.ProjectionVersion == 0 || payload.TerminalError == "" {
			return nil, errors.New("invalid projection terminal failure payload")
		}
		if !projectionIntentKeyMatches(intent, payload.Kind, payload.Scope, payload.SourceRevision, payload.ProjectionVersion) {
			return nil, errors.New("projection terminal failure key does not match payload")
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
				return nil, errors.New("invalid location projection terminal scope")
			}
			result, err = tx.ExecContext(ctx, `UPDATE location_projection_state SET terminal_error=?,updated_at=? WHERE repository_id=? AND owner_id=? AND source_revision=? AND published_revision<source_revision`, payload.TerminalError, now, parts[0], parts[1], payload.SourceRevision)
		default:
			return nil, fmt.Errorf("unsupported projection terminal kind %q", payload.Kind)
		}
		if err != nil {
			return nil, err
		}
		changed, _ := result.RowsAffected()
		if changed == 0 {
			outcomes[index] = OutcomeDuplicate
		} else {
			outcomes[index] = OutcomeApplied
		}
	}
	return outcomes, nil
}

func applyProjections(ctx context.Context, tx *sql.Tx, intents []Intent, eventService *event.Service, locationService service.LocationService, indexingService service.AssetIndexingService) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	now := time.Now().UTC().UnixMicro()
	for index, intent := range intents {
		if payload, ok := intent.Payload.(LocationProjectionApplied); ok {
			if locationService == nil || payload.Prepared.RepositoryID == uuid.Nil || payload.Prepared.OwnerID <= 0 || payload.Prepared.SourceRevision <= 0 || payload.ProjectionVersion == 0 {
				return nil, errors.New("location projection commit is not configured")
			}
			scope := payload.Prepared.RepositoryID.String() + ":" + strconv.FormatInt(int64(payload.Prepared.OwnerID), 10)
			if !projectionIntentKeyMatches(intent, "location", scope, uint64(payload.Prepared.SourceRevision), payload.ProjectionVersion) {
				return nil, errors.New("location projection commit key does not match payload")
			}
			var source, published int64
			err := tx.QueryRowContext(ctx, `SELECT source_revision,published_revision FROM location_projection_state WHERE repository_id=? AND owner_id=?`, payload.Prepared.RepositoryID.String(), payload.Prepared.OwnerID).Scan(&source, &published)
			if errors.Is(err, sql.ErrNoRows) {
				outcomes[index] = OutcomeStale
				continue
			}
			if err != nil {
				return nil, err
			}
			if source != payload.Prepared.SourceRevision || uint64(source) != payload.ProjectionVersion {
				outcomes[index] = OutcomeStale
				continue
			}
			if payload.Prepared.Complete && published >= source {
				outcomes[index] = OutcomeDuplicate
				continue
			}
			if err := locationService.ApplyPreparedLocationRebuildTx(ctx, tx, payload.Prepared); err != nil {
				if errors.Is(err, service.ErrLocationProjectionStale) {
					outcomes[index] = OutcomeStale
					continue
				}
				return nil, err
			}
			if payload.Prepared.Complete {
				parts := strings.SplitN(intent.Key.Subject, ":", 2)
				if len(parts) != 2 {
					return nil, errors.New("invalid location projection scope")
				}
				if _, err := tx.ExecContext(ctx, `UPDATE catalog_operation_receipts AS receipt SET state='completed',applied_version=desired_version,terminal_error=NULL,updated_at=? WHERE receipt.kind='rebuild' AND receipt.state='pending' AND EXISTS(SELECT 1 FROM location_projection_receipt_scopes link WHERE link.receipt_id=receipt.receipt_id AND link.repository_id=? AND link.owner_id=?) AND NOT EXISTS(SELECT 1 FROM location_projection_receipt_scopes link JOIN location_projection_state state ON state.repository_id=link.repository_id AND state.owner_id=link.owner_id WHERE link.receipt_id=receipt.receipt_id AND state.published_revision<link.desired_revision)`, now, parts[0], payload.Prepared.OwnerID); err != nil {
					return nil, err
				}
			}
			outcomes[index] = OutcomeApplied
			continue
		}
		if payload, ok := intent.Payload.(LocationResolutionApplied); ok {
			if locationService == nil || payload.Prepared.Revision <= 0 || payload.ProjectionVersion == 0 {
				return nil, errors.New("location resolution commit is not configured")
			}
			if !projectionIntentKeyMatches(intent, "location_resolution", "all", uint64(payload.Prepared.Revision), payload.ProjectionVersion) {
				return nil, errors.New("location resolution commit key does not match payload")
			}
			var source, version, applied uint64
			err := tx.QueryRowContext(ctx, `SELECT source_revision,projection_version,applied_revision FROM location_resolution_pipeline_state WHERE scope='all'`).Scan(&source, &version, &applied)
			if errors.Is(err, sql.ErrNoRows) {
				outcomes[index] = OutcomeStale
				continue
			}
			if err != nil {
				return nil, err
			}
			if source != uint64(payload.Prepared.Revision) || version != payload.ProjectionVersion {
				outcomes[index] = OutcomeStale
				continue
			}
			if applied >= version {
				outcomes[index] = OutcomeDuplicate
				continue
			}
			if err := locationService.ApplyPreparedLocationResolutionTx(ctx, tx, payload.Prepared, payload.ProjectionVersion); err != nil {
				if errors.Is(err, service.ErrLocationProjectionStale) {
					outcomes[index] = OutcomeStale
					continue
				}
				return nil, err
			}
			outcomes[index] = OutcomeApplied
			continue
		}
		if payload, ok := intent.Payload.(OCRProjectionApplied); ok {
			if payload.SourceRevision == 0 || payload.ProjectionVersion == 0 {
				return nil, errors.New("OCR projection commit has no revision")
			}
			if !projectionIntentKeyMatches(intent, "ocr", "all", payload.SourceRevision, payload.ProjectionVersion) {
				return nil, errors.New("OCR projection commit key does not match payload")
			}
			var source, version, applied uint64
			err := tx.QueryRowContext(ctx, `SELECT source_revision,projection_version,applied_revision FROM ocr_projection_pipeline_state WHERE scope='all'`).Scan(&source, &version, &applied)
			if errors.Is(err, sql.ErrNoRows) {
				outcomes[index] = OutcomeStale
				continue
			}
			if err != nil {
				return nil, err
			}
			if source != payload.SourceRevision || version != payload.ProjectionVersion {
				outcomes[index] = OutcomeStale
				continue
			}
			if applied >= version {
				outcomes[index] = OutcomeDuplicate
				continue
			}
			queries := repo.New(tx)
			for _, entry := range payload.Entries {
				if entry.AssetID == uuid.Nil || entry.Revision <= 0 {
					return nil, errors.New("invalid OCR projection entry")
				}
				if _, err := queries.AcknowledgeOCRIndexOutbox(ctx, repo.AcknowledgeOCRIndexOutboxParams{AssetID: entry.AssetID, Revision: entry.Revision}); err != nil {
					return nil, fmt.Errorf("acknowledge OCR index outbox %s@%d: %w", entry.AssetID, entry.Revision, err)
				}
			}
			if payload.Complete {
				if _, err := tx.ExecContext(ctx, `UPDATE ocr_projection_pipeline_state SET applied_revision=projection_version,terminal_error=NULL,updated_at=? WHERE scope='all' AND source_revision=? AND projection_version=? AND applied_revision<projection_version`, now, source, version); err != nil {
					return nil, err
				}
			}
			outcomes[index] = OutcomeApplied
			continue
		}
		if payload, ok := intent.Payload.(ReindexProjectionApplied); ok {
			if indexingService == nil || payload.Prepared.ReceiptID == uuid.Nil || payload.Prepared.RequestedRevision == 0 || payload.ProjectionVersion == 0 {
				return nil, errors.New("reindex projection commit is not configured")
			}
			if !projectionIntentKeyMatches(intent, "asset_reindex", payload.Prepared.ReceiptID.String(), payload.Prepared.RequestedRevision, payload.ProjectionVersion) {
				return nil, errors.New("reindex projection commit key does not match payload")
			}
			var requested, applied uint64
			err := tx.QueryRowContext(ctx, `SELECT requested_revision,applied_revision FROM asset_reindex_requests WHERE receipt_id=?`, payload.Prepared.ReceiptID.String()).Scan(&requested, &applied)
			if errors.Is(err, sql.ErrNoRows) {
				outcomes[index] = OutcomeStale
				continue
			}
			if err != nil {
				return nil, err
			}
			if requested != payload.Prepared.RequestedRevision || requested != payload.ProjectionVersion {
				outcomes[index] = OutcomeStale
				continue
			}
			if applied >= requested {
				outcomes[index] = OutcomeDuplicate
				continue
			}
			if err := indexingService.ApplyPreparedReindexTx(ctx, tx, payload.Prepared); err != nil {
				if errors.Is(err, service.ErrReindexProjectionStale) {
					outcomes[index] = OutcomeStale
					continue
				}
				return nil, err
			}
			outcomes[index] = OutcomeApplied
			continue
		}
		if payload, ok := intent.Payload.(EventProjectionApplied); ok {
			if eventService == nil || payload.Prepared.OwnerID <= 0 || payload.Prepared.SourceRevision <= 0 || payload.ProjectionVersion == 0 {
				return nil, errors.New("event projection commit is not configured")
			}
			if !projectionIntentKeyMatches(intent, "event", strconv.FormatInt(int64(payload.Prepared.OwnerID), 10), uint64(payload.Prepared.SourceRevision), payload.ProjectionVersion) {
				return nil, errors.New("event projection commit key does not match payload")
			}
			ownerID := payload.Prepared.OwnerID
			var source, version, applied uint64
			if err := tx.QueryRowContext(ctx, `SELECT source_revision,projection_version,applied_revision FROM event_projection_pipeline_state WHERE owner_id=?`, ownerID).Scan(&source, &version, &applied); errors.Is(err, sql.ErrNoRows) {
				outcomes[index] = OutcomeStale
				continue
			} else if err != nil {
				return nil, err
			} else if source != uint64(payload.Prepared.SourceRevision) || version != payload.ProjectionVersion {
				outcomes[index] = OutcomeStale
				continue
			} else if applied >= source {
				outcomes[index] = OutcomeDuplicate
				continue
			}
			if err := eventService.ApplyPreparedRebuildTx(ctx, tx, payload.Prepared); err != nil {
				if errors.Is(err, event.ErrStaleRevision) {
					outcomes[index] = OutcomeStale
					continue
				}
				return nil, err
			}
			result, err := tx.ExecContext(ctx, `UPDATE event_projection_pipeline_state SET applied_revision=?,cursor=NULL,terminal_error=NULL,updated_at=? WHERE owner_id=? AND source_revision=? AND projection_version=?`, source, now, ownerID, source, version)
			if err != nil {
				return nil, err
			}
			changed, _ := result.RowsAffected()
			if changed == 0 {
				outcomes[index] = OutcomeStale
			} else {
				outcomes[index] = OutcomeApplied
			}
			continue
		}
		return nil, fmt.Errorf("unsupported projection commit payload %T", intent.Payload)
	}
	return outcomes, nil
}

func projectionIntentKeyMatches(intent Intent, kind, scope string, sourceRevision, projectionVersion uint64) bool {
	return intent.Key.Stage == kind &&
		intent.Key.Subject == scope &&
		intent.Key.Fence == strconv.FormatUint(sourceRevision, 10) &&
		intent.Key.DesiredVersion == projectionVersion
}

func assetIntentKeyMatches(intent Intent, assetID, sourceFence uuid.UUID, stage string, desiredVersion uint64) bool {
	return assetID != uuid.Nil && sourceFence != uuid.Nil && stage != "" && desiredVersion > 0 &&
		intent.Key.Subject == assetID.String() &&
		intent.Key.Fence == sourceFence.String() &&
		intent.Key.Stage == stage &&
		intent.Key.DesiredVersion == desiredVersion
}

func applyOperationReceipts(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	now := time.Now().UTC().UnixMicro()
	for index, intent := range intents {
		payload, ok := intent.Payload.(OperationReceiptApplied)
		if !ok || payload.ReceiptID == uuid.Nil || payload.Kind == "" {
			return nil, errors.New("invalid operation receipt commit payload")
		}
		if intent.Key.Subject != payload.ReceiptID.String() || intent.Key.Fence != payload.ReceiptID.String() || intent.Key.Stage != payload.Kind || intent.Key.DesiredVersion != 1 {
			return nil, errors.New("operation receipt commit key does not match payload")
		}
		var kind string
		err := tx.QueryRowContext(ctx, `SELECT kind FROM catalog_operation_receipts WHERE receipt_id=?`, payload.ReceiptID.String()).Scan(&kind)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if kind != payload.Kind {
			return nil, errors.New("operation receipt commit kind does not match receipt")
		}
		state, applied := "completed", 1
		if payload.TerminalError != "" {
			state, applied = "failed", 0
		}
		result, err := tx.ExecContext(ctx, `UPDATE catalog_operation_receipts SET state=?,applied_version=?,terminal_error=NULLIF(?,''),updated_at=? WHERE receipt_id=? AND kind=? AND state='pending'`, state, applied, payload.TerminalError, now, payload.ReceiptID.String(), payload.Kind)
		if err != nil {
			return nil, err
		}
		changed, _ := result.RowsAffected()
		if changed == 0 {
			outcomes[index] = OutcomeDuplicate
		} else {
			outcomes[index] = OutcomeApplied
		}
	}
	return outcomes, nil
}

func applyAssetStages(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	now := time.Now().UTC().UnixMicro()
	for index, intent := range intents {
		payload, ok := intent.Payload.(AssetStageApplied)
		if !ok || payload.AssetID == uuid.Nil || payload.SourceFence == uuid.Nil || payload.PipelineVersion == "" || payload.DesiredVersion == 0 {
			return nil, errors.New("invalid asset stage commit payload")
		}
		switch payload.Stage {
		case string(pipeline.StageAnalyze), string(pipeline.StageDerivatives), string(pipeline.StageTranscode), string(pipeline.StageEnrich):
		default:
			return nil, fmt.Errorf("invalid asset stage %q", payload.Stage)
		}
		if !assetIntentKeyMatches(intent, payload.AssetID, payload.SourceFence, payload.Stage, payload.DesiredVersion) {
			return nil, errors.New("asset stage commit key does not match payload")
		}
		var fence, pipelineVersion string
		var desired, applied uint64
		err := tx.QueryRowContext(ctx, `SELECT source_content_id,pipeline_version,desired_version,applied_version FROM asset_pipeline_state WHERE asset_id=? AND stage=?`, payload.AssetID.String(), payload.Stage).Scan(&fence, &pipelineVersion, &desired, &applied)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if fence != payload.SourceFence.String() || pipelineVersion != payload.PipelineVersion || desired != payload.DesiredVersion {
			outcomes[index] = OutcomeStale
			continue
		}
		if payload.TerminalError != "" {
			if applied >= desired {
				outcomes[index] = OutcomeDuplicate
				continue
			}
			result, err := tx.ExecContext(ctx, `UPDATE asset_pipeline_state SET terminal_error=?,updated_at=? WHERE asset_id=? AND stage=? AND source_content_id=? AND pipeline_version=? AND desired_version=? AND applied_version<?`, payload.TerminalError, now, payload.AssetID.String(), payload.Stage, payload.SourceFence.String(), payload.PipelineVersion, desired, desired)
			if err != nil {
				return nil, err
			}
			changed, _ := result.RowsAffected()
			if changed == 0 {
				outcomes[index] = OutcomeStale
				continue
			}
			if err := refreshAssetProductStatus(ctx, tx, payload.AssetID, now); err != nil {
				return nil, err
			}
			if err := settleAssetPipelineReceipts(ctx, tx, payload.AssetID, now); err != nil {
				return nil, err
			}
			outcomes[index] = OutcomeApplied
			continue
		}
		if applied >= desired {
			outcomes[index] = OutcomeDuplicate
			if err := settleAssetPipelineReceipts(ctx, tx, payload.AssetID, now); err != nil {
				return nil, err
			}
			continue
		}
		result, err := tx.ExecContext(ctx, `UPDATE asset_pipeline_state SET applied_version=?,terminal_error=NULL,updated_at=? WHERE asset_id=? AND stage=? AND source_content_id=? AND pipeline_version=? AND desired_version=? AND applied_version<?`, desired, now, payload.AssetID.String(), payload.Stage, payload.SourceFence.String(), payload.PipelineVersion, desired, desired)
		if err != nil {
			return nil, err
		}
		changed, _ := result.RowsAffected()
		if changed == 0 {
			outcomes[index] = OutcomeStale
		} else {
			outcomes[index] = OutcomeApplied
		}
		if outcomes[index] != OutcomeStale {
			if err := pipeline.PublishReadyAssetStagesTx(ctx, tx, payload.AssetID, pipeline.AdmissionBackground); err != nil {
				return nil, err
			}
			if err := refreshAssetProductStatus(ctx, tx, payload.AssetID, now); err != nil {
				return nil, err
			}
			if err := settleAssetPipelineReceipts(ctx, tx, payload.AssetID, now); err != nil {
				return nil, err
			}
		}
	}
	return outcomes, nil
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

func applyIngestReceipts(ctx context.Context, tx *sql.Tx, intents []Intent) ([]Outcome, error) {
	outcomes := make([]Outcome, len(intents))
	now := time.Now().UTC().UnixMicro()
	for index, intent := range intents {
		payload, ok := intent.Payload.(IngestReceiptApplied)
		if !ok {
			return nil, errors.New("invalid ingest receipt commit payload")
		}
		if payload.ReceiptID == uuid.Nil || intent.Key.Subject != payload.ReceiptID.String() || intent.Key.Fence == "" || intent.Key.Stage != "ingest" || intent.Key.DesiredVersion != 1 {
			return nil, errors.New("ingest receipt commit key does not match payload")
		}
		if commitID, err := uuid.Parse(intent.Key.Fence); err != nil || commitID == uuid.Nil {
			return nil, errors.New("ingest receipt commit key has an invalid commit fence")
		}
		var subject string
		err := tx.QueryRowContext(ctx, `SELECT subject_id FROM catalog_operation_receipts WHERE receipt_id=? AND kind='ingest'`, payload.ReceiptID.String()).Scan(&subject)
		if errors.Is(err, sql.ErrNoRows) {
			outcomes[index] = OutcomeStale
			continue
		}
		if err != nil {
			return nil, err
		}
		if subject != intent.Key.Fence {
			return nil, errors.New("ingest receipt commit fence does not match receipt subject")
		}
		state := "completed"
		applied := 1
		if payload.TerminalError != "" {
			state = "failed"
			applied = 0
		}
		result, err := tx.ExecContext(ctx, `UPDATE catalog_operation_receipts SET state=?,applied_version=?,terminal_error=NULLIF(?,''),updated_at=? WHERE receipt_id=? AND kind='ingest' AND state='pending'`, state, applied, payload.TerminalError, now, payload.ReceiptID.String())
		if err != nil {
			return nil, err
		}
		changed, _ := result.RowsAffected()
		if changed == 0 {
			outcomes[index] = OutcomeDuplicate
		} else {
			outcomes[index] = OutcomeApplied
		}
	}
	return outcomes, nil
}
