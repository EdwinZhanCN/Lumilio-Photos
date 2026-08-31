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

// IndexOutboxEntry is the immutable catalog fence for one external OCR index
// update. It is intentionally separate from the generated query row so the
// commit coordinator can carry only stable IDs and revisions.
type IndexOutboxEntry struct {
	AssetID  uuid.UUID
	Revision int64
}

// PreparedBatch contains the read-only OCR documents and their corresponding
// outbox fences. ApplyPreparedBatch updates Bleve outside a catalog
// transaction; the coordinator subsequently acknowledges Entries atomically.
type PreparedBatch struct {
	Entries   []IndexOutboxEntry
	Mutations []Mutation
	More      bool
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
	prepared, err := w.PrepareBatch(ctx, batchSize)
	if err != nil {
		return 0, err
	}
	if len(prepared.Entries) == 0 {
		return 0, nil
	}
	if err := w.ApplyPreparedBatch(prepared); err != nil {
		return 0, err
	}

	tx, err := w.writer.BeginTx(ctx, catalogtx.OperationOCRIndexBatch, nil)
	if err != nil {
		return 0, fmt.Errorf("begin OCR outbox acknowledgement: %w", err)
	}
	defer tx.Rollback()
	txQueries := w.queries.WithTx(tx.Raw())
	for _, event := range prepared.Entries {
		if _, err := txQueries.AcknowledgeOCRIndexOutbox(ctx, repo.AcknowledgeOCRIndexOutboxParams{
			AssetID: event.AssetID, Revision: event.Revision,
		}); err != nil {
			return 0, fmt.Errorf("acknowledge OCR outbox %s@%d: %w", event.AssetID, event.Revision, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit OCR outbox acknowledgement: %w", err)
	}
	return len(prepared.Entries), nil
}

// PrepareBatch reads one bounded OCR outbox page and builds deterministic
// Bleve mutations without touching either catalog or external index state.
func (w *Writer) PrepareBatch(ctx context.Context, batchSize int) (PreparedBatch, error) {
	if w == nil || w.pool == nil || w.queries == nil || w.index == nil {
		return PreparedBatch{}, fmt.Errorf("OCR Bleve writer is not configured")
	}
	if batchSize <= 0 {
		batchSize = DefaultOutboxBatchSize
	}
	events, err := w.queries.ListOCRIndexOutboxBatch(ctx, int64(batchSize+1))
	if err != nil {
		return PreparedBatch{}, fmt.Errorf("list OCR index outbox: %w", err)
	}
	if len(events) == 0 {
		return PreparedBatch{}, nil
	}
	more := len(events) > batchSize
	if more {
		events = events[:batchSize]
	}

	ids := make([]uuid.UUID, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.AssetID)
	}
	idsJSON := dbtypes.UUIDsJSONParam(ids)
	if idsJSON == nil {
		return PreparedBatch{}, nil
	}
	rows, err := w.queries.GetOCRDocumentsByAssetIDs(ctx, *idsJSON)
	if err != nil {
		return PreparedBatch{}, fmt.Errorf("read pending OCR documents: %w", err)
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
	entries := make([]IndexOutboxEntry, 0, len(events))
	for _, event := range events {
		entries = append(entries, IndexOutboxEntry{AssetID: event.AssetID, Revision: event.Revision})
	}
	return PreparedBatch{Entries: entries, Mutations: mutations, More: more}, nil
}

// ApplyPreparedBatch publishes the immutable external index mutations. A
// caller may safely retry this method; catalog acknowledgement is the durable
// idempotency fence.
func (w *Writer) ApplyPreparedBatch(prepared PreparedBatch) error {
	if w == nil || w.index == nil {
		return fmt.Errorf("OCR Bleve writer is not configured")
	}
	if err := w.index.Apply(prepared.Mutations); err != nil {
		return err
	}
	if w.afterApply != nil {
		if err := w.afterApply(); err != nil {
			return err
		}
	}
	return nil
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
