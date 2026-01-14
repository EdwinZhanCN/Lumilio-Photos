package service

import (
	"context"
	"fmt"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/jackc/pgx/v5/pgtype"
)

// SpeciesService defines species prediction related operations interface
type SpeciesService interface {
	SaveSpeciesPredictions(ctx context.Context, assetID pgtype.UUID, predictions []dbtypes.SpeciesPredictionMeta) error
	GetSpeciesPredictionsByAsset(ctx context.Context, assetID pgtype.UUID) ([]repo.SpeciesPrediction, error)
	GetTopSpeciesForAsset(ctx context.Context, assetID pgtype.UUID, minScore float32, limit int) ([]repo.SpeciesPrediction, error)
	SearchAssetsBySpecies(ctx context.Context, query string, limit, offset int) ([]repo.Asset, error)
	GetSpeciesPredictionsByLabel(ctx context.Context, label string, limit, offset int) ([]repo.SpeciesPrediction, error)
	DeleteSpeciesPredictions(ctx context.Context, assetID pgtype.UUID) error
	GetSpeciesStats(ctx context.Context) (*repo.GetSpeciesStatsRow, error)
	GetTopSpeciesLabels(ctx context.Context, limit int) ([]repo.GetTopSpeciesLabelsRow, error)
}

type speciesService struct {
	queries *repo.Queries
}

// NewSpeciesService creates a new species service instance
func NewSpeciesService(queries *repo.Queries) SpeciesService {
	return &speciesService{
		queries: queries,
	}
}

// SaveSpeciesPredictions saves species predictions to database
func (s *speciesService) SaveSpeciesPredictions(ctx context.Context, assetID pgtype.UUID, predictions []dbtypes.SpeciesPredictionMeta) error {
	// Delete existing predictions first
	if err := s.queries.DeleteSpeciesPredictionsByAsset(ctx, assetID); err != nil {
		return fmt.Errorf("failed to delete existing species predictions: %w", err)
	}

	// Insert new predictions
	for _, pred := range predictions {
		params := repo.CreateSpeciesPredictionParams{
			AssetID: assetID,
			Label:   pred.Label,
			Score:   pred.Score,
		}
		if _, err := s.queries.CreateSpeciesPrediction(ctx, params); err != nil {
			return fmt.Errorf("failed to create species prediction: %w", err)
		}
	}

	return nil
}

// GetSpeciesPredictionsByAsset retrieves all species predictions for an asset
func (s *speciesService) GetSpeciesPredictionsByAsset(ctx context.Context, assetID pgtype.UUID) ([]repo.SpeciesPrediction, error) {
	predictions, err := s.queries.GetSpeciesPredictionsByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get species predictions: %w", err)
	}
	return predictions, nil
}

// GetTopSpeciesForAsset retrieves top species predictions with minimum confidence
func (s *speciesService) GetTopSpeciesForAsset(ctx context.Context, assetID pgtype.UUID, minScore float32, limit int) ([]repo.SpeciesPrediction, error) {
	params := repo.GetTopSpeciesForAssetParams{
		AssetID: assetID,
		Score:   minScore,
		Limit:   int32(limit),
	}

	predictions, err := s.queries.GetTopSpeciesForAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get top species predictions: %w", err)
	}
	return predictions, nil
}

// SearchAssetsBySpecies searches assets by species label
func (s *speciesService) SearchAssetsBySpecies(ctx context.Context, query string, limit, offset int) ([]repo.Asset, error) {
	params := repo.SearchAssetsBySpeciesParams{
		Column1: &query,
		Offset:  int32(offset),
		Limit:   int32(limit),
	}

	assets, err := s.queries.SearchAssetsBySpecies(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to search assets by species: %w", err)
	}
	return assets, nil
}

// GetSpeciesPredictionsByLabel retrieves predictions for a specific label
func (s *speciesService) GetSpeciesPredictionsByLabel(ctx context.Context, label string, limit, offset int) ([]repo.SpeciesPrediction, error) {
	params := repo.GetSpeciesPredictionsByLabelParams{
		Label:  label,
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	predictions, err := s.queries.GetSpeciesPredictionsByLabel(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to get predictions by label: %w", err)
	}
	return predictions, nil
}

// DeleteSpeciesPredictions deletes all species predictions for an asset
func (s *speciesService) DeleteSpeciesPredictions(ctx context.Context, assetID pgtype.UUID) error {
	if err := s.queries.DeleteSpeciesPredictionsByAsset(ctx, assetID); err != nil {
		return fmt.Errorf("failed to delete species predictions: %w", err)
	}
	return nil
}

// GetSpeciesStats retrieves statistics about species predictions
func (s *speciesService) GetSpeciesStats(ctx context.Context) (*repo.GetSpeciesStatsRow, error) {
	stats, err := s.queries.GetSpeciesStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get species stats: %w", err)
	}
	return &stats, nil
}

// GetTopSpeciesLabels retrieves the most common species labels
func (s *speciesService) GetTopSpeciesLabels(ctx context.Context, limit int) ([]repo.GetTopSpeciesLabelsRow, error) {
	labels, err := s.queries.GetTopSpeciesLabels(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("failed to get top species labels: %w", err)
	}
	return labels, nil
}
