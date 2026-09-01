package commit

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"server/internal/db/repo"
	"server/internal/service"
	roematerializer "server/internal/storage/roe/materializer"
)

func (c *Coordinator) submitOutcome(ctx context.Context, kind OperationKind, apply func(context.Context, *sql.Tx) (Outcome, error)) (Result, error) {
	return c.SubmitOperation(ctx, Operation{
		Kind: kind,
		Apply: func(ctx context.Context, tx *sql.Tx) (Result, error) {
			outcome, err := apply(ctx, tx)
			return Result{Outcome: outcome}, err
		},
	})
}

func (c *Coordinator) ApplyAssetStage(ctx context.Context, payload AssetStageApplied) (Result, error) {
	if err := validateAssetStage(payload); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogAssetStage, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyAssetStages(ctx, tx, payload)
	})
}

func (c *Coordinator) ApplyAssetMetadata(ctx context.Context, payload AssetMetadataApplied) (Result, error) {
	if err := validateAssetMetadata(payload); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogAssetMetadata, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyAssetMetadata(ctx, tx, payload)
	})
}

func (c *Coordinator) ApplyAssetDerivatives(ctx context.Context, payload AssetDerivativesApplied) (Result, error) {
	if err := validateAssetDerivatives(payload); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogAssetDerivatives, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyAssetDerivatives(ctx, tx, payload)
	})
}

func (c *Coordinator) ApplyAssetStack(ctx context.Context, payload AssetStackApplied) (Result, error) {
	if err := validateAssetStack(payload); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogAssetStack, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyAssetStack(ctx, tx, payload)
	})
}

func (c *Coordinator) ApplyRepositoryAsset(ctx context.Context, payload RepositoryAssetApplied) (Result, error) {
	if err := validateRepositoryAsset(payload); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogRepositoryAsset, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyRepositoryAssets(ctx, tx, payload)
	})
}

func (c *Coordinator) ApplyRepositoryKnownContent(ctx context.Context, payload RepositoryKnownContentApplied) (Result, error) {
	if err := validateRepositoryKnownContent(payload); err != nil {
		return Result{}, err
	}
	if c.catalog.Materializer == nil {
		return Result{}, errors.New("repository known-content committer is not configured")
	}
	return c.submitOutcome(ctx, OperationKindCatalogRepositoryKnownContent, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyRepositoryKnownContentResult(ctx, tx, payload, c.catalog.Materializer)
	})
}

func (c *Coordinator) ApplyRepositoryHash(ctx context.Context, payload RepositoryHashApplied) (Result, error) {
	if err := validateRepositoryHash(payload); err != nil {
		return Result{}, err
	}
	if c.catalog.Materializer == nil {
		return Result{}, errors.New("repository hash committer is not configured")
	}
	return c.submitOutcome(ctx, OperationKindCatalogRepositoryHash, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyRepositoryHashResult(ctx, tx, payload, c.catalog.Materializer)
	})
}

func (c *Coordinator) ApplyVideoFrameEmbeddings(ctx context.Context, payload VideoFrameEmbeddingsApplied) (Result, error) {
	if err := validateVideoFrameEmbeddings(payload); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogVideoFrameEmbeddings, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyVideoFrameEmbeddings(ctx, tx, payload)
	})
}

func (c *Coordinator) ApplyEnrichment(ctx context.Context, payload EnrichmentApplied) (Result, error) {
	if err := validateEnrichment(payload); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogEnrichment, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyEnrichment(ctx, tx, payload, c.catalog.Face)
	})
}

func (c *Coordinator) ApplyIngestReceipt(ctx context.Context, payload IngestReceiptApplied, commitID uuid.UUID) (Result, error) {
	if err := validateIngestReceipt(payload, commitID); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogIngestReceipt, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyIngestReceipts(ctx, tx, payload, commitID)
	})
}

func (c *Coordinator) ApplyOperationReceipt(ctx context.Context, payload OperationReceiptApplied) (Result, error) {
	if err := validateOperationReceipt(payload); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogOperationReceipt, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyOperationReceipts(ctx, tx, payload)
	})
}

func (c *Coordinator) ApplyRepositoryEpoch(ctx context.Context, payload RepositoryEpochApplied) (Result, error) {
	if err := validateRepositoryEpoch(payload); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogRepositoryEpoch, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyRepositoryEpochs(ctx, tx, payload)
	})
}

func (c *Coordinator) ApplyProjectionTerminalFailure(ctx context.Context, payload ProjectionTerminalFailure) (Result, error) {
	if err := validateProjectionTerminalFailure(payload); err != nil {
		return Result{}, err
	}
	return c.submitOutcome(ctx, OperationKindCatalogProjection, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyProjectionTerminalFailures(ctx, tx, payload)
	})
}

func (c *Coordinator) ApplyEventProjection(ctx context.Context, payload EventProjectionApplied) (Result, error) {
	if err := validateEventProjection(payload); err != nil {
		return Result{}, err
	}
	return c.submitProjection(ctx, projectionCommit{Event: &payload})
}

func (c *Coordinator) ApplyLocationProjection(ctx context.Context, payload LocationProjectionApplied) (Result, error) {
	if err := validateLocationProjection(payload); err != nil {
		return Result{}, err
	}
	return c.submitProjection(ctx, projectionCommit{Location: &payload})
}

func (c *Coordinator) ApplyLocationResolution(ctx context.Context, payload LocationResolutionApplied) (Result, error) {
	if err := validateLocationResolution(payload); err != nil {
		return Result{}, err
	}
	return c.submitProjection(ctx, projectionCommit{LocationResolution: &payload})
}

func (c *Coordinator) ApplyOCRProjection(ctx context.Context, payload OCRProjectionApplied) (Result, error) {
	if err := validateOCRProjection(payload); err != nil {
		return Result{}, err
	}
	return c.submitProjection(ctx, projectionCommit{OCR: &payload})
}

func (c *Coordinator) ApplyReindexProjection(ctx context.Context, payload ReindexProjectionApplied) (Result, error) {
	if err := validateReindexProjection(payload); err != nil {
		return Result{}, err
	}
	return c.submitProjection(ctx, projectionCommit{Reindex: &payload})
}

func (c *Coordinator) submitProjection(ctx context.Context, payload projectionCommit) (Result, error) {
	return c.submitOutcome(ctx, OperationKindCatalogProjection, func(ctx context.Context, tx *sql.Tx) (Outcome, error) {
		return applyProjections(ctx, tx, payload, c.catalog.Event, c.catalog.Location, c.catalog.Indexing)
	})
}

func validateAssetIdentity(assetID, sourceFence uuid.UUID, pipelineVersion string, desiredVersion uint64, kind string) error {
	if assetID == uuid.Nil || sourceFence == uuid.Nil || pipelineVersion == "" || desiredVersion == 0 {
		return errors.New("invalid " + kind + " result")
	}
	return nil
}

func validateAssetStage(payload AssetStageApplied) error {
	if err := validateAssetIdentity(payload.AssetID, payload.SourceFence, payload.PipelineVersion, payload.DesiredVersion, "asset stage"); err != nil {
		return err
	}
	switch payload.Stage {
	case "analyze", "derivatives", "transcode", "enrich":
		return nil
	default:
		return errors.New("invalid asset stage " + strconv.Quote(payload.Stage))
	}
}

func validateAssetMetadata(payload AssetMetadataApplied) error {
	return validateAssetIdentity(payload.AssetID, payload.SourceFence, payload.PipelineVersion, payload.DesiredVersion, "asset metadata")
}

func validateAssetDerivatives(payload AssetDerivativesApplied) error {
	return validateAssetIdentity(payload.AssetID, payload.SourceFence, payload.PipelineVersion, payload.DesiredVersion, "asset derivatives")
}

func validateAssetStack(payload AssetStackApplied) error {
	return validateAssetIdentity(payload.AssetID, payload.SourceFence, payload.PipelineVersion, payload.DesiredVersion, "asset stack")
}

func validateRepositoryAsset(payload RepositoryAssetApplied) error {
	if payload.RepositoryID == uuid.Nil || payload.NodeID == uuid.Nil || payload.AssetID == uuid.Nil || payload.ContentID == uuid.Nil || payload.ObservationRevision <= 0 {
		return errors.New("invalid repository asset result")
	}
	return nil
}

func validateRepositoryKnownContent(payload RepositoryKnownContentApplied) error {
	fact := payload.Fact
	if fact.RepositoryID == uuid.Nil || fact.OwnerID <= 0 || fact.SourceEventKey == "" || fact.Observation.ObservationToken == "" {
		return errors.New("invalid repository known-content result")
	}
	return nil
}

func validateRepositoryHash(payload RepositoryHashApplied) error {
	prepared := payload.Prepared
	if prepared.Node.NodeID == uuid.Nil || prepared.Node.RepositoryID == uuid.Nil || prepared.Node.ObservationRevision <= 0 || prepared.Observation.ObservationToken == "" {
		return errors.New("invalid repository hash result")
	}
	return nil
}

func validateVideoFrameEmbeddings(payload VideoFrameEmbeddingsApplied) error {
	if err := validateAssetIdentity(payload.AssetID, payload.SourceFence, payload.PipelineVersion, payload.DesiredVersion, "video frame embedding"); err != nil {
		return err
	}
	if payload.ModelID == "" || len(payload.Frames) == 0 {
		return errors.New("invalid video frame embedding result")
	}
	return nil
}

func validateEnrichment(payload EnrichmentApplied) error {
	return validateAssetIdentity(payload.AssetID, payload.SourceFence, payload.PipelineVersion, payload.DesiredVersion, "enrichment")
}

func validateIngestReceipt(payload IngestReceiptApplied, commitID uuid.UUID) error {
	if payload.ReceiptID == uuid.Nil || commitID == uuid.Nil {
		return errors.New("invalid ingest receipt result")
	}
	return nil
}

func validateOperationReceipt(payload OperationReceiptApplied) error {
	if payload.ReceiptID == uuid.Nil || payload.Kind == "" {
		return errors.New("invalid operation receipt result")
	}
	return nil
}

func validateRepositoryEpoch(payload RepositoryEpochApplied) error {
	if payload.RepositoryID == uuid.Nil || payload.RequestedEpoch == 0 {
		return errors.New("invalid repository epoch result")
	}
	return nil
}

func validateProjectionTerminalFailure(payload ProjectionTerminalFailure) error {
	if payload.Kind == "" || payload.Scope == "" || payload.SourceRevision == 0 || payload.ProjectionVersion == 0 || payload.TerminalError == "" {
		return errors.New("invalid projection terminal failure result")
	}
	return nil
}

func validateEventProjection(payload EventProjectionApplied) error {
	if payload.Prepared.OwnerID <= 0 || payload.Prepared.SourceRevision <= 0 || payload.ProjectionVersion == 0 {
		return errors.New("event projection commit is not configured")
	}
	return nil
}

func validateLocationProjection(payload LocationProjectionApplied) error {
	if payload.Prepared.RepositoryID == uuid.Nil || payload.Prepared.OwnerID <= 0 || payload.Prepared.SourceRevision <= 0 || payload.ProjectionVersion == 0 {
		return errors.New("location projection commit is not configured")
	}
	return nil
}

func validateLocationResolution(payload LocationResolutionApplied) error {
	if payload.Prepared.Revision <= 0 || payload.ProjectionVersion == 0 {
		return errors.New("location resolution commit is not configured")
	}
	return nil
}

func validateOCRProjection(payload OCRProjectionApplied) error {
	if payload.SourceRevision == 0 || payload.ProjectionVersion == 0 {
		return errors.New("OCR projection commit has no revision")
	}
	for _, entry := range payload.Entries {
		if entry.AssetID == uuid.Nil || entry.Revision <= 0 {
			return errors.New("invalid OCR projection entry")
		}
	}
	return nil
}

func validateReindexProjection(payload ReindexProjectionApplied) error {
	if payload.Prepared.ReceiptID == uuid.Nil || payload.Prepared.RequestedRevision == 0 || payload.ProjectionVersion == 0 {
		return errors.New("reindex projection commit is not configured")
	}
	return nil
}

func applyRepositoryHashResult(ctx context.Context, tx *sql.Tx, payload RepositoryHashApplied, materializer *roematerializer.HashApplier) (Outcome, error) {
	prepared := payload.Prepared
	if prepared.Node.NodeID == uuid.Nil || prepared.Node.RepositoryID == uuid.Nil || prepared.Node.ObservationRevision <= 0 || prepared.Observation.ObservationToken == "" {
		return 0, errors.New("invalid repository hash result")
	}
	result, err := materializer.ApplyHash(ctx, tx, prepared)
	if err != nil {
		return 0, err
	}
	if result.Code == roematerializer.ResultStale || result.AssetID == uuid.Nil {
		return OutcomeStale, nil
	}
	if err := serviceApplyAssetActivation(ctx, tx, result.RepositoryID, result.NodeID, result.AssetID, result.ContentID); err != nil {
		return 0, err
	}
	if result.Code == roematerializer.ResultNoop {
		return OutcomeDuplicate, nil
	}
	return OutcomeApplied, nil
}

func applyRepositoryKnownContentResult(ctx context.Context, tx *sql.Tx, payload RepositoryKnownContentApplied, materializer *roematerializer.HashApplier) (Outcome, error) {
	fact := payload.Fact
	if fact.RepositoryID == uuid.Nil || fact.SourceEventKey == "" {
		return 0, errors.New("invalid repository known-content result")
	}
	result, err := materializer.ApplyKnownContent(ctx, tx, fact)
	if err != nil {
		return 0, err
	}
	if result.Code == roematerializer.ResultStale || result.AssetID == uuid.Nil {
		return OutcomeStale, nil
	}
	if err := serviceApplyAssetActivation(ctx, tx, result.RepositoryID, result.NodeID, result.AssetID, result.ContentID); err != nil {
		return 0, err
	}
	if result.Code == roematerializer.ResultNoop {
		return OutcomeDuplicate, nil
	}
	return OutcomeApplied, nil
}

func serviceApplyAssetActivation(ctx context.Context, tx *sql.Tx, repositoryID, nodeID, assetID, contentID uuid.UUID) error {
	return service.ApplyAssetActivationTx(ctx, tx, repo.New(tx), repositoryID, nodeID, assetID, contentID)
}

func formatOwner(ownerID int32) string {
	return strconv.FormatInt(int64(ownerID), 10)
}

func uint64String(value uint64) string {
	return strconv.FormatUint(value, 10)
}
