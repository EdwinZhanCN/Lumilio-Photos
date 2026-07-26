package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/search"

	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/google/uuid"
)

// OCRService defines OCR related operations interface
type OCRService interface {
	SaveOCRResults(ctx context.Context, assetID uuid.UUID, ocrResult *types.OCRV1, processingTimeMs int) error
	GetOCRResults(ctx context.Context, assetID uuid.UUID) (*OCRResultWithItems, error)
	SearchAssetsByText(ctx context.Context, searchText string, limit, offset int, minConfidence float32) ([]repo.Asset, error)
	DeleteOCRResults(ctx context.Context, assetID uuid.UUID) error
	GetOCRStats(ctx context.Context) (*dbtypes.OCRStats, error)
}

// OCRResultWithItems contains OCR results and detailed text items
type OCRResultWithItems struct {
	Result *repo.OcrResult
	Items  []repo.OcrTextItem
}

type ocrService struct {
	queries *repo.Queries
	pool    *sql.DB
}

// NewOCRService creates OCR service instance
func NewOCRService(queries *repo.Queries, pool *sql.DB) OCRService {
	return &ocrService{
		queries: queries,
		pool:    pool,
	}
}

// SaveOCRResults saves OCR results to database
func (s *ocrService) SaveOCRResults(ctx context.Context, assetID uuid.UUID, ocrResult *types.OCRV1, processingTimeMs int) error {
	if err := s.queries.DeleteOCRResultByAsset(ctx, assetID); err != nil {
		return fmt.Errorf("failed to delete existing OCR results: %w", err)
	}

	processingTimePtr := int64(processingTimeMs)
	_, err := s.queries.CreateOCRResult(ctx, repo.CreateOCRResultParams{
		AssetID:          assetID,
		ModelID:          ocrResult.ModelID,
		TotalCount:       int64(len(ocrResult.Items)),
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

		areaFloat64 := float64(area)
		_, err = s.queries.CreateOCRTextItem(ctx, repo.CreateOCRTextItemParams{
			AssetID:     assetID,
			TextContent: item.Text,
			Confidence:  float64(item.Confidence),
			BoundingBox: dbtypes.JSON(boundingBoxJSON),
			TextLength:  int64(len(item.Text)),
			AreaPixels:  &areaFloat64,
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
func (s *ocrService) GetOCRResults(ctx context.Context, assetID uuid.UUID) (*OCRResultWithItems, error) {
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

	assets, err := s.queries.SearchAssetsByOCRText(ctx, repo.SearchAssetsByOCRTextParams{
		Query:  tokenized,
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, fmt.Errorf("search assets by OCR text: %w", err)
	}
	return assets, nil
}

// DeleteOCRResults deletes OCR results for specified asset
func (s *ocrService) DeleteOCRResults(ctx context.Context, assetID uuid.UUID) error {
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
		TotalTextItems:    int(derefFloat64(firstStat.TotalTextItems)),
		AvgItemsPerAsset:  derefFloat64(firstStat.AvgItemsPerAsset),
		MinProcessingTime: minTime,
		MaxProcessingTime: maxTime,
		AvgProcessingTime: derefFloat64(firstStat.AvgProcessingTime),
	}, nil
}

// GetOCRTextItemsByAssetWithLimit gets OCR text items for specified asset (with limit)
func (s *ocrService) GetOCRTextItemsByAssetWithLimit(ctx context.Context, assetID uuid.UUID, limit int) ([]repo.OcrTextItem, error) {
	return s.queries.GetOCRTextItemsByAssetWithLimit(ctx, repo.GetOCRTextItemsByAssetWithLimitParams{
		AssetID: assetID,
		Limit:   int64(limit),
	})
}

// GetHighConfidenceTextItems gets high confidence text items
func (s *ocrService) GetHighConfidenceTextItems(ctx context.Context, minConfidence float32, limit int) ([]repo.OcrTextItem, error) {
	return s.queries.GetHighConfidenceTextItems(ctx, repo.GetHighConfidenceTextItemsParams{
		Confidence: float64(minConfidence),
		Limit:      int64(limit),
	})
}

// ConvertOCRToJSONMetadata converts OCR results to JSON metadata format
func (s *ocrService) ConvertOCRToJSONMetadata(ctx context.Context, assetID uuid.UUID) (*dbtypes.OCRResultMeta, error) {
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
