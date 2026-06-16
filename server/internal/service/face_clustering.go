package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

type faceClusterScope struct {
	RepositoryID   pgtype.UUID
	OwnerID        *int32
	EmbeddingModel *string
}

type pendingFaceRecognitionResult int

const (
	pendingFaceRecognitionSkipped pendingFaceRecognitionResult = iota
	pendingFaceRecognitionDeferred
	pendingFaceRecognitionAssigned
)

type pendingFaceRecognitionDecision int

const (
	pendingFaceRecognitionDecisionSkip pendingFaceRecognitionDecision = iota
	pendingFaceRecognitionDecisionDefer
	pendingFaceRecognitionDecisionAssignExisting
	pendingFaceRecognitionDecisionCreateCluster
)

func (s *faceService) recognizePendingFacesForAsset(ctx context.Context, asset repo.Asset, items []repo.FaceItem) error {
	if !asset.RepositoryID.Valid {
		return nil
	}
	for _, scope := range collectPendingFaceRecognitionScopes(asset, items) {
		if err := s.recognizePendingFaces(ctx, scope); err != nil {
			return err
		}
	}
	return nil
}

func collectPendingFaceRecognitionScopes(asset repo.Asset, items []repo.FaceItem) []faceClusterScope {
	if !asset.RepositoryID.Valid {
		return nil
	}

	scopes := make([]faceClusterScope, 0)
	seen := make(map[string]struct{})
	for _, item := range items {
		if !isClusterCandidate(item) {
			continue
		}
		scope := faceClusterScope{
			RepositoryID:   asset.RepositoryID,
			OwnerID:        cloneInt32Ptr(asset.OwnerID),
			EmbeddingModel: normalizedName(item.EmbeddingModel),
		}
		key := faceClusterScopeKey(scope)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		scopes = append(scopes, scope)
	}
	sort.Slice(scopes, func(i, j int) bool {
		return faceClusterScopeKey(scopes[i]) < faceClusterScopeKey(scopes[j])
	})
	return scopes
}

func (s *faceService) recognizePendingFaces(ctx context.Context, scope faceClusterScope) error {
	return s.withTx(ctx, func(q *repo.Queries) error {
		return s.recognizePendingFacesWithQueries(ctx, q, scope)
	})
}

func (s *faceService) recognizePendingFacesWithQueries(ctx context.Context, q *repo.Queries, scope faceClusterScope) error {
	minFaceSize := int32(0)
	pending, err := q.GetUnclusteredFacesInScope(ctx, repo.GetUnclusteredFacesInScopeParams{
		RepositoryID:   scope.RepositoryID,
		OwnerID:        scope.OwnerID,
		EmbeddingModel: scope.EmbeddingModel,
		MinConfidence:  faceRecognitionMinScore,
		MinFaceSize:    &minFaceSize,
	})
	if err != nil {
		return fmt.Errorf("load pending face recognition candidates: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	deferred := make([]repo.FaceItem, 0, len(pending))
	coreCache := make(map[int32]bool, len(pending))
	for _, item := range pending {
		result, err := s.recognizePendingFace(ctx, q, item, scope, false, coreCache)
		if err != nil {
			return err
		}
		if result == pendingFaceRecognitionDeferred {
			deferred = append(deferred, item)
		}
	}

	for _, item := range deferred {
		if _, err := s.recognizePendingFace(ctx, q, item, scope, true, coreCache); err != nil {
			return err
		}
	}

	if err := q.DeleteEmptyUnconfirmedFaceClusters(ctx); err != nil {
		return fmt.Errorf("delete empty face clusters: %w", err)
	}
	return nil
}

func (s *faceService) recognizePendingFace(ctx context.Context, q *repo.Queries, item repo.FaceItem, scope faceClusterScope, deferred bool, coreCache map[int32]bool) (pendingFaceRecognitionResult, error) {
	isCore, err := s.isCoreFaceDBSCAN(ctx, q, item, scope, coreCache)
	if err != nil {
		return pendingFaceRecognitionSkipped, err
	}
	clusterID, similarity, err := s.findNearestAssignedFaceCluster(ctx, q, item, scope)
	if err != nil {
		return pendingFaceRecognitionSkipped, err
	}

	switch decidePendingFaceRecognition(isCore, deferred, clusterID > 0) {
	case pendingFaceRecognitionDecisionDefer:
		return pendingFaceRecognitionDeferred, nil
	case pendingFaceRecognitionDecisionAssignExisting:
		if _, err := q.AssignFaceClusterMemberExclusive(ctx, repo.AssignFaceClusterMemberExclusiveParams{
			ClusterID:       clusterID,
			FaceID:          item.ID,
			SimilarityScore: similarity,
			Confidence:      similarity,
			IsManual:        boolPtr(false),
		}); err != nil {
			return pendingFaceRecognitionSkipped, fmt.Errorf("assign face cluster member: %w", err)
		}
		if err := s.refreshClusterRepresentativeWithQueries(ctx, q, clusterID); err != nil {
			return pendingFaceRecognitionSkipped, err
		}
		return pendingFaceRecognitionAssigned, nil
	case pendingFaceRecognitionDecisionCreateCluster:
		if _, err := s.createClusterForFaceWithQueries(ctx, q, item, nil, false); err != nil {
			return pendingFaceRecognitionSkipped, err
		}
		return pendingFaceRecognitionAssigned, nil
	default:
		return pendingFaceRecognitionSkipped, nil
	}
}

func (s *faceService) findNearestAssignedFaceCluster(ctx context.Context, q *repo.Queries, item repo.FaceItem, scope faceClusterScope) (int32, float32, error) {
	if item.Embedding == nil || len(item.Embedding.Slice()) == 0 {
		return 0, 0, nil
	}

	queryVector := pgvector.NewVector(item.Embedding.Slice())
	minFaceSize := int32(0)
	row, err := q.GetNearestAssignedFaceCluster(ctx, repo.GetNearestAssignedFaceClusterParams{
		EmbeddingQuery: &queryVector,
		ID:             item.ID,
		RepositoryID:   scope.RepositoryID,
		OwnerID:        scope.OwnerID,
		EmbeddingModel: scope.EmbeddingModel,
		MinConfidence:  faceRecognitionMinScore,
		MinFaceSize:    &minFaceSize,
		MinSimilarity:  1 - faceRecognitionMaxDistance,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("find nearest assigned face cluster: %w", err)
	}
	return row.ClusterID, clampSimilarity32(float32(row.Similarity)), nil
}

func faceClusterScopeKey(scope faceClusterScope) string {
	parts := []string{pgUUIDToString(scope.RepositoryID), "owner", "nil", "model", "nil"}
	if scope.OwnerID != nil {
		parts[2] = fmt.Sprintf("%d", *scope.OwnerID)
	}
	if scope.EmbeddingModel != nil {
		parts[4] = strings.TrimSpace(*scope.EmbeddingModel)
	}
	return strings.Join(parts, ":")
}

func decidePendingFaceRecognition(isCore, deferred, hasAssignedCluster bool) pendingFaceRecognitionDecision {
	if !isCore && !deferred {
		return pendingFaceRecognitionDecisionDefer
	}
	if hasAssignedCluster {
		return pendingFaceRecognitionDecisionAssignExisting
	}
	if isCore {
		return pendingFaceRecognitionDecisionCreateCluster
	}
	return pendingFaceRecognitionDecisionSkip
}

func (s *faceService) isCoreFaceDBSCAN(ctx context.Context, q *repo.Queries, item repo.FaceItem, scope faceClusterScope, cache map[int32]bool) (bool, error) {
	if cached, ok := cache[item.ID]; ok {
		return cached, nil
	}
	if item.Embedding == nil || len(item.Embedding.Slice()) == 0 {
		cache[item.ID] = false
		return false, nil
	}

	queryVector := pgvector.NewVector(item.Embedding.Slice())
	minFaceSize := int32(0)
	count, err := q.CountIncrementalFaceNeighbors(ctx, repo.CountIncrementalFaceNeighborsParams{
		ID:             item.ID,
		RepositoryID:   scope.RepositoryID,
		OwnerID:        scope.OwnerID,
		EmbeddingModel: scope.EmbeddingModel,
		MinConfidence:  faceRecognitionMinScore,
		MinFaceSize:    &minFaceSize,
		EmbeddingQuery: &queryVector,
		MinSimilarity:  1 - faceRecognitionMaxDistance,
	})
	if err != nil {
		return false, fmt.Errorf("count dbscan face neighbors: %w", err)
	}

	isCore := int(count)+1 >= faceRecognitionMinFaces
	cache[item.ID] = isCore
	return isCore, nil
}

func collectFaceClusteringScopes(rows []repo.GetFaceClusteringCandidatesRow) []faceClusterScope {
	scopes := make([]faceClusterScope, 0)
	seen := make(map[string]struct{})
	for _, row := range rows {
		scope := faceClusterScope{
			RepositoryID:   row.RepositoryID,
			OwnerID:        cloneInt32Ptr(row.OwnerID),
			EmbeddingModel: normalizedName(row.EmbeddingModel),
		}
		key := faceClusterScopeKey(scope)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		scopes = append(scopes, scope)
	}
	sort.Slice(scopes, func(i, j int) bool {
		return faceClusterScopeKey(scopes[i]) < faceClusterScopeKey(scopes[j])
	})
	return scopes
}

func (s *faceService) RebuildFaceClusters(ctx context.Context, repositoryID pgtype.UUID, ownerID *int32) (FaceClusterRebuildResult, error) {
	startedAt := time.Now()
	result := FaceClusterRebuildResult{
		Algorithm:    "immich-dbscan-sequential-v1",
		RepositoryID: optionalUUIDToString(repositoryID),
	}

	minFaceSize := int32(0)
	candidateRows, err := s.queries.GetFaceClusteringCandidates(ctx, repo.GetFaceClusteringCandidatesParams{
		RepositoryID:  repositoryID,
		OwnerID:       ownerID,
		MinConfidence: faceRecognitionMinScore,
		MinFaceSize:   &minFaceSize,
	})
	if err != nil {
		return result, fmt.Errorf("load face clustering candidates: %w", err)
	}
	result.CandidateFaces = len(candidateRows)
	scopes := collectFaceClusteringScopes(candidateRows)

	if err := s.withTx(ctx, func(q *repo.Queries) error {
		if err := q.DeleteFaceClusterMembersForScope(ctx, repo.DeleteFaceClusterMembersForScopeParams{
			RepositoryID: repositoryID,
			OwnerID:      ownerID,
		}); err != nil {
			return fmt.Errorf("delete old face cluster memberships: %w", err)
		}

		if err := q.DeleteEmptyFaceClusters(ctx); err != nil {
			return fmt.Errorf("delete empty face clusters after unassign: %w", err)
		}

		for _, scope := range scopes {
			if err := s.recognizePendingFacesWithQueries(ctx, q, scope); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return result, err
	}

	assignments, err := s.queries.GetFaceClusterAssignmentsForScope(ctx, repo.GetFaceClusterAssignmentsForScopeParams{
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	})
	if err != nil {
		return result, fmt.Errorf("load rebuilt face cluster assignments: %w", err)
	}
	result.ClusteredFaces = len(assignments)
	result.NoiseFaces = result.CandidateFaces - result.ClusteredFaces

	clusterCount, err := s.queries.CountPeopleScoped(ctx, repo.CountPeopleScopedParams{
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	})
	if err != nil {
		return result, fmt.Errorf("count rebuilt face clusters: %w", err)
	}
	result.ClustersTotal = int(clusterCount)
	result.ClustersCreated = result.ClustersTotal
	result.ClustersReused = 0
	result.DurationMs = time.Since(startedAt).Milliseconds()
	return result, nil
}

func cosineSimilarity(left, right []float32) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return 0
	}
	dot := 0.0
	leftNorm := 0.0
	rightNorm := 0.0
	for i := range left {
		l := float64(left[i])
		r := float64(right[i])
		dot += l * r
		leftNorm += l * l
		rightNorm += r * r
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	similarity := dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
	return math.Max(0, math.Min(1, similarity))
}

func clampSimilarity32(value float32) float32 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func cloneInt32Ptr(value *int32) *int32 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
