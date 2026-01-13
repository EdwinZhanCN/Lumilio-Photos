package service

import (
	"context"
	"fmt"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/pgvector/pgvector-go"
)

// FaceService defines face detection and recognition related operations interface
type FaceService interface {
	SaveFaceResults(ctx context.Context, assetID pgtype.UUID, faceV1 *types.FaceV1, processingTimeMs int) error
	GetFaceResults(ctx context.Context, assetID pgtype.UUID) (*FaceResultWithItems, error)
	SearchAssetsByFaceID(ctx context.Context, faceID string, limit, offset int) ([]repo.Asset, error)
	SearchAssetsByFaceCluster(ctx context.Context, clusterID int32, limit, offset int) ([]repo.Asset, error)
	DeleteFaceResults(ctx context.Context, assetID pgtype.UUID) error
	GetFaceStats(ctx context.Context) ([]dbtypes.FaceStats, error)
	CreateFaceCluster(ctx context.Context, clusterName string, representativeFaceID int32) (*repo.FaceCluster, error)
	GetUnclusteredFaces(ctx context.Context, minConfidence float32, limit int) ([]repo.FaceItem, error)
	FindSimilarFaces(ctx context.Context, embeddingVector []float32, faceID int32, minSimilarity float32, limit int) ([]SimilarFace, error)
	UpdateFaceEmbedding(ctx context.Context, faceID int32, embedding []float32, modelID string) (*repo.FaceItem, error)
	ConvertToJSONMetadata(ctx context.Context, assetID pgtype.UUID) (*dbtypes.FaceResultMeta, error)
}

// FaceResultWithItems contains face results and detailed face items
type FaceResultWithItems struct {
	Result *repo.FaceResult
	Items  []repo.FaceItem
}

// SimilarFace represents a face with similarity score
type SimilarFace struct {
	repo.FaceItem
	Similarity float32
}

type faceService struct {
	queries *repo.Queries
}

// NewFaceService creates face service instance
func NewFaceService(queries *repo.Queries) FaceService {
	return &faceService{
		queries: queries,
	}
}

// SaveFaceResults saves face detection results from FaceV1 to database
func (s *faceService) SaveFaceResults(ctx context.Context, assetID pgtype.UUID, faceV1 *types.FaceV1, processingTimeMs int) error {
	// Delete existing face results first
	if err := s.queries.DeleteFaceResultByAsset(ctx, assetID); err != nil {
		return fmt.Errorf("failed to delete existing face results: %w", err)
	}

	// Delete existing face items
	if err := s.queries.DeleteFaceItemsByAsset(ctx, assetID); err != nil {
		return fmt.Errorf("failed to delete existing face items: %w", err)
	}

	// Save face result main record
	processingTimePtr := int32(processingTimeMs)
	_, err := s.queries.CreateFaceResult(ctx, repo.CreateFaceResultParams{
		AssetID:          assetID,
		ModelID:          faceV1.ModelID,
		TotalFaces:       int32(faceV1.Count),
		ProcessingTimeMs: &processingTimePtr,
	})
	if err != nil {
		return fmt.Errorf("failed to create face result: %w", err)
	}

	// Save each face item
	for i, face := range faceV1.Faces {
		// Convert lumen Face to database format
		faceItemMeta, err := s.convertLumenFaceToDBFace(face, i)
		if err != nil {
			return fmt.Errorf("failed to convert face %d: %w", i, err)
		}

		// Serialize bounding box to JSON
		boundingBoxJSON, err := faceItemMeta.BoundingBox.SerializeToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize bounding box for face %d: %w", i, err)
		}

		// Serialize landmarks to JSON if available
		var landmarksJSON []byte
		if faceItemMeta.Landmarks != nil {
			landmarksJSON, err = faceItemMeta.Landmarks.SerializeToJSON()
			if err != nil {
				return fmt.Errorf("failed to serialize landmarks for face %d: %w", i, err)
			}
		}

		// Convert embedding to pgvector.Vector if exists
		var embeddingVector *pgvector.Vector
		if len(face.Embedding) > 0 {
			vec := pgvector.NewVector(face.Embedding)
			embeddingVector = &vec
		}

		// Determine primary face (largest bounding box by area)
		isPrimary := i == 0 // For now, mark first face as primary
		if faceV1.Count > 1 {
			// Find the face with largest area
			maxArea := faceItemMeta.BoundingBox.GetArea()
			for j, otherFace := range faceV1.Faces {
				if j != i {
					otherBBox := dbtypes.NewFaceBoundingBoxFromLumen(otherFace.BBox)
					if otherBBox != nil && otherBBox.GetArea() > maxArea {
						isPrimary = false
						break
					}
				}
			}
		}

		_, err = s.queries.CreateFaceItem(ctx, repo.CreateFaceItemParams{
			AssetID:        assetID,
			FaceID:         nil, // Generate later if needed
			BoundingBox:    boundingBoxJSON,
			Confidence:     face.Confidence,
			AgeGroup:       nil, // Not provided by lumen-sdk
			Gender:         nil, // Not provided by lumen-sdk
			Ethnicity:      nil, // Not provided by lumen-sdk
			Expression:     nil, // Not provided by lumen-sdk
			FaceSize:       &faceItemMeta.FaceSize,
			FaceImagePath:  nil, // Not provided by lumen-sdk
			Embedding:      embeddingVector,
			EmbeddingModel: &faceV1.ModelID,
			IsPrimary:      &isPrimary,
			QualityScore:   nil,           // Not provided by lumen-sdk
			BlurScore:      nil,           // Not provided by lumen-sdk
			PoseAngles:     landmarksJSON, // Store landmarks in pose_angles field for now
		})
		if err != nil {
			return fmt.Errorf("failed to create face item %d: %w", i, err)
		}
	}

	return nil
}

// convertLumenFaceToDBFace converts lumen-sdk Face to database FaceItemMeta
func (s *faceService) convertLumenFaceToDBFace(lumenFace types.Face, index int) (*dbtypes.FaceItemMeta, error) {
	// Convert bounding box
	boundingBox := dbtypes.NewFaceBoundingBoxFromLumen(lumenFace.BBox)
	if boundingBox == nil {
		return nil, fmt.Errorf("invalid bounding box")
	}

	// Convert landmarks
	var landmarks *dbtypes.FaceLandmarks
	if len(lumenFace.Landmarks) > 0 {
		landmarks = dbtypes.NewFaceLandmarksFromLumen(lumenFace.Landmarks)
	}

	// Calculate face size (approximate area)
	faceSize := int32(boundingBox.GetArea())

	return &dbtypes.FaceItemMeta{
		ID:          int32(index),
		BoundingBox: boundingBox,
		Confidence:  lumenFace.Confidence,
		Landmarks:   landmarks,
		Embedding:   lumenFace.Embedding,
		FaceSize:    faceSize,
		CreatedAt:   time.Now(),
	}, nil
}

// GetFaceResults gets face detection results for specified asset
func (s *faceService) GetFaceResults(ctx context.Context, assetID pgtype.UUID) (*FaceResultWithItems, error) {
	// Get face result main record
	result, err := s.queries.GetFaceResultByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get face result: %w", err)
	}

	// Get all face items
	items, err := s.queries.GetFaceItemsByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get face items: %w", err)
	}

	return &FaceResultWithItems{
		Result: &result,
		Items:  items,
	}, nil
}

// DeleteFaceResults deletes face results for specified asset
func (s *faceService) DeleteFaceResults(ctx context.Context, assetID pgtype.UUID) error {
	return s.queries.DeleteFaceResultByAsset(ctx, assetID)
}

// GetFaceStats gets face detection statistics
func (s *faceService) GetFaceStats(ctx context.Context) ([]dbtypes.FaceStats, error) {
	stats, err := s.queries.GetFaceStatsByModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get face stats: %w", err)
	}

	result := make([]dbtypes.FaceStats, len(stats))
	for i, stat := range stats {
		// Helper function to safely convert interface{} to int
		toSafeInt := func(v interface{}) int {
			if v == nil {
				return 0
			}
			switch val := v.(type) {
			case int64:
				return int(val)
			case float64:
				return int(val)
			case int32:
				return int(val)
			default:
				return 0
			}
		}

		result[i] = dbtypes.FaceStats{
			ModelID:           stat.ModelID,
			TotalAssets:       int(stat.TotalAssets),
			TotalFaces:        int(stat.TotalFaces),
			AvgFacesPerAsset:  stat.AvgFacesPerAsset,
			MinProcessingTime: toSafeInt(stat.MinProcessingTime),
			MaxProcessingTime: toSafeInt(stat.MaxProcessingTime),
			AvgProcessingTime: stat.AvgProcessingTime,
		}
	}

	return result, nil
}

// ConvertToJSONMetadata converts face results to JSON metadata format
func (s *faceService) ConvertToJSONMetadata(ctx context.Context, assetID pgtype.UUID) (*dbtypes.FaceResultMeta, error) {
	result, err := s.queries.GetFaceResultByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}

	// Get primary face as preview
	items, err := s.queries.GetPrimaryFaces(ctx, repo.GetPrimaryFacesParams{
		Confidence: 0.5, // Minimum confidence threshold
		Limit:      1,
	})
	if err != nil {
		return nil, err
	}

	hasPrimary := len(items) > 0

	var processingTime int
	if result.ProcessingTimeMs != nil {
		processingTime = int(*result.ProcessingTimeMs)
	}

	return &dbtypes.FaceResultMeta{
		HasFaces:       true,
		TotalFaces:     int(result.TotalFaces),
		HasPrimaryFace: hasPrimary,
		ProcessingTime: processingTime,
		GeneratedAt:    result.CreatedAt.Time,
		ModelID:        result.ModelID,
	}, nil
}

// Implement remaining required methods with basic functionality
func (s *faceService) SearchAssetsByFaceID(ctx context.Context, faceID string, limit, offset int) ([]repo.Asset, error) {
	return s.queries.SearchAssetsByFaceID(ctx, repo.SearchAssetsByFaceIDParams{
		FaceID: &faceID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
}

func (s *faceService) SearchAssetsByFaceCluster(ctx context.Context, clusterID int32, limit, offset int) ([]repo.Asset, error) {
	return s.queries.SearchAssetsByFaceCluster(ctx, repo.SearchAssetsByFaceClusterParams{
		ClusterID: clusterID,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
}

func (s *faceService) CreateFaceCluster(ctx context.Context, clusterName string, representativeFaceID int32) (*repo.FaceCluster, error) {
	confidence := float32(0.0)
	isConfirmed := false
	cluster, err := s.queries.CreateFaceCluster(ctx, repo.CreateFaceClusterParams{
		ClusterName:          &clusterName,
		RepresentativeFaceID: representativeFaceID,
		ConfidenceScore:      &confidence,
		IsConfirmed:          &isConfirmed,
	})
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

func (s *faceService) GetUnclusteredFaces(ctx context.Context, minConfidence float32, limit int) ([]repo.FaceItem, error) {
	return s.queries.GetUnclusteredFaces(ctx, repo.GetUnclusteredFacesParams{
		Confidence: minConfidence,
		Limit:      int32(limit),
	})
}

func (s *faceService) FindSimilarFaces(ctx context.Context, embeddingVector []float32, faceID int32, minSimilarity float32, limit int) ([]SimilarFace, error) {
	// Convert embedding to pgvector types
	pgVector := pgvector.NewVector(embeddingVector)

	// Get similar faces from database
	rows, err := s.queries.GetSimilarFaces(ctx, repo.GetSimilarFacesParams{
		Column1:   &pgVector,
		ID:        faceID,
		Embedding: &pgVector,
		Limit:     int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find similar faces: %w", err)
	}

	// Convert to SimilarFace format, filter by similarity threshold
	var result []SimilarFace
	for _, row := range rows {
		similarity := float32(row.Similarity) / 1000.0 // Convert from int to float32 and normalize
		if similarity >= minSimilarity {
			result = append(result, SimilarFace{
				FaceItem: repo.FaceItem{
					ID:             row.ID,
					AssetID:        row.AssetID,
					FaceID:         row.FaceID,
					BoundingBox:    row.BoundingBox,
					Confidence:     row.Confidence,
					AgeGroup:       row.AgeGroup,
					Gender:         row.Gender,
					Ethnicity:      row.Ethnicity,
					Expression:     row.Expression,
					FaceSize:       row.FaceSize,
					FaceImagePath:  row.FaceImagePath,
					Embedding:      row.Embedding,
					EmbeddingModel: row.EmbeddingModel,
					IsPrimary:      row.IsPrimary,
					QualityScore:   row.QualityScore,
					BlurScore:      row.BlurScore,
					PoseAngles:     row.PoseAngles,
					CreatedAt:      row.CreatedAt,
				},
				Similarity: similarity,
			})
		}
	}

	return result, nil
}

func (s *faceService) UpdateFaceEmbedding(ctx context.Context, faceID int32, embedding []float32, modelID string) (*repo.FaceItem, error) {
	var embeddingVector *pgvector.Vector
	if len(embedding) > 0 {
		vec := pgvector.NewVector(embedding)
		embeddingVector = &vec
	}

	item, err := s.queries.UpdateFaceItemEmbedding(ctx, repo.UpdateFaceItemEmbeddingParams{
		ID:             faceID,
		Embedding:      embeddingVector,
		EmbeddingModel: &modelID,
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
}
