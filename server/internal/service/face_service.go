package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/edwinzhancn/lumen-sdk/pkg/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

const (
	faceRecognitionMinScore    = float32(0.70)
	faceRecognitionMaxDistance = float64(0.50)
	faceRecognitionMinFaces    = 3
	faceCropPaddingMultiplier  = float32(0.12)
	faceCropQuality            = 85
)

type faceRepositoryPathResolver interface {
	GetRepositoryPath(repoID string) (string, error)
}

// FaceService defines face detection and recognition related operations interface.
type FaceService interface {
	SaveFaceResults(ctx context.Context, assetID pgtype.UUID, faceV1 *types.FaceV1, imageData []byte, processingTimeMs int) error
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
	ListPeople(ctx context.Context, repositoryID pgtype.UUID, ownerID *int32, includeHidden bool, limit, offset int) ([]Person, int64, error)
	GetPerson(ctx context.Context, clusterID int32, repositoryID pgtype.UUID, ownerID *int32) (*Person, error)
	RenamePerson(ctx context.Context, clusterID int32, name string) (*repo.FaceCluster, error)
	RebuildFaceClusters(ctx context.Context, repositoryID pgtype.UUID, ownerID *int32) (FaceClusterRebuildResult, error)
	ListPersonFaces(ctx context.Context, clusterID int32, repositoryID pgtype.UUID, ownerID *int32, limit, offset int) ([]PersonFace, int64, error)
	GetPersonFaceCrop(ctx context.Context, clusterID, faceID int32, repositoryID pgtype.UUID, ownerID *int32) (*PersonFaceCrop, error)
	MergePeople(ctx context.Context, targetClusterID int32, sourceClusterIDs []int32, repositoryID pgtype.UUID, ownerID *int32) error
	MoveFace(ctx context.Context, faceID, targetClusterID int32, repositoryID pgtype.UUID, ownerID *int32) error
	RemoveFaceFromPerson(ctx context.Context, faceID, clusterID int32, repositoryID pgtype.UUID, ownerID *int32) error
	SetPersonCover(ctx context.Context, clusterID, faceID int32, repositoryID pgtype.UUID, ownerID *int32) error
	SetPersonHidden(ctx context.Context, clusterID int32, hidden bool, repositoryID pgtype.UUID, ownerID *int32) (*Person, error)
}

// FaceResultWithItems contains face results and detailed face items.
type FaceResultWithItems struct {
	Result *repo.FaceResult
	Items  []repo.FaceItem
}

// SimilarFace represents a face with similarity score.
type SimilarFace struct {
	repo.FaceItem
	Similarity float32
}

// Person is the service-level representation of a face cluster exposed as a person.
type Person struct {
	PersonID              int32
	Name                  *string
	IsConfirmed           bool
	IsHidden              bool
	HiddenAt              *time.Time
	MemberCount           int64
	AssetCount            int64
	CoverFaceImagePath    *string
	RepresentativeAssetID *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// PersonFace is a UI-safe view of a single face that belongs to a person.
// It intentionally omits embeddings, bounding boxes, pose angles and any
// demographic attributes.
type PersonFace struct {
	FaceID           int32
	AssetID          string
	Confidence       float32
	IsRepresentative bool
	IsManual         bool
	HasCrop          bool
	Filename         string
	TakenTime        *time.Time
	UploadTime       *time.Time
}

// PersonFaceCrop locates a single face crop on disk for media serving.
type PersonFaceCrop struct {
	FaceID        int32
	RepositoryID  string
	FaceImagePath string
}

// FaceClusterRebuildResult summarizes a full people-cluster rebuild.
type FaceClusterRebuildResult struct {
	Algorithm       string
	RepositoryID    *string
	CandidateFaces  int
	ClusteredFaces  int
	NoiseFaces      int
	ClustersCreated int
	ClustersReused  int
	ClustersTotal   int
	DurationMs      int64
}

type faceService struct {
	queries          *repo.Queries
	pool             *pgxpool.Pool
	repoPathResolver faceRepositoryPathResolver
}

// NewFaceService creates face service instance.
func NewFaceService(queries *repo.Queries, repoPathResolver faceRepositoryPathResolver, pool ...*pgxpool.Pool) FaceService {
	var pgPool *pgxpool.Pool
	if len(pool) > 0 {
		pgPool = pool[0]
	}
	return &faceService{
		queries:          queries,
		pool:             pgPool,
		repoPathResolver: repoPathResolver,
	}
}

func (s *faceService) withTx(ctx context.Context, fn func(*repo.Queries) error) error {
	if s.pool == nil {
		return fn(s.queries)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin face clustering transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(s.queries.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit face clustering transaction: %w", err)
	}
	return nil
}

// SaveFaceResults saves face detection results, persists crops, and updates recognition clusters.
func (s *faceService) SaveFaceResults(ctx context.Context, assetID pgtype.UUID, faceV1 *types.FaceV1, imageData []byte, processingTimeMs int) error {
	if faceV1 == nil {
		return fmt.Errorf("face result payload is required")
	}

	repoPath, asset, err := s.resolveAssetRepository(ctx, assetID)
	if err != nil {
		return err
	}

	affectedClusters, err := s.cleanupExistingFaceState(ctx, assetID, repoPath)
	if err != nil {
		return err
	}
	if err := s.cleanupAffectedClusters(ctx, affectedClusters); err != nil {
		return err
	}

	processingTimePtr := int32(processingTimeMs)
	if _, err := s.queries.CreateFaceResult(ctx, repo.CreateFaceResultParams{
		AssetID:          assetID,
		ModelID:          faceV1.ModelID,
		TotalFaces:       int32(faceV1.Count),
		ProcessingTimeMs: &processingTimePtr,
	}); err != nil {
		return fmt.Errorf("failed to create face result: %w", err)
	}

	primaryFaceIndex := largestFaceIndex(faceV1.Faces)
	createdItems := make([]repo.FaceItem, 0, len(faceV1.Faces))

	for i, face := range faceV1.Faces {
		faceItemMeta, err := s.convertLumenFaceToDBFace(face, i)
		if err != nil {
			return fmt.Errorf("failed to convert face %d: %w", i, err)
		}

		boundingBoxJSON, err := faceItemMeta.BoundingBox.SerializeToJSON()
		if err != nil {
			return fmt.Errorf("failed to serialize bounding box for face %d: %w", i, err)
		}

		var landmarksJSON []byte
		if faceItemMeta.Landmarks != nil {
			landmarksJSON, err = faceItemMeta.Landmarks.SerializeToJSON()
			if err != nil {
				return fmt.Errorf("failed to serialize landmarks for face %d: %w", i, err)
			}
		}

		var embeddingVector *pgvector.Vector
		if len(face.Embedding) > 0 {
			vec := pgvector.NewVector(face.Embedding)
			embeddingVector = &vec
		}

		isPrimary := i == primaryFaceIndex

		faceImagePath, err := s.persistFaceCrop(repoPath, assetID, i, imageData, faceItemMeta.BoundingBox)
		if err != nil {
			return fmt.Errorf("failed to persist face crop %d: %w", i, err)
		}

		createdItem, err := s.queries.CreateFaceItem(ctx, repo.CreateFaceItemParams{
			AssetID:        assetID,
			FaceID:         nil,
			BoundingBox:    boundingBoxJSON,
			Confidence:     face.Confidence,
			AgeGroup:       nil,
			Gender:         nil,
			Ethnicity:      nil,
			Expression:     nil,
			FaceSize:       &faceItemMeta.FaceSize,
			FaceImagePath:  faceImagePath,
			Embedding:      embeddingVector,
			EmbeddingModel: &faceV1.ModelID,
			IsPrimary:      &isPrimary,
			QualityScore:   nil,
			BlurScore:      nil,
			PoseAngles:     landmarksJSON,
		})
		if err != nil {
			return fmt.Errorf("failed to create face item %d: %w", i, err)
		}
		createdItems = append(createdItems, createdItem)
	}

	if err := s.recognizePendingFacesForAsset(ctx, *asset, createdItems); err != nil {
		return fmt.Errorf("recognize pending faces: %w", err)
	}

	return nil
}

func largestFaceIndex(faces []types.Face) int {
	if len(faces) == 0 {
		return 0
	}

	bestIndex := 0
	bestArea := float32(-1)
	for i, face := range faces {
		bbox := dbtypes.NewFaceBoundingBoxFromLumen(face.BBox)
		if bbox == nil {
			continue
		}
		area := bbox.GetArea()
		if area > bestArea {
			bestArea = area
			bestIndex = i
		}
	}

	return bestIndex
}

func (s *faceService) convertLumenFaceToDBFace(lumenFace types.Face, index int) (*dbtypes.FaceItemMeta, error) {
	boundingBox := dbtypes.NewFaceBoundingBoxFromLumen(lumenFace.BBox)
	if boundingBox == nil {
		return nil, fmt.Errorf("invalid bounding box")
	}

	var landmarks *dbtypes.FaceLandmarks
	if len(lumenFace.Landmarks) > 0 {
		landmarks = dbtypes.NewFaceLandmarksFromLumen(lumenFace.Landmarks)
	}

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

func (s *faceService) resolveAssetRepository(ctx context.Context, assetID pgtype.UUID) (string, *repo.Asset, error) {
	asset, err := s.queries.GetAssetByID(ctx, assetID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load asset for face processing: %w", err)
	}
	if !asset.RepositoryID.Valid {
		return "", nil, fmt.Errorf("asset %s does not have a repository", pgUUIDToString(assetID))
	}
	if s.repoPathResolver == nil {
		return "", nil, fmt.Errorf("face repository path resolver is unavailable")
	}

	repositoryID := pgUUIDToString(asset.RepositoryID)
	repoPath, err := s.repoPathResolver.GetRepositoryPath(repositoryID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve repository path for asset %s: %w", pgUUIDToString(assetID), err)
	}

	return repoPath, &asset, nil
}

func (s *faceService) cleanupExistingFaceState(ctx context.Context, assetID pgtype.UUID, repoPath string) ([]int32, error) {
	existingItems, err := s.queries.GetFaceItemsByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing face items: %w", err)
	}

	affectedClusterIDs, err := s.collectAffectedClusterIDs(ctx, existingItems)
	if err != nil {
		return nil, err
	}

	for _, item := range existingItems {
		if item.FaceImagePath == nil || strings.TrimSpace(*item.FaceImagePath) == "" {
			continue
		}
		if err := removeRepositoryFile(repoPath, *item.FaceImagePath); err != nil {
			return nil, fmt.Errorf("failed to remove previous face crop: %w", err)
		}
	}

	if err := s.queries.DeleteFaceResultByAsset(ctx, assetID); err != nil {
		return nil, fmt.Errorf("failed to delete existing face result: %w", err)
	}
	if err := s.queries.DeleteFaceItemsByAsset(ctx, assetID); err != nil {
		return nil, fmt.Errorf("failed to delete existing face items: %w", err)
	}

	return affectedClusterIDs, nil
}

func (s *faceService) collectAffectedClusterIDs(ctx context.Context, items []repo.FaceItem) ([]int32, error) {
	clusterIDs := make(map[int32]struct{})
	for _, item := range items {
		cluster, err := s.queries.GetFaceClusterByFaceID(ctx, item.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return nil, fmt.Errorf("failed to load cluster for face %d: %w", item.ID, err)
		}
		clusterIDs[cluster.ClusterID] = struct{}{}
	}

	ids := make([]int32, 0, len(clusterIDs))
	for id := range clusterIDs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (s *faceService) cleanupAffectedClusters(ctx context.Context, clusterIDs []int32) error {
	for _, clusterID := range clusterIDs {
		if err := s.refreshClusterRepresentative(ctx, clusterID); err != nil {
			return err
		}
	}
	return nil
}

func (s *faceService) assignFaceToCluster(ctx context.Context, item repo.FaceItem, asset repo.Asset) error {
	if !isClusterCandidate(item) || item.Embedding == nil || !asset.RepositoryID.Valid {
		return nil
	}
	return s.recognizePendingFaces(ctx, faceClusterScope{
		RepositoryID:   asset.RepositoryID,
		OwnerID:        cloneInt32Ptr(asset.OwnerID),
		EmbeddingModel: normalizedName(item.EmbeddingModel),
	})
}

func (s *faceService) createClusterForFace(ctx context.Context, item repo.FaceItem) (*repo.FaceCluster, error) {
	return s.createClusterForFaceWithQueries(ctx, s.queries, item, nil, false)
}

func (s *faceService) createClusterForFaceWithQueries(ctx context.Context, q *repo.Queries, item repo.FaceItem, name *string, confirmed bool) (*repo.FaceCluster, error) {
	cluster, err := q.CreateFaceCluster(ctx, repo.CreateFaceClusterParams{
		ClusterName:          normalizedName(name),
		RepresentativeFaceID: &item.ID,
		ConfidenceScore:      &item.Confidence,
		IsConfirmed:          boolPtr(confirmed),
	})
	if err != nil {
		return nil, fmt.Errorf("create face cluster: %w", err)
	}

	if _, err := q.AssignFaceClusterMemberExclusive(ctx, repo.AssignFaceClusterMemberExclusiveParams{
		ClusterID:       cluster.ClusterID,
		FaceID:          item.ID,
		SimilarityScore: 1.0,
		Confidence:      1.0,
		IsManual:        boolPtr(false),
	}); err != nil {
		return nil, fmt.Errorf("create initial face cluster member: %w", err)
	}

	if err := s.refreshClusterRepresentativeWithQueries(ctx, q, cluster.ClusterID); err != nil {
		return nil, err
	}

	refreshed, err := q.GetFaceClusterByID(ctx, cluster.ClusterID)
	if err != nil {
		return nil, fmt.Errorf("reload face cluster: %w", err)
	}
	return &refreshed, nil
}

func (s *faceService) refreshClusterRepresentative(ctx context.Context, clusterID int32) error {
	return s.refreshClusterRepresentativeWithQueries(ctx, s.queries, clusterID)
}

func (s *faceService) refreshClusterRepresentativeWithQueries(ctx context.Context, q *repo.Queries, clusterID int32) error {
	members, err := q.GetFaceClusterMembers(ctx, clusterID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("failed to load cluster members: %w", err)
	}

	if len(members) == 0 {
		if err := q.DeleteFaceCluster(ctx, clusterID); err != nil {
			return fmt.Errorf("failed to delete empty face cluster %d: %w", clusterID, err)
		}
		return nil
	}

	sort.SliceStable(members, func(i, j int) bool {
		leftPrimary := members[i].IsPrimary != nil && *members[i].IsPrimary
		rightPrimary := members[j].IsPrimary != nil && *members[j].IsPrimary
		if leftPrimary != rightPrimary {
			return leftPrimary
		}
		if members[i].Confidence != members[j].Confidence {
			return members[i].Confidence > members[j].Confidence
		}

		leftSize := int32(0)
		if members[i].FaceSize != nil {
			leftSize = *members[i].FaceSize
		}
		rightSize := int32(0)
		if members[j].FaceSize != nil {
			rightSize = *members[j].FaceSize
		}
		if leftSize != rightSize {
			return leftSize > rightSize
		}

		return members[i].ID < members[j].ID
	})

	representativeID := members[0].ID
	confidenceScore := members[0].Confidence
	if _, err := q.UpdateFaceClusterRepresentative(ctx, repo.UpdateFaceClusterRepresentativeParams{
		ClusterID:            clusterID,
		RepresentativeFaceID: &representativeID,
		ConfidenceScore:      &confidenceScore,
	}); err != nil {
		return fmt.Errorf("failed to refresh face cluster representative: %w", err)
	}

	return nil
}

func isClusterCandidate(item repo.FaceItem) bool {
	if item.Embedding == nil || len(item.Embedding.Slice()) == 0 {
		return false
	}
	if item.Confidence < faceRecognitionMinScore {
		return false
	}
	return true
}

func (s *faceService) persistFaceCrop(repoPath string, assetID pgtype.UUID, index int, imageData []byte, bbox *dbtypes.FaceBoundingBox) (*string, error) {
	if bbox == nil {
		return nil, fmt.Errorf("bounding box is required")
	}
	if len(imageData) == 0 {
		return nil, fmt.Errorf("face crop source image is empty")
	}

	img, err := vips.NewImageFromBuffer(imageData)
	if err != nil {
		return nil, fmt.Errorf("decode face crop source: %w", err)
	}
	defer img.Close()

	srcW := img.Width()
	srcH := img.Height()
	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("invalid face crop source image dimensions")
	}

	left, top, width, height := clampFaceCropBounds(bbox, srcW, srcH)
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid face crop bounds")
	}

	if err := img.ExtractArea(left, top, width, height); err != nil {
		return nil, fmt.Errorf("crop face image: %w", err)
	}

	exportParams := vips.NewWebpExportParams()
	exportParams.Quality = faceCropQuality
	exportParams.StripMetadata = true
	cropBytes, _, err := img.ExportWebp(exportParams)
	if err != nil {
		return nil, fmt.Errorf("encode face crop: %w", err)
	}

	filename := fmt.Sprintf("%s_%d.webp", pgUUIDToString(assetID), index)
	relativePath := filepath.Join(storage.DefaultStructure.FacesDir, filename)
	fullPath, err := resolveRepositoryFile(repoPath, relativePath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, fmt.Errorf("create face crop directory: %w", err)
	}
	if err := os.WriteFile(fullPath, cropBytes, 0644); err != nil {
		return nil, fmt.Errorf("write face crop: %w", err)
	}

	normalized := filepath.ToSlash(relativePath)
	return &normalized, nil
}

func clampFaceCropBounds(bbox *dbtypes.FaceBoundingBox, imageWidth, imageHeight int) (left int, top int, width int, height int) {
	faceWidth := bbox.GetWidth()
	faceHeight := bbox.GetHeight()
	paddingX := faceWidth * faceCropPaddingMultiplier
	paddingY := faceHeight * faceCropPaddingMultiplier

	x1 := math.Max(0, float64(bbox.X1-paddingX))
	y1 := math.Max(0, float64(bbox.Y1-paddingY))
	x2 := math.Min(float64(imageWidth), float64(bbox.X2+paddingX))
	y2 := math.Min(float64(imageHeight), float64(bbox.Y2+paddingY))

	left = int(math.Floor(x1))
	top = int(math.Floor(y1))
	right := int(math.Ceil(x2))
	bottom := int(math.Ceil(y2))
	width = right - left
	height = bottom - top
	return left, top, width, height
}

func resolveRepositoryFile(repoPath, relativeOrAbsolutePath string) (string, error) {
	if strings.TrimSpace(relativeOrAbsolutePath) == "" {
		return "", fmt.Errorf("repository file path is empty")
	}

	if filepath.IsAbs(relativeOrAbsolutePath) {
		clean := filepath.Clean(relativeOrAbsolutePath)
		rel, err := filepath.Rel(repoPath, clean)
		if err != nil {
			return "", fmt.Errorf("failed to resolve repository file path: %w", err)
		}
		if strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("path %q is outside repository root", relativeOrAbsolutePath)
		}
		return clean, nil
	}

	cleanRel := filepath.Clean(relativeOrAbsolutePath)
	if strings.HasPrefix(cleanRel, "..") {
		return "", fmt.Errorf("path %q escapes repository root", relativeOrAbsolutePath)
	}

	return filepath.Join(repoPath, cleanRel), nil
}

func removeRepositoryFile(repoPath, relativeOrAbsolutePath string) error {
	fullPath, err := resolveRepositoryFile(repoPath, relativeOrAbsolutePath)
	if err != nil {
		return err
	}
	if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// GetFaceResults gets face detection results for specified asset.
func (s *faceService) GetFaceResults(ctx context.Context, assetID pgtype.UUID) (*FaceResultWithItems, error) {
	result, err := s.queries.GetFaceResultByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get face result: %w", err)
	}

	items, err := s.queries.GetFaceItemsByAsset(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get face items: %w", err)
	}

	return &FaceResultWithItems{
		Result: &result,
		Items:  items,
	}, nil
}

// DeleteFaceResults deletes face results for specified asset, including crops and cluster memberships.
func (s *faceService) DeleteFaceResults(ctx context.Context, assetID pgtype.UUID) error {
	repoPath, _, err := s.resolveAssetRepository(ctx, assetID)
	if err != nil {
		return err
	}

	affectedClusters, err := s.cleanupExistingFaceState(ctx, assetID, repoPath)
	if err != nil {
		return err
	}

	return s.cleanupAffectedClusters(ctx, affectedClusters)
}

// GetFaceStats gets face detection statistics.
func (s *faceService) GetFaceStats(ctx context.Context) ([]dbtypes.FaceStats, error) {
	stats, err := s.queries.GetFaceStatsByModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get face stats: %w", err)
	}

	result := make([]dbtypes.FaceStats, len(stats))
	for i, stat := range stats {
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

// ConvertToJSONMetadata converts face results to JSON metadata format.
func (s *faceService) ConvertToJSONMetadata(ctx context.Context, assetID pgtype.UUID) (*dbtypes.FaceResultMeta, error) {
	result, err := s.queries.GetFaceResultByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}

	items, err := s.queries.GetFaceItemsByAssetWithLimit(ctx, repo.GetFaceItemsByAssetWithLimitParams{
		AssetID: assetID,
		Limit:   1,
	})
	if err != nil {
		return nil, err
	}

	var processingTime int
	if result.ProcessingTimeMs != nil {
		processingTime = int(*result.ProcessingTimeMs)
	}

	return &dbtypes.FaceResultMeta{
		HasFaces:       true,
		TotalFaces:     int(result.TotalFaces),
		HasPrimaryFace: len(items) > 0,
		ProcessingTime: processingTime,
		GeneratedAt:    result.CreatedAt.Time,
		ModelID:        result.ModelID,
	}, nil
}

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
	var name *string
	if trimmed := strings.TrimSpace(clusterName); trimmed != "" {
		name = &trimmed
	}
	confidence := float32(0.0)
	isConfirmed := false
	cluster, err := s.queries.CreateFaceCluster(ctx, repo.CreateFaceClusterParams{
		ClusterName:          name,
		RepresentativeFaceID: &representativeFaceID,
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
	pgVector := pgvector.NewVector(embeddingVector)

	rows, err := s.queries.GetSimilarFaces(ctx, repo.GetSimilarFacesParams{
		EmbeddingQuery: &pgVector,
		ID:             faceID,
		MinSimilarity:  float64(minSimilarity),
		Limit:          int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find similar faces: %w", err)
	}

	result := make([]SimilarFace, 0, len(rows))
	for _, row := range rows {
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
			Similarity: float32(row.Similarity),
		})
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

func (s *faceService) ListPeople(ctx context.Context, repositoryID pgtype.UUID, ownerID *int32, includeHidden bool, limit, offset int) ([]Person, int64, error) {
	total, err := s.queries.CountPeopleScoped(ctx, repo.CountPeopleScopedParams{
		RepositoryID:  repositoryID,
		OwnerID:       ownerID,
		IncludeHidden: includeHidden,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count people: %w", err)
	}

	rows, err := s.queries.ListPeopleScoped(ctx, repo.ListPeopleScopedParams{
		RepositoryID:  repositoryID,
		OwnerID:       ownerID,
		IncludeHidden: includeHidden,
		Offset:        int32(offset),
		Limit:         int32(limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list people: %w", err)
	}

	people := make([]Person, 0, len(rows))
	for _, row := range rows {
		people = append(people, personFromListRow(row))
	}

	return people, total, nil
}

func (s *faceService) GetPerson(ctx context.Context, clusterID int32, repositoryID pgtype.UUID, ownerID *int32) (*Person, error) {
	row, err := s.queries.GetPersonByIDScoped(ctx, repo.GetPersonByIDScopedParams{
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
		ClusterID:    clusterID,
	})
	if err != nil {
		return nil, err
	}

	person := personFromDetailRow(row)
	return &person, nil
}

func (s *faceService) RenamePerson(ctx context.Context, clusterID int32, name string) (*repo.FaceCluster, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, fmt.Errorf("person name cannot be empty")
	}

	cluster, err := s.queries.RenameFaceCluster(ctx, repo.RenameFaceClusterParams{
		ClusterID:   clusterID,
		ClusterName: &trimmedName,
	})
	if err != nil {
		return nil, fmt.Errorf("rename face cluster: %w", err)
	}

	return &cluster, nil
}

// authorizePerson ensures the cluster has at least one face inside the given
// repository/owner scope. It returns pgx.ErrNoRows when the person is not
// visible to the caller, which handlers translate into a 404.
func (s *faceService) authorizePerson(ctx context.Context, clusterID int32, repositoryID pgtype.UUID, ownerID *int32) error {
	_, err := s.queries.GetPersonByIDScoped(ctx, repo.GetPersonByIDScopedParams{
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
		ClusterID:    clusterID,
	})
	return err
}

// ListPersonFaces returns UI-safe face crops for a person, scoped to the caller.
func (s *faceService) ListPersonFaces(ctx context.Context, clusterID int32, repositoryID pgtype.UUID, ownerID *int32, limit, offset int) ([]PersonFace, int64, error) {
	if err := s.authorizePerson(ctx, clusterID, repositoryID, ownerID); err != nil {
		return nil, 0, err
	}

	var representativeFaceID int32
	if cluster, err := s.queries.GetFaceClusterByID(ctx, clusterID); err == nil && cluster.RepresentativeFaceID != nil {
		representativeFaceID = *cluster.RepresentativeFaceID
	}

	total, err := s.queries.CountPersonFacesScoped(ctx, repo.CountPersonFacesScopedParams{
		ClusterID:    clusterID,
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count person faces: %w", err)
	}

	rows, err := s.queries.ListPersonFacesScoped(ctx, repo.ListPersonFacesScopedParams{
		ClusterID:    clusterID,
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
		Offset:       int32(offset),
		Limit:        int32(limit),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list person faces: %w", err)
	}

	faces := make([]PersonFace, 0, len(rows))
	for _, row := range rows {
		faces = append(faces, PersonFace{
			FaceID:           row.ID,
			AssetID:          pgUUIDToString(row.AssetID),
			Confidence:       row.Confidence,
			IsRepresentative: row.ID == representativeFaceID,
			IsManual:         row.IsManual,
			HasCrop:          row.FaceImagePath != nil && strings.TrimSpace(*row.FaceImagePath) != "",
			Filename:         row.OriginalFilename,
			TakenTime:        optionalTimestamp(row.TakenTime),
			UploadTime:       optionalTimestamp(row.UploadTime),
		})
	}

	return faces, total, nil
}

// GetPersonFaceCrop locates a single face crop on disk so handlers can serve it.
func (s *faceService) GetPersonFaceCrop(ctx context.Context, clusterID, faceID int32, repositoryID pgtype.UUID, ownerID *int32) (*PersonFaceCrop, error) {
	row, err := s.queries.GetPersonFaceScoped(ctx, repo.GetPersonFaceScopedParams{
		ClusterID:    clusterID,
		FaceID:       faceID,
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	})
	if err != nil {
		return nil, err
	}
	if row.FaceImagePath == nil || strings.TrimSpace(*row.FaceImagePath) == "" {
		return nil, pgx.ErrNoRows
	}
	if !row.RepositoryID.Valid {
		return nil, fmt.Errorf("face %d has no repository", faceID)
	}

	return &PersonFaceCrop{
		FaceID:        row.ID,
		RepositoryID:  pgUUIDToString(row.RepositoryID),
		FaceImagePath: *row.FaceImagePath,
	}, nil
}

// MergePeople folds every source person into the target person. All source
// memberships become manual on the target so they survive incremental
// recognition, and empty source clusters are deleted. The target name,
// confirmation and hidden state are preserved.
func (s *faceService) MergePeople(ctx context.Context, targetClusterID int32, sourceClusterIDs []int32, repositoryID pgtype.UUID, ownerID *int32) error {
	if err := s.authorizePerson(ctx, targetClusterID, repositoryID, ownerID); err != nil {
		return err
	}

	sources := make([]int32, 0, len(sourceClusterIDs))
	seen := map[int32]struct{}{targetClusterID: {}}
	for _, id := range sourceClusterIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if err := s.authorizePerson(ctx, id, repositoryID, ownerID); err != nil {
			return err
		}
		sources = append(sources, id)
	}
	if len(sources) == 0 {
		return fmt.Errorf("merge requires at least one source person")
	}

	return s.withTx(ctx, func(q *repo.Queries) error {
		for _, sourceID := range sources {
			if err := q.MoveClusterMembersToClusterManual(ctx, repo.MoveClusterMembersToClusterManualParams{
				TargetClusterID: targetClusterID,
				SourceClusterID: sourceID,
			}); err != nil {
				return fmt.Errorf("merge source cluster %d: %w", sourceID, err)
			}
			if err := q.DeleteFaceCluster(ctx, sourceID); err != nil {
				return fmt.Errorf("delete merged source cluster %d: %w", sourceID, err)
			}
		}
		return s.refreshClusterRepresentativeWithQueries(ctx, q, targetClusterID)
	})
}

// MoveFace reassigns a single face to another person as a manual correction.
func (s *faceService) MoveFace(ctx context.Context, faceID, targetClusterID int32, repositoryID pgtype.UUID, ownerID *int32) error {
	if err := s.authorizePerson(ctx, targetClusterID, repositoryID, ownerID); err != nil {
		return err
	}

	if _, err := s.queries.GetFaceForCorrectionScoped(ctx, repo.GetFaceForCorrectionScopedParams{
		FaceID:       faceID,
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	}); err != nil {
		return err
	}

	var sourceClusterID int32
	if source, err := s.queries.GetFaceClusterByFaceID(ctx, faceID); err == nil {
		sourceClusterID = source.ClusterID
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load current cluster for face %d: %w", faceID, err)
	}
	if sourceClusterID == targetClusterID {
		return nil
	}

	return s.withTx(ctx, func(q *repo.Queries) error {
		if _, err := q.AssignFaceClusterMemberExclusive(ctx, repo.AssignFaceClusterMemberExclusiveParams{
			ClusterID:       targetClusterID,
			FaceID:          faceID,
			SimilarityScore: 1.0,
			Confidence:      1.0,
			IsManual:        boolPtr(true),
		}); err != nil {
			return fmt.Errorf("assign moved face: %w", err)
		}
		if sourceClusterID != 0 {
			if err := s.refreshClusterRepresentativeWithQueries(ctx, q, sourceClusterID); err != nil {
				return err
			}
		}
		return s.refreshClusterRepresentativeWithQueries(ctx, q, targetClusterID)
	})
}

// RemoveFaceFromPerson detaches a face from a person, leaving it unclustered so
// it can be recovered by a later rebuild or correction. The asset is unchanged.
func (s *faceService) RemoveFaceFromPerson(ctx context.Context, faceID, clusterID int32, repositoryID pgtype.UUID, ownerID *int32) error {
	if err := s.authorizePerson(ctx, clusterID, repositoryID, ownerID); err != nil {
		return err
	}
	if _, err := s.queries.GetPersonFaceScoped(ctx, repo.GetPersonFaceScopedParams{
		ClusterID:    clusterID,
		FaceID:       faceID,
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	}); err != nil {
		return err
	}

	return s.withTx(ctx, func(q *repo.Queries) error {
		if err := q.DeleteFaceClusterMember(ctx, repo.DeleteFaceClusterMemberParams{
			ClusterID: clusterID,
			FaceID:    faceID,
		}); err != nil {
			return fmt.Errorf("remove face from cluster: %w", err)
		}
		return s.refreshClusterRepresentativeWithQueries(ctx, q, clusterID)
	})
}

// SetPersonCover marks a face as the representative cover. The face must belong
// to the person.
func (s *faceService) SetPersonCover(ctx context.Context, clusterID, faceID int32, repositoryID pgtype.UUID, ownerID *int32) error {
	if err := s.authorizePerson(ctx, clusterID, repositoryID, ownerID); err != nil {
		return err
	}
	face, err := s.queries.GetPersonFaceScoped(ctx, repo.GetPersonFaceScopedParams{
		ClusterID:    clusterID,
		FaceID:       faceID,
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	})
	if err != nil {
		return err
	}

	confidence := face.Confidence
	if _, err := s.queries.UpdateFaceClusterRepresentative(ctx, repo.UpdateFaceClusterRepresentativeParams{
		ClusterID:            clusterID,
		RepresentativeFaceID: &face.ID,
		ConfidenceScore:      &confidence,
	}); err != nil {
		return fmt.Errorf("set person cover: %w", err)
	}
	return nil
}

// SetPersonHidden hides or unhides a person from default people views without
// touching face assignments, names or assets.
func (s *faceService) SetPersonHidden(ctx context.Context, clusterID int32, hidden bool, repositoryID pgtype.UUID, ownerID *int32) (*Person, error) {
	if err := s.authorizePerson(ctx, clusterID, repositoryID, ownerID); err != nil {
		return nil, err
	}
	if _, err := s.queries.SetFaceClusterHidden(ctx, repo.SetFaceClusterHiddenParams{
		ClusterID: clusterID,
		IsHidden:  hidden,
	}); err != nil {
		return nil, fmt.Errorf("set person hidden: %w", err)
	}
	return s.GetPerson(ctx, clusterID, repositoryID, ownerID)
}

func personFromListRow(row repo.ListPeopleScopedRow) Person {
	return Person{
		PersonID:              row.ClusterID,
		Name:                  normalizedName(row.ClusterName),
		IsConfirmed:           row.IsConfirmed != nil && *row.IsConfirmed,
		IsHidden:              row.IsHidden,
		HiddenAt:              optionalTimestamp(row.HiddenAt),
		MemberCount:           row.MemberCount,
		AssetCount:            row.AssetCount,
		CoverFaceImagePath:    row.CoverFaceImagePath,
		RepresentativeAssetID: optionalUUIDToString(row.RepresentativeAssetID),
		CreatedAt:             row.CreatedAt.Time,
		UpdatedAt:             row.UpdatedAt.Time,
	}
}

func personFromDetailRow(row repo.GetPersonByIDScopedRow) Person {
	return Person{
		PersonID:              row.ClusterID,
		Name:                  normalizedName(row.ClusterName),
		IsConfirmed:           row.IsConfirmed != nil && *row.IsConfirmed,
		IsHidden:              row.IsHidden,
		HiddenAt:              optionalTimestamp(row.HiddenAt),
		MemberCount:           row.MemberCount,
		AssetCount:            row.AssetCount,
		CoverFaceImagePath:    row.CoverFaceImagePath,
		RepresentativeAssetID: optionalUUIDToString(row.RepresentativeAssetID),
		CreatedAt:             row.CreatedAt.Time,
		UpdatedAt:             row.UpdatedAt.Time,
	}
}

func optionalTimestamp(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}

func normalizedName(name *string) *string {
	if name == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*name)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalUUIDToString(value pgtype.UUID) *string {
	if !value.Valid {
		return nil
	}
	id := uuid.UUID(value.Bytes).String()
	return &id
}

func pgUUIDToString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}

func boolPtr(value bool) *bool {
	return &value
}
