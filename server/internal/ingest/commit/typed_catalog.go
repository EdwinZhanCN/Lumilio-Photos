package commit

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/google/uuid"
)

func (c *Coordinator) submitOutcome(ctx context.Context, kind OperationKind, apply func(context.Context, *sql.Tx) (Outcome, error)) (Result, error) {
	result, err := c.SubmitOperation(ctx, Operation{
		Kind: kind,
		Apply: func(ctx context.Context, tx *sql.Tx) (Result, error) {
			outcome, err := apply(ctx, tx)
			return Result{Outcome: outcome}, err
		},
	})
	if err != nil {
		return result, &unacknowledgedError{cause: err}
	}
	return result, nil
}

// IsUnacknowledged distinguishes a failed Catalog publication from a domain
// execution failure. Delivery must retry publication without spending a media
// processing failure budget or reporting success.
func IsUnacknowledged(err error) bool {
	var failure *unacknowledgedError
	return errors.As(err, &failure)
}

type unacknowledgedError struct{ cause error }

func (e *unacknowledgedError) Error() string { return e.cause.Error() }
func (e *unacknowledgedError) Unwrap() error { return e.cause }

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

func formatOwner(ownerID int32) string {
	return strconv.FormatInt(int64(ownerID), 10)
}

func uint64String(value uint64) string {
	return strconv.FormatUint(value, 10)
}
