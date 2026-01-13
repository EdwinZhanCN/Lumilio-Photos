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

// AIDescriptionService defines AI description related operations interface
type AIDescriptionService interface {
	GenerateAndSaveDescription(ctx context.Context, assetID pgtype.UUID, imageData []byte, customPrompt string) (*dbtypes.AIDescriptionResponse, error)
	GetAIDescription(ctx context.Context, assetID pgtype.UUID) (*repo.AiDescription, error)
	SearchAssetsByDescription(ctx context.Context, searchText string, limit, offset int, minConfidence float64) ([]repo.Asset, error)
	DeleteAIDescription(ctx context.Context, assetID pgtype.UUID) error
	GetAIDescriptionStats(ctx context.Context) ([]dbtypes.AIDescriptionStats, error)
	ConvertToJSONMetadata(ctx context.Context, assetID pgtype.UUID) (*dbtypes.AIDescriptionMeta, error)
}

type aiDescriptionService struct {
	queries      *repo.Queries
	lumenService LumenService
}

// NewAIDescriptionService creates AI description service instance
func NewAIDescriptionService(queries *repo.Queries, lumenService LumenService) AIDescriptionService {
	return &aiDescriptionService{
		queries:      queries,
		lumenService: lumenService,
	}
}

// GenerateAndSaveDescription generates AI description and saves it to database
func (s *aiDescriptionService) GenerateAndSaveDescription(ctx context.Context, assetID pgtype.UUID, imageData []byte, customPrompt string) (*dbtypes.AIDescriptionResponse, error) {
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

	// Delete existing AI description
	if err := s.queries.DeleteAIDescriptionByAsset(ctx, assetID); err != nil {
		return nil, fmt.Errorf("failed to delete existing AI description: %w", err)
	}

	// Save to database
	confidenceFloat32 := float32(confidence)
	processingTimeInt32 := int32(processingTimeMs)
	tokensInt32 := int32(tokensGenerated)

	// Convert string fields to pointers for the database struct
	summaryPtr := &summary
	promptPtr := &prompt
	finishReasonPtr := &finishReason

	dbRecord, err := s.queries.CreateAIDescription(ctx, repo.CreateAIDescriptionParams{
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
		return nil, fmt.Errorf("failed to create AI description: %w", err)
	}

	return &dbtypes.AIDescriptionResponse{
		AssetID:          assetID.String(),
		Description:      description,
		Summary:          summary,
		ModelID:          modelID,
		Confidence:       confidence,
		TokensGenerated:  tokensGenerated,
		ProcessingTime:   processingTimeMs,
		FinishReason:     finishReason,
		PromptUsed:       prompt,
		GeneratedAt:      dbRecord.CreatedAt.Time,
	}, nil
}

// GetAIDescription gets AI description for specified asset
func (s *aiDescriptionService) GetAIDescription(ctx context.Context, assetID pgtype.UUID) (*repo.AiDescription, error) {
	description, err := s.queries.GetAIDescriptionByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI description: %w", err)
	}
	return &description, nil
}

// SearchAssetsByDescription searches assets by AI description text
func (s *aiDescriptionService) SearchAssetsByDescription(ctx context.Context, searchText string, limit, offset int, minConfidence float64) ([]repo.Asset, error) {
	// Clean and prepare search text
	searchText = strings.TrimSpace(searchText)
	if searchText == "" {
		return []repo.Asset{}, nil
	}

	if minConfidence > 0 {
		minConfidenceFloat32 := float32(minConfidence)
		return s.queries.SearchAssetsByAIDescriptionWithConfidence(ctx,
			repo.SearchAssetsByAIDescriptionWithConfidenceParams{
				PlaintoTsquery: searchText,
				Limit:          int32(limit),
				Offset:         int32(offset),
				Confidence:     &minConfidenceFloat32,
			})
	}
	return s.queries.SearchAssetsByAIDescription(ctx,
		repo.SearchAssetsByAIDescriptionParams{
			PlaintoTsquery: searchText,
			Limit:          int32(limit),
			Offset:         int32(offset),
		})
}

// DeleteAIDescription deletes AI description for specified asset
func (s *aiDescriptionService) DeleteAIDescription(ctx context.Context, assetID pgtype.UUID) error {
	return s.queries.DeleteAIDescriptionByAsset(ctx, assetID)
}

// GetAIDescriptionStats gets AI description processing statistics
func (s *aiDescriptionService) GetAIDescriptionStats(ctx context.Context) ([]dbtypes.AIDescriptionStats, error) {
	stats, err := s.queries.GetAIDescriptionStatsByModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI description stats: %w", err)
	}

	result := make([]dbtypes.AIDescriptionStats, len(stats))
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

		result[i] = dbtypes.AIDescriptionStats{
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

// ConvertToJSONMetadata converts AI description to JSON metadata format
func (s *aiDescriptionService) ConvertToJSONMetadata(ctx context.Context, assetID pgtype.UUID) (*dbtypes.AIDescriptionMeta, error) {
	description, err := s.queries.GetAIDescriptionByAsset(ctx, assetID)
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

	return &dbtypes.AIDescriptionMeta{
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

// GetTopAIDescriptionsByTokens gets descriptions with highest token counts
func (s *aiDescriptionService) GetTopAIDescriptionsByTokens(ctx context.Context, limit int) ([]repo.AiDescription, error) {
	return s.queries.GetTopAIDescriptionsByTokens(ctx, int32(limit))
}

// GetLongAIDescriptions gets descriptions with longest text
func (s *aiDescriptionService) GetLongAIDescriptions(ctx context.Context, minLength int, limit int) ([]repo.AiDescription, error) {
	return s.queries.GetLongAIDescriptions(ctx, repo.GetLongAIDescriptionsParams{
		Description: fmt.Sprintf("%d", minLength), // Pass minLength as string for the LENGTH(description) > $1 comparison
		Limit:       int32(limit),
	})
}

// RegenerateDescription regenerates AI description for an asset
func (s *aiDescriptionService) RegenerateDescription(ctx context.Context, assetID pgtype.UUID, imageData []byte, customPrompt string) (*dbtypes.AIDescriptionResponse, error) {
	// Delete existing description
	if err := s.queries.DeleteAIDescriptionByAsset(ctx, assetID); err != nil {
		return nil, fmt.Errorf("failed to delete existing description: %w", err)
	}

	// Generate new description
	return s.GenerateAndSaveDescription(ctx, assetID, imageData, customPrompt)
}