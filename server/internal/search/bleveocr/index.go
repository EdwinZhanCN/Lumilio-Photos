package bleveocr

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/blevesearch/bleve/v2"
)

const indexDirectoryName = "ocr-v1"

var ErrClosed = errors.New("OCR Bleve index is closed")

type Hit struct {
	AssetID string
	Score   float64
}

type SearchPage struct {
	Hits  []Hit
	Total uint64
}

type Mutation struct {
	AssetID  string
	Document *OCRDocument
}

type Index struct {
	path  string
	mu    sync.RWMutex
	bleve bleve.Index
}

func PathForDatabase(databasePath string) string {
	return filepath.Join(filepath.Dir(databasePath), "indexes", "bleve", indexDirectoryName)
}

func (i *Index) Path() string {
	if i == nil {
		return ""
	}
	return i.path
}

func (i *Index) SearchPage(
	ctx context.Context,
	rawQuery string,
	filters BasicFilters,
	mode QueryMode,
	from int,
	size int,
) (SearchPage, error) {
	if err := ctx.Err(); err != nil {
		return SearchPage{}, err
	}
	if from < 0 {
		from = 0
	}
	if size <= 0 {
		return SearchPage{Hits: []Hit{}}, nil
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.bleve == nil {
		return SearchPage{}, ErrClosed
	}
	searchQuery, searchable := buildQuery(i.bleve.Mapping(), rawQuery, filters, mode)
	if !searchable {
		return SearchPage{Hits: []Hit{}}, nil
	}
	request := bleve.NewSearchRequestOptions(searchQuery, size, from, false)
	result, err := i.bleve.SearchInContext(ctx, request)
	if err != nil {
		return SearchPage{}, fmt.Errorf("search OCR Bleve index: %w", err)
	}

	page := SearchPage{
		Hits:  make([]Hit, 0, len(result.Hits)),
		Total: result.Total,
	}
	for _, hit := range result.Hits {
		page.Hits = append(page.Hits, Hit{AssetID: hit.ID, Score: hit.Score})
	}
	return page, nil
}

func (i *Index) Apply(mutations []Mutation) error {
	if len(mutations) == 0 {
		return nil
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.bleve == nil {
		return ErrClosed
	}

	batch := i.bleve.NewBatch()
	for _, mutation := range mutations {
		if mutation.AssetID == "" {
			continue
		}
		if mutation.Document == nil || !mutation.Document.HasSearchableText() {
			batch.Delete(mutation.AssetID)
			continue
		}
		if err := batch.Index(mutation.AssetID, mutation.Document); err != nil {
			return fmt.Errorf("map OCR document %s: %w", mutation.AssetID, err)
		}
	}
	if batch.Size() == 0 {
		return nil
	}
	if err := i.bleve.Batch(batch); err != nil {
		return fmt.Errorf("apply OCR Bleve batch: %w", err)
	}
	return nil
}

func (i *Index) Close() error {
	if i == nil {
		return nil
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.bleve == nil {
		return nil
	}
	err := i.bleve.Close()
	i.bleve = nil
	if err != nil {
		return fmt.Errorf("close OCR Bleve index: %w", err)
	}
	return nil
}
