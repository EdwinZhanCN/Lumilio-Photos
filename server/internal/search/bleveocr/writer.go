package bleveocr

import (
	"context"
	"database/sql"
	"fmt"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

const (
	DefaultOutboxBatchSize = 128
	DefaultMaxDrainBatches = 16
)

type Writer struct {
	pool       *sql.DB
	writer     *catalogtx.Writer
	queries    *repo.Queries
	index      *Index
	afterApply func() error
}

func NewWriter(pool *sql.DB, writer *catalogtx.Writer, queries *repo.Queries, index *Index) *Writer {
	return &Writer{pool: pool, writer: writer, queries: queries, index: index}
}

func (w *Writer) Drain(ctx context.Context, batchSize, maxBatches int) (int, error) {
	if batchSize <= 0 {
		batchSize = DefaultOutboxBatchSize
	}
	if maxBatches <= 0 {
		maxBatches = DefaultMaxDrainBatches
	}
	total := 0
	for range maxBatches {
		processed, err := w.ProcessBatch(ctx, batchSize)
		if err != nil {
			return total, err
		}
		total += processed
		if processed < batchSize {
			break
		}
	}
	return total, nil
}

func (w *Writer) ProcessBatch(ctx context.Context, batchSize int) (int, error) {
	if w == nil || w.pool == nil || w.queries == nil || w.index == nil {
		return 0, fmt.Errorf("OCR Bleve writer is not configured")
	}
	if batchSize <= 0 {
		batchSize = DefaultOutboxBatchSize
	}
	events, err := w.queries.ListOCRIndexOutboxBatch(ctx, int64(batchSize))
	if err != nil {
		return 0, fmt.Errorf("list OCR index outbox: %w", err)
	}
	if len(events) == 0 {
		return 0, nil
	}

	ids := make([]uuid.UUID, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.AssetID)
	}
	idsJSON := dbtypes.UUIDsJSONParam(ids)
	if idsJSON == nil {
		return 0, nil
	}
	rows, err := w.queries.GetOCRDocumentsByAssetIDs(ctx, *idsJSON)
	if err != nil {
		return 0, fmt.Errorf("read pending OCR documents: %w", err)
	}
	documents := groupPendingDocuments(rows)

	mutations := make([]Mutation, 0, len(events))
	for _, event := range events {
		assetID := event.AssetID.String()
		source, ok := documents[assetID]
		if !ok {
			mutations = append(mutations, Mutation{AssetID: assetID})
			continue
		}
		document := BuildDocument(source)
		mutations = append(mutations, Mutation{AssetID: assetID, Document: &document})
	}
	if err := w.index.Apply(mutations); err != nil {
		return 0, err
	}
	if w.afterApply != nil {
		if err := w.afterApply(); err != nil {
			return 0, err
		}
	}

	tx, err := w.writer.BeginTx(ctx, catalogtx.OperationOCRIndexBatch, nil)
	if err != nil {
		return 0, fmt.Errorf("begin OCR outbox acknowledgement: %w", err)
	}
	defer tx.Rollback()
	txQueries := w.queries.WithTx(tx.Raw())
	for _, event := range events {
		if _, err := txQueries.AcknowledgeOCRIndexOutbox(ctx, repo.AcknowledgeOCRIndexOutboxParams{
			AssetID:  event.AssetID,
			Revision: event.Revision,
		}); err != nil {
			return 0, fmt.Errorf("acknowledge OCR outbox %s@%d: %w", event.AssetID, event.Revision, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit OCR outbox acknowledgement: %w", err)
	}
	return len(events), nil
}

func groupPendingDocuments(rows []repo.GetOCRDocumentsByAssetIDsRow) map[string]SourceDocument {
	documents := make(map[string]SourceDocument)
	for _, row := range rows {
		assetID := row.AssetID.String()
		source, ok := documents[assetID]
		if !ok {
			source = SourceDocument{
				AssetID:   assetID,
				AssetType: row.AssetType,
				IsDeleted: row.IsDeleted,
				Revision:  row.Revision,
			}
			if row.OwnerID != nil {
				source.OwnerID = *row.OwnerID
			}
			if repositoryID := repositoryIDString(row.RepositoryID); repositoryID != "" {
				source.RepositoryID = repositoryID
			}
		}
		if row.TextContent != nil {
			source.TextItems = append(source.TextItems, *row.TextContent)
		}
		documents[assetID] = source
	}
	return documents
}
