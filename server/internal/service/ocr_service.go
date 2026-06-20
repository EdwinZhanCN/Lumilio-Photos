package service

import (
	"context"
	"fmt"
	"strings"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/search"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OCRService defines OCR related operations interface
type OCRService interface {
	SaveOCRResults(ctx context.Context, assetID pgtype.UUID, ocrResult *types.OCRV1, processingTimeMs int) error
	GetOCRResults(ctx context.Context, assetID pgtype.UUID) (*OCRResultWithItems, error)
	SearchAssetsByText(ctx context.Context, searchText string, limit, offset int, minConfidence float32) ([]repo.Asset, error)
	DeleteOCRResults(ctx context.Context, assetID pgtype.UUID) error
	GetOCRStats(ctx context.Context) (*dbtypes.OCRStats, error)
}

// OCRResultWithItems contains OCR results and detailed text items
type OCRResultWithItems struct {
	Result *repo.OcrResult
	Items  []repo.OcrTextItem
}

type ocrService struct {
	queries *repo.Queries
	pool    *pgxpool.Pool
}

// NewOCRService creates OCR service instance
func NewOCRService(queries *repo.Queries, pool *pgxpool.Pool) OCRService {
	return &ocrService{
		queries: queries,
		pool:    pool,
	}
}

// SaveOCRResults saves OCR results to database
func (s *ocrService) SaveOCRResults(ctx context.Context, assetID pgtype.UUID, ocrResult *types.OCRV1, processingTimeMs int) error {
	if err := s.queries.DeleteOCRResultByAsset(ctx, assetID); err != nil {
		return fmt.Errorf("failed to delete existing OCR results: %w", err)
	}

	processingTimePtr := int32(processingTimeMs)
	_, err := s.queries.CreateOCRResult(ctx, repo.CreateOCRResultParams{
		AssetID:          assetID,
		ModelID:          ocrResult.ModelID,
		TotalCount:       int32(len(ocrResult.Items)),
		ProcessingTimeMs: &processingTimePtr,
	})
	if err != nil {
		return fmt.Errorf("failed to create OCR result: %w", err)
	}

	texts := make([]string, 0, len(ocrResult.Items))
	for i, item := range ocrResult.Items {
		boundingBox := dbtypes.NewBoundingBox(item.Box)
		area := boundingBox.CalculateArea()

		boundingBoxJSON, err := boundingBox.SerializeToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize bounding box for item %d: %w", i, err)
		}

		areaFloat32 := float32(area)
		_, err = s.queries.CreateOCRTextItem(ctx, repo.CreateOCRTextItemParams{
			AssetID:     assetID,
			TextContent: item.Text,
			Confidence:  item.Confidence,
			BoundingBox: boundingBoxJSON,
			TextLength:  int32(len(item.Text)),
			AreaPixels:  &areaFloat32,
		})
		if err != nil {
			return fmt.Errorf("failed to create OCR text item %d: %w", i, err)
		}

		if t := strings.TrimSpace(item.Text); t != "" {
			texts = append(texts, t)
		}
	}

	fullText := search.TokenizeForSearch(strings.Join(texts, " "))
	if err := s.queries.UpdateOCRFullText(ctx, repo.UpdateOCRFullTextParams{
		AssetID:  assetID,
		FullText: fullText,
	}); err != nil {
		return fmt.Errorf("failed to update OCR full text: %w", err)
	}

	return nil
}

// GetOCRResults gets OCR results for specified asset
func (s *ocrService) GetOCRResults(ctx context.Context, assetID pgtype.UUID) (*OCRResultWithItems, error) {
	result, err := s.queries.GetOCRResultByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get OCR result: %w", err)
	}

	items, err := s.queries.GetOCRTextItemsByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get OCR text items: %w", err)
	}

	return &OCRResultWithItems{
		Result: &result,
		Items:  items,
	}, nil
}

func (s *ocrService) SearchAssetsByText(ctx context.Context, searchText string, limit, offset int, minConfidence float32) ([]repo.Asset, error) {
	tokenized := search.TokenizeQuery(searchText)
	if tokenized == "" {
		return []repo.Asset{}, nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("search assets by OCR text: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL pg_trgm.word_similarity_threshold = 0.15"); err != nil {
		return nil, fmt.Errorf("search assets by OCR text: set threshold: %w", err)
	}

	query := `
SELECT a.*
FROM ocr_results r
JOIN assets a ON a.asset_id = r.asset_id
WHERE r.full_text %> $1
ORDER BY word_similarity($1, r.full_text) DESC, a.asset_id DESC
LIMIT $2 OFFSET $3
`
	rows, err := tx.Query(ctx, query, tokenized, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("search assets by OCR text: %w", err)
	}
	defer rows.Close()

	assets, err := pgx.CollectRows(rows, pgx.RowToStructByName[repo.Asset])
	if err != nil {
		return nil, fmt.Errorf("scan OCR search results: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("search assets by OCR text: commit: %w", err)
	}
	return assets, nil
}

// DeleteOCRResults deletes OCR results for specified asset
func (s *ocrService) DeleteOCRResults(ctx context.Context, assetID pgtype.UUID) error {
	return s.queries.DeleteOCRResultByAsset(ctx, assetID)
}

// GetOCRStats gets OCR processing statistics
func (s *ocrService) GetOCRStats(ctx context.Context) (*dbtypes.OCRStats, error) {
	stats, err := s.queries.GetOCRStatsByModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OCR stats: %w", err)
	}

	if len(stats) == 0 {
		return &dbtypes.OCRStats{}, nil
	}

	firstStat := stats[0]

	var minTime, maxTime int
	if firstStat.MinProcessingTime != nil {
		if val, ok := firstStat.MinProcessingTime.(int64); ok {
			minTime = int(val)
		} else if val, ok := firstStat.MinProcessingTime.(int32); ok {
			minTime = int(val)
		}
	}
	if firstStat.MaxProcessingTime != nil {
		if val, ok := firstStat.MaxProcessingTime.(int64); ok {
			maxTime = int(val)
		} else if val, ok := firstStat.MaxProcessingTime.(int32); ok {
			maxTime = int(val)
		}
	}

	return &dbtypes.OCRStats{
		ModelID:           firstStat.ModelID,
		TotalAssets:       int(firstStat.TotalAssets),
		TotalTextItems:    int(firstStat.TotalTextItems),
		AvgItemsPerAsset:  firstStat.AvgItemsPerAsset,
		MinProcessingTime: minTime,
		MaxProcessingTime: maxTime,
		AvgProcessingTime: firstStat.AvgProcessingTime,
	}, nil
}

// GetOCRTextItemsByAssetWithLimit gets OCR text items for specified asset (with limit)
func (s *ocrService) GetOCRTextItemsByAssetWithLimit(ctx context.Context, assetID pgtype.UUID, limit int) ([]repo.OcrTextItem, error) {
	return s.queries.GetOCRTextItemsByAssetWithLimit(ctx, repo.GetOCRTextItemsByAssetWithLimitParams{
		AssetID: assetID,
		Limit:   int32(limit),
	})
}

// GetHighConfidenceTextItems gets high confidence text items
func (s *ocrService) GetHighConfidenceTextItems(ctx context.Context, minConfidence float32, limit int) ([]repo.OcrTextItem, error) {
	return s.queries.GetHighConfidenceTextItems(ctx, repo.GetHighConfidenceTextItemsParams{
		Confidence: minConfidence,
		Limit:      int32(limit),
	})
}

// ConvertOCRToJSONMetadata converts OCR results to JSON metadata format
func (s *ocrService) ConvertOCRToJSONMetadata(ctx context.Context, assetID pgtype.UUID) (*dbtypes.OCRResultMeta, error) {
	result, err := s.queries.GetOCRResultByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}

	items, err := s.queries.GetOCRTextItemsByAssetWithLimit(ctx, repo.GetOCRTextItemsByAssetWithLimitParams{
		AssetID: assetID,
		Limit:   1,
	})
	if err != nil {
		return nil, err
	}

	firstText := ""
	if len(items) > 0 {
		firstText = items[0].TextContent
	}

	var processingTime int
	if result.ProcessingTimeMs != nil {
		processingTime = int(*result.ProcessingTimeMs)
	}

	return &dbtypes.OCRResultMeta{
		HasOCR:         true,
		TotalCount:     int(result.TotalCount),
		FirstText:      firstText,
		ProcessingTime: processingTime,
		GeneratedAt:    result.CreatedAt.Time,
		ModelID:        result.ModelID,
	}, nil
}
