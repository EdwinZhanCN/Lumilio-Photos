package bleveocr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"server/internal/db/repo"

	"github.com/blevesearch/bleve/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const rebuildBatchSize = 256

func Open(
	ctx context.Context,
	databasePath string,
	queries *repo.Queries,
	forceRebuild bool,
	logger *zap.Logger,
) (*Index, error) {
	if queries == nil {
		return nil, fmt.Errorf("open OCR Bleve index: queries are nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	path := PathForDatabase(databasePath)
	if !forceRebuild {
		if index, err := openHealthy(path); err == nil {
			return index, nil
		} else {
			logger.Warn("OCR Bleve index requires rebuild", zap.String("path", path), zap.Error(err))
		}
	} else {
		logger.Info("forcing OCR Bleve rebuild", zap.String("path", path))
	}

	return rebuild(ctx, path, queries)
}

func openHealthy(path string) (*Index, error) {
	raw, err := bleve.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	closeOnError := func(cause error) (*Index, error) {
		return nil, errorsJoin(cause, raw.Close())
	}

	version, err := raw.GetInternal(mappingVersionKey)
	if err != nil {
		return closeOnError(fmt.Errorf("read mapping version: %w", err))
	}
	if string(version) != MappingVersion {
		return closeOnError(fmt.Errorf("mapping version = %q, want %q", string(version), MappingVersion))
	}
	if _, err := raw.DocCount(); err != nil {
		return closeOnError(fmt.Errorf("read document count: %w", err))
	}
	if _, err := raw.Fields(); err != nil {
		return closeOnError(fmt.Errorf("read fields: %w", err))
	}
	return &Index{path: path, bleve: raw}, nil
}

func rebuild(ctx context.Context, path string, queries *repo.Queries) (*Index, error) {
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, fmt.Errorf("create OCR Bleve parent: %w", err)
	}
	if err := os.Chmod(parent, 0o700); err != nil {
		return nil, fmt.Errorf("secure OCR Bleve parent: %w", err)
	}
	tempRoot, err := os.MkdirTemp(parent, ".ocr-v1-rebuild-")
	if err != nil {
		return nil, fmt.Errorf("create OCR Bleve rebuild directory: %w", err)
	}
	defer os.RemoveAll(tempRoot)
	tempPath := filepath.Join(tempRoot, indexDirectoryName)

	indexMapping, err := NewMapping()
	if err != nil {
		return nil, err
	}
	raw, err := bleve.New(tempPath, indexMapping)
	if err != nil {
		return nil, fmt.Errorf("create OCR Bleve rebuild index: %w", err)
	}
	closeRaw := true
	defer func() {
		if closeRaw {
			_ = raw.Close()
		}
	}()

	after := uuid.Nil
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, err := queries.GetOCRDocumentsForRebuild(ctx, repo.GetOCRDocumentsForRebuildParams{
			AfterAssetID: after,
			BatchSize:    rebuildBatchSize,
		})
		if err != nil {
			return nil, fmt.Errorf("read OCR rebuild batch: %w", err)
		}
		if len(rows) == 0 {
			break
		}

		sources := make([]SourceDocument, 0)
		for _, row := range rows {
			source := sourceFromRebuildRow(row)
			if len(sources) == 0 || sources[len(sources)-1].AssetID != source.AssetID {
				sources = append(sources, source)
			} else if row.TextContent != nil {
				sources[len(sources)-1].TextItems = append(sources[len(sources)-1].TextItems, *row.TextContent)
			}
			after = row.AssetID
		}

		batch := raw.NewBatch()
		for _, source := range sources {
			document := BuildDocument(source)
			if !document.HasSearchableText() {
				continue
			}
			if err := batch.Index(document.AssetID, document); err != nil {
				return nil, fmt.Errorf("map OCR rebuild document %s: %w", document.AssetID, err)
			}
		}
		if batch.Size() > 0 {
			if err := raw.Batch(batch); err != nil {
				return nil, fmt.Errorf("write OCR rebuild batch: %w", err)
			}
		}
	}
	if err := raw.SetInternal(mappingVersionKey, []byte(MappingVersion)); err != nil {
		return nil, fmt.Errorf("write OCR mapping version: %w", err)
	}
	if err := raw.Close(); err != nil {
		return nil, fmt.Errorf("close rebuilt OCR Bleve index: %w", err)
	}
	closeRaw = false

	if err := removeIndexPath(path); err != nil {
		return nil, err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return nil, fmt.Errorf("install rebuilt OCR Bleve index: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return nil, fmt.Errorf("secure rebuilt OCR Bleve index: %w", err)
	}

	index, err := openHealthy(path)
	if err != nil {
		return nil, fmt.Errorf("verify rebuilt OCR Bleve index: %w", err)
	}
	if err := queries.ClearOCRIndexOutbox(ctx); err != nil {
		_ = index.Close()
		return nil, fmt.Errorf("clear rebuilt OCR outbox: %w", err)
	}
	return index, nil
}

func sourceFromRebuildRow(row repo.GetOCRDocumentsForRebuildRow) SourceDocument {
	source := SourceDocument{
		AssetID:   row.AssetID.String(),
		AssetType: row.AssetType,
		IsDeleted: row.IsDeleted,
		Revision:  row.Revision,
	}
	if row.OwnerID != nil {
		source.OwnerID = *row.OwnerID
	}
	if row.RepositoryID.Valid {
		source.RepositoryID = row.RepositoryID.UUID.String()
	}
	if row.TextContent != nil {
		source.TextItems = []string{*row.TextContent}
	}
	return source
}

func removeIndexPath(path string) error {
	if filepath.Base(path) != indexDirectoryName || filepath.Base(filepath.Dir(path)) != "bleve" {
		return fmt.Errorf("refuse to remove unexpected OCR index path %q", path)
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove old OCR Bleve index: %w", err)
	}
	return nil
}

func errorsJoin(primary, closeErr error) error {
	if closeErr == nil {
		return primary
	}
	return fmt.Errorf("%v; close index: %w", primary, closeErr)
}
