package service

import (
	"context"
	"fmt"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// CaptionService defines caption related operations interface
type CaptionService interface {
	GenerateAndSaveCaption(ctx context.Context, assetID pgtype.UUID, imageData []byte, customPrompt string) (*dbtypes.CaptionResponse, error)
	GetCaption(ctx context.Context, assetID pgtype.UUID) (*repo.Caption, error)
	SearchAssetsByCaption(ctx context.Context, searchText string, limit, offset int, minConfidence float64) ([]repo.Asset, error)
	DeleteCaption(ctx context.Context, assetID pgtype.UUID) error
	GetCaptionStats(ctx context.Context) ([]dbtypes.CaptionStats, error)
	ConvertToJSONMetadata(ctx context.Context, assetID pgtype.UUID) (*dbtypes.CaptionMeta, error)
}

type captionService struct {
	queries      *repo.Queries
	lumenService LumenService
}

// NewCaptionService creates caption service instance
func NewCaptionService(queries *repo.Queries, lumenService LumenService) CaptionService {
	return &captionService{
		queries:      queries,
		lumenService: lumenService,
	}
}

// GenerateAndSaveCaption generates caption and saves it to database
func (s *captionService) GenerateAndSaveCaption(ctx context.Context, assetID pgtype.UUID, imageData []byte, customPrompt string) (*dbtypes.CaptionResponse, error) {
	startTime := time.Now()

	// Prepare prompt
	prompt := customPrompt
	if prompt == "" {
		prompt = "<image>Describe this image in detail."
	}

	// Generate description using VLM with metadata
	captionResp, err := s.lumenService.VLMCaptionWithMetadata(ctx, imageData, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate VLM caption: %w", err)
	}

	description := captionResp.Text

	// Calculate processing time
	processingTimeMs := int(time.Since(startTime).Milliseconds())

	// Generate summary (first 200 characters)
	summary := dbtypes.GenerateSummary(description, 200)

	// Use real data from the VLM response instead of placeholders
	tokensGenerated := captionResp.GeneratedTokens
	modelID := captionResp.ModelID
	finishReason := captionResp.FinishReason
	confidence := 0.8 // Still using default confidence as it's not provided by the response

	// Delete existing caption
	if err := s.queries.DeleteCaptionByAsset(ctx, assetID); err != nil {
		return nil, fmt.Errorf("failed to delete existing caption: %w", err)
	}

	// Save to database
	confidenceFloat32 := float32(confidence)
	processingTimeInt32 := int32(processingTimeMs)
	tokensInt32 := int32(tokensGenerated)

	// Convert string fields to pointers for the database struct
	summaryPtr := &summary
	promptPtr := &prompt
	finishReasonPtr := &finishReason

	dbRecord, err := s.queries.CreateCaption(ctx, repo.CreateCaptionParams{
		AssetID:          assetID,
		ModelID:          modelID,
		Description:      description,
		Summary:          summaryPtr,
		Confidence:       &confidenceFloat32,
		TokensGenerated:  &tokensInt32,
		ProcessingTimeMs: &processingTimeInt32,
		PromptUsed:       promptPtr,
		FinishReason:     finishReasonPtr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create caption: %w", err)
	}

	return &dbtypes.CaptionResponse{
		AssetID:         assetID.String(),
		Description:     description,
		Summary:         summary,
		ModelID:         modelID,
		Confidence:      confidence,
		TokensGenerated: tokensGenerated,
		ProcessingTime:  processingTimeMs,
		FinishReason:    finishReason,
		PromptUsed:      prompt,
		GeneratedAt:     dbRecord.CreatedAt.Time,
	}, nil
}

// GetCaption gets caption for specified asset
func (s *captionService) GetCaption(ctx context.Context, assetID pgtype.UUID) (*repo.Caption, error) {
	description, err := s.queries.GetCaptionByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get caption: %w", err)
	}
	return &description, nil
}

// SearchAssetsByCaption searches assets by caption text
func (s *captionService) SearchAssetsByCaption(ctx context.Context, searchText string, limit, offset int, minConfidence float64) ([]repo.Asset, error) {
	// Clean and prepare search text
	searchText = strings.TrimSpace(searchText)
	if searchText == "" {
		return []repo.Asset{}, nil
	}

	if minConfidence > 0 {
		minConfidenceFloat32 := float32(minConfidence)
		return s.queries.SearchAssetsByCaptionWithConfidence(ctx,
			repo.SearchAssetsByCaptionWithConfidenceParams{
				PlaintoTsquery: searchText,
				Limit:          int32(limit),
				Offset:         int32(offset),
				Confidence:     &minConfidenceFloat32,
			})
	}
	return s.queries.SearchAssetsByCaption(ctx,
		repo.SearchAssetsByCaptionParams{
			PlaintoTsquery: searchText,
			Limit:          int32(limit),
			Offset:         int32(offset),
		})
}

// DeleteCaption deletes caption for specified asset
func (s *captionService) DeleteCaption(ctx context.Context, assetID pgtype.UUID) error {
	return s.queries.DeleteCaptionByAsset(ctx, assetID)
}

// GetCaptionStats gets caption processing statistics
func (s *captionService) GetCaptionStats(ctx context.Context) ([]dbtypes.CaptionStats, error) {
	stats, err := s.queries.GetCaptionStatsByModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get caption stats: %w", err)
	}

	result := make([]dbtypes.CaptionStats, len(stats))
	for i, stat := range stats {
		// Helper function to safely convert interface{} to int64
		toSafeInt64 := func(v interface{}) int64 {
			if v == nil {
				return 0
			}
			switch val := v.(type) {
			case int64:
				return val
			case float64:
				return int64(val)
			case int32:
				return int64(val)
			default:
				return 0
			}
		}

		result[i] = dbtypes.CaptionStats{
			ModelID:           stat.ModelID,
			TotalDescriptions: int(stat.TotalDescriptions),
			AvgTokens:         stat.AvgTokens,
			MinTokens:         toSafeInt64(stat.MinTokens),
			MaxTokens:         toSafeInt64(stat.MaxTokens),
			AvgProcessingTime: stat.AvgProcessingTime,
			MinProcessingTime: toSafeInt64(stat.MinProcessingTime),
			MaxProcessingTime: toSafeInt64(stat.MaxProcessingTime),
			AvgConfidence:     stat.AvgConfidence,
		}
	}

	return result, nil
}

// ConvertToJSONMetadata converts caption to JSON metadata format
func (s *captionService) ConvertToJSONMetadata(ctx context.Context, assetID pgtype.UUID) (*dbtypes.CaptionMeta, error) {
	description, err := s.queries.GetCaptionByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}

	var tokensGenerated int
	if description.TokensGenerated != nil {
		tokensGenerated = int(*description.TokensGenerated)
	}

	var confidence float64
	if description.Confidence != nil {
		confidence = float64(*description.Confidence)
	}

	var processingTime int
	if description.ProcessingTimeMs != nil {
		processingTime = int(*description.ProcessingTimeMs)
	}

	// Handle nullable pointer fields
	var summary string
	if description.Summary != nil {
		summary = *description.Summary
	}

	var finishReason string
	if description.FinishReason != nil {
		finishReason = *description.FinishReason
	}

	return &dbtypes.CaptionMeta{
		HasDescription:  true,
		Summary:         summary,
		ModelID:         description.ModelID,
		TokensGenerated: tokensGenerated,
		Confidence:      confidence,
		GeneratedAt:     description.CreatedAt.Time,
		ProcessingTime:  processingTime,
		FinishReason:    finishReason,
	}, nil
}

// GetTopCaptionsByTokens gets descriptions with highest token counts
func (s *captionService) GetTopCaptionsByTokens(ctx context.Context, limit int) ([]repo.Caption, error) {
	return s.queries.GetTopCaptionsByTokens(ctx, int32(limit))
}

// GetLongCaptions gets descriptions with longest text
func (s *captionService) GetLongCaptions(ctx context.Context, minLength int, limit int) ([]repo.Caption, error) {
	return s.queries.GetLongCaptions(ctx, repo.GetLongCaptionsParams{
		MinLength: int32(minLength),
		RowLimit:  int32(limit),
	})
}

// RegenerateCaption regenerates caption for an asset
func (s *captionService) RegenerateCaption(ctx context.Context, assetID pgtype.UUID, imageData []byte, customPrompt string) (*dbtypes.CaptionResponse, error) {
	// Delete existing description
	if err := s.queries.DeleteCaptionByAsset(ctx, assetID); err != nil {
		return nil, fmt.Errorf("failed to delete existing description: %w", err)
	}

	// Generate new description
	return s.GenerateAndSaveCaption(ctx, assetID, imageData, customPrompt)
}
