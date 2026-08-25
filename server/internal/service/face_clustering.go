package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
)

// faceClusterScope is the cluster identity partition: clusters span
// repositories (the product intent — one person across all storage
// locations) but never owners or embedding models.
type faceClusterScope struct {
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
	occurrence, err := s.queries.GetPreferredActiveAssetOccurrence(ctx, asset.AssetID)
	if err != nil {
		return fmt.Errorf("resolve active asset occurrence for face recognition: %w", err)
	}
	for _, scope := range collectPendingFaceRecognitionScopes(asset, items) {
		// The asset's repository bounds which pending faces get processed on
		// this save; cluster matching inside is owner-wide across repos.
		if err := s.recognizePendingFaces(ctx, scope, uuid.NullUUID{UUID: occurrence.RepositoryID, Valid: true}); err != nil {
			return err
		}
	}
	return nil
}

func collectPendingFaceRecognitionScopes(asset repo.Asset, items []repo.FaceItem) []faceClusterScope {
	scopes := make([]faceClusterScope, 0)
	seen := make(map[string]struct{})
	for _, item := range items {
		if !isClusterCandidate(item) {
			continue
		}
		scope := faceClusterScope{
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

func (s *faceService) recognizePendingFaces(ctx context.Context, scope faceClusterScope, selectRepositoryID uuid.NullUUID) error {
	return s.withTx(ctx, func(q *repo.Queries) error {
		return s.recognizePendingFacesWithQueries(ctx, q, scope, selectRepositoryID)
	})
}

// recognizePendingFacesWithQueries assigns unclustered faces to clusters.
// selectRepositoryID only bounds which pending faces are picked up (invalid
// UUID = all repositories); it never partitions cluster identity.
func (s *faceService) recognizePendingFacesWithQueries(ctx context.Context, q *repo.Queries, scope faceClusterScope, selectRepositoryID uuid.NullUUID) error {
	minFaceSize := int64(0)
	pending, err := q.GetUnclusteredFacesInScope(ctx, repo.GetUnclusteredFacesInScopeParams{
		RepositoryID:   selectRepositoryID,
		OwnerID:        scope.OwnerID,
		EmbeddingModel: scope.EmbeddingModel,
		MinConfidence:  float64(faceRecognitionMinScore),
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
			SimilarityScore: float64(similarity),
			Confidence:      float64(similarity),
			IsManual:        false,
		}); err != nil {
			return pendingFaceRecognitionSkipped, fmt.Errorf("assign face cluster member: %w", err)
		}
		if err := s.refreshClusterRepresentativeWithQueries(ctx, q, clusterID); err != nil {
			return pendingFaceRecognitionSkipped, err
		}
		return pendingFaceRecognitionAssigned, nil
	case pendingFaceRecognitionDecisionCreateCluster:
		if _, err := s.createClusterForFaceWithQueries(ctx, q, item, scope.OwnerID, nil, false); err != nil {
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

	queryVector := dbtypes.NewVector(item.Embedding.Slice())
	minFaceSize := int64(0)
	row, err := q.GetNearestAssignedFaceCluster(ctx, repo.GetNearestAssignedFaceClusterParams{
		EmbeddingQuery: queryVector,
		ID:             item.ID,
		OwnerID:        scope.OwnerID,
		EmbeddingModel: scope.EmbeddingModel,
		MinConfidence:  float64(faceRecognitionMinScore),
		MinFaceSize:    &minFaceSize,
		MinSimilarity:  1 - faceRecognitionMaxDistance,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("find nearest assigned face cluster: %w", err)
	}
	return row.ClusterID, clampSimilarity32(float32(row.Similarity)), nil
}

func faceClusterScopeKey(scope faceClusterScope) string {
	parts := []string{"owner", "nil", "model", "nil"}
	if scope.OwnerID != nil {
		parts[1] = fmt.Sprintf("%d", *scope.OwnerID)
	}
	if scope.EmbeddingModel != nil {
		parts[3] = strings.TrimSpace(*scope.EmbeddingModel)
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

	queryVector := dbtypes.NewVector(item.Embedding.Slice())
	minFaceSize := int64(0)
	count, err := q.CountIncrementalFaceNeighbors(ctx, repo.CountIncrementalFaceNeighborsParams{
		ID:             item.ID,
		OwnerID:        scope.OwnerID,
		EmbeddingModel: scope.EmbeddingModel,
		MinConfidence:  float64(faceRecognitionMinScore),
		MinFaceSize:    &minFaceSize,
		EmbeddingQuery: queryVector,
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

type manualFaceMembershipPayload struct {
	FaceID          int32   `json:"face_id"`
	ClusterID       int32   `json:"cluster_id"`
	SimilarityScore float64 `json:"similarity_score"`
	Confidence      float64 `json:"confidence"`
}

// resetFaceClusterScope replaces only the derived membership slice and
// restores user-authored corrections in one set statement. The expensive
// vector work deliberately happens after this short transaction has released
// SQLite's sole writer.
func (s *faceService) resetFaceClusterScope(
	ctx context.Context,
	repositoryID uuid.NullUUID,
	ownerID *int32,
	manual []repo.GetManualFaceClusterMembershipsForScopeRow,
) error {
	payload := make([]manualFaceMembershipPayload, 0, len(manual))
	for _, membership := range manual {
		payload = append(payload, manualFaceMembershipPayload{
			FaceID:          membership.FaceID,
			ClusterID:       membership.ClusterID,
			SimilarityScore: membership.SimilarityScore,
			Confidence:      membership.Confidence,
		})
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode manual face memberships: %w", err)
	}

	tx, err := s.writer.BeginTx(ctx, catalogtx.OperationFaceClusterResetScope, nil)
	if err != nil {
		return fmt.Errorf("begin face rebuild reset: %w", err)
	}
	defer tx.Rollback()
	q := s.queries.WithTx(tx.Raw())
	if err := q.DeleteFaceClusterMembersForScope(ctx, repo.DeleteFaceClusterMembersForScopeParams{
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	}); err != nil {
		return fmt.Errorf("delete old face cluster memberships: %w", err)
	}
	if len(payload) > 0 {
		inserted, err := tx.ExecContext(ctx, `
INSERT INTO face_cluster_members (
  cluster_id, face_id, similarity_score, confidence, is_manual, created_at
)
SELECT
  CAST(json_extract(value, '$.cluster_id') AS INTEGER),
  CAST(json_extract(value, '$.face_id') AS INTEGER),
  CAST(json_extract(value, '$.similarity_score') AS REAL),
  CAST(json_extract(value, '$.confidence') AS REAL),
  1,
  CAST(unixepoch('subsec') * 1000000 AS INTEGER)
FROM json_each(?)`, encoded)
		if err != nil {
			return fmt.Errorf("restore manual face memberships: %w", err)
		}
		if count, countErr := inserted.RowsAffected(); countErr != nil || count != int64(len(payload)) {
			return fmt.Errorf("restore manual face memberships: inserted %d of %d (rows error: %v)", count, len(payload), countErr)
		}
	}
	if err := q.DeleteEmptyFaceClusters(ctx); err != nil {
		return fmt.Errorf("delete empty face clusters after reset: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit face rebuild reset: %w", err)
	}
	return nil
}

// recognizePendingFacesConvergently performs neighbor/nearest-cluster reads on
// the query-only pool and publishes at most one face per writer transaction.
// The caller holds clusterRebuildMu exclusively, so an artificial rebuild
// cannot overwrite a concurrent user correction between planning and publish.
func (s *faceService) recognizePendingFacesConvergently(
	ctx context.Context,
	scope faceClusterScope,
	selectRepositoryID uuid.NullUUID,
) (int, map[int32]struct{}, error) {
	minFaceSize := int64(0)
	pending, err := s.queries.GetUnclusteredFacesInScope(ctx, repo.GetUnclusteredFacesInScopeParams{
		RepositoryID:   selectRepositoryID,
		OwnerID:        scope.OwnerID,
		EmbeddingModel: scope.EmbeddingModel,
		MinConfidence:  float64(faceRecognitionMinScore),
		MinFaceSize:    &minFaceSize,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("load pending face recognition candidates: %w", err)
	}

	created := 0
	reused := make(map[int32]struct{})
	deferred := make([]repo.FaceItem, 0, len(pending))
	coreCache := make(map[int32]bool, len(pending))
	for _, item := range pending {
		isCore, err := s.isCoreFaceDBSCAN(ctx, s.queries, item, scope, coreCache)
		if err != nil {
			return created, reused, err
		}
		if !isCore {
			deferred = append(deferred, item)
			continue
		}
		clusterID, wasCreated, err := s.publishPendingFace(ctx, item, scope, true)
		if err != nil {
			return created, reused, err
		}
		if wasCreated {
			created++
		} else if clusterID != 0 {
			reused[clusterID] = struct{}{}
		}
	}
	for _, item := range deferred {
		clusterID, _, err := s.publishPendingFace(ctx, item, scope, false)
		if err != nil {
			return created, reused, err
		}
		if clusterID != 0 {
			reused[clusterID] = struct{}{}
		}
	}
	if err := s.queries.DeleteEmptyUnconfirmedFaceClusters(ctx); err != nil {
		return created, reused, fmt.Errorf("delete empty face clusters: %w", err)
	}
	return created, reused, nil
}

func (s *faceService) publishPendingFace(
	ctx context.Context,
	item repo.FaceItem,
	scope faceClusterScope,
	createIfUnmatched bool,
) (clusterID int32, created bool, err error) {
	clusterID, similarity, err := s.findNearestAssignedFaceCluster(ctx, s.queries, item, scope)
	if err != nil {
		return 0, false, err
	}
	if clusterID == 0 && !createIfUnmatched {
		return 0, false, nil
	}

	tx, err := s.writer.BeginTx(ctx, catalogtx.OperationFaceClusterPublishPending, nil)
	if err != nil {
		return 0, false, fmt.Errorf("begin bounded face publish: %w", err)
	}
	defer tx.Rollback()
	q := s.queries.WithTx(tx.Raw())
	if clusterID == 0 {
		cluster, err := q.CreateFaceCluster(ctx, repo.CreateFaceClusterParams{
			OwnerID:              cloneInt32Ptr(scope.OwnerID),
			RepresentativeFaceID: int64Ptr(int64(item.ID)),
			ConfidenceScore:      item.Confidence,
			IsConfirmed:          false,
		})
		if err != nil {
			return 0, false, fmt.Errorf("create face cluster: %w", err)
		}
		clusterID = cluster.ClusterID
		similarity = 1
		created = true
	}
	if _, err := q.AssignFaceClusterMemberExclusive(ctx, repo.AssignFaceClusterMemberExclusiveParams{
		ClusterID:       clusterID,
		FaceID:          item.ID,
		SimilarityScore: float64(similarity),
		Confidence:      float64(similarity),
		IsManual:        false,
	}); err != nil {
		return 0, false, fmt.Errorf("assign face cluster member: %w", err)
	}
	if err := refreshFaceClusterRepresentativeTx(ctx, tx.Raw(), clusterID); err != nil {
		return 0, false, err
	}
	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("commit bounded face publish: %w", err)
	}
	return clusterID, created, nil
}

func refreshFaceClusterRepresentativeTx(ctx context.Context, tx *sql.Tx, clusterID int32) error {
	var faceID int64
	var confidence float64
	err := tx.QueryRowContext(ctx, `
SELECT fi.id, fi.confidence
FROM face_cluster_members AS member
JOIN face_items AS fi ON fi.id = member.face_id
WHERE member.cluster_id = ?
ORDER BY fi.is_primary DESC, fi.confidence DESC,
         COALESCE(fi.face_size, 0) DESC, fi.id ASC
LIMIT 1`, clusterID).Scan(&faceID, &confidence)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := tx.ExecContext(ctx, `DELETE FROM face_clusters WHERE cluster_id = ?`, clusterID); err != nil {
			return fmt.Errorf("delete empty face cluster %d: %w", clusterID, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("select face cluster representative %d: %w", clusterID, err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE face_clusters
SET representative_face_id = ?, confidence_score = ?,
    member_count = (SELECT count(*) FROM face_cluster_members WHERE cluster_id = ?),
    updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
WHERE cluster_id = ?`, faceID, confidence, clusterID, clusterID); err != nil {
		return fmt.Errorf("refresh face cluster representative %d: %w", clusterID, err)
	}
	return nil
}

func (s *faceService) refreshFaceClusterBounded(ctx context.Context, clusterID int32) error {
	tx, err := s.writer.BeginTx(ctx, catalogtx.OperationFaceClusterRefresh, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := refreshFaceClusterRepresentativeTx(ctx, tx.Raw(), clusterID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *faceService) RebuildFaceClusters(ctx context.Context, repositoryID uuid.NullUUID, ownerID *int32) (FaceClusterRebuildResult, error) {
	s.clusterRebuildMu.Lock()
	defer s.clusterRebuildMu.Unlock()

	startedAt := time.Now()
	result := FaceClusterRebuildResult{
		Algorithm:    "immich-dbscan-sequential-v1",
		RepositoryID: optionalUUIDToString(repositoryID),
	}

	minFaceSize := int64(0)
	candidateRows, err := s.queries.GetFaceClusteringCandidates(ctx, repo.GetFaceClusteringCandidatesParams{
		RepositoryID:  repositoryID,
		OwnerID:       ownerID,
		MinConfidence: float64(faceRecognitionMinScore),
		MinFaceSize:   &minFaceSize,
	})
	if err != nil {
		return result, fmt.Errorf("load face clustering candidates: %w", err)
	}
	result.CandidateFaces = len(candidateRows)
	scopes := collectFaceClusteringScopes(candidateRows)

	// Capture manual corrections before unassigning everything so a full
	// rebuild preserves user-authored membership (moves and merges).
	manualMemberships, err := s.queries.GetManualFaceClusterMembershipsForScope(ctx, repo.GetManualFaceClusterMembershipsForScopeParams{
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	})
	if err != nil {
		return result, fmt.Errorf("load manual face memberships: %w", err)
	}
	priorAssignments, err := s.queries.GetFaceClusterAssignmentsForScope(ctx, repo.GetFaceClusterAssignmentsForScopeParams{
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	})
	if err != nil {
		return result, fmt.Errorf("load prior face cluster assignments: %w", err)
	}
	affectedClusters := make(map[int32]struct{}, len(priorAssignments)+len(manualMemberships))
	for _, assignment := range priorAssignments {
		affectedClusters[assignment.ClusterID] = struct{}{}
	}
	for _, membership := range manualMemberships {
		affectedClusters[membership.ClusterID] = struct{}{}
	}

	if err := s.resetFaceClusterScope(ctx, repositoryID, ownerID, manualMemberships); err != nil {
		return result, err
	}
	if s.afterRebuildReset != nil {
		s.afterRebuildReset()
	}
	createdClusters := 0
	reusedClusters := make(map[int32]struct{})
	for _, scope := range scopes {
		created, reused, err := s.recognizePendingFacesConvergently(ctx, scope, repositoryID)
		if err != nil {
			return result, err
		}
		createdClusters += created
		for clusterID := range reused {
			reusedClusters[clusterID] = struct{}{}
			affectedClusters[clusterID] = struct{}{}
		}
	}
	// A cluster that retained only out-of-scope members might receive no new
	// assignment, but its former representative may have belonged to the reset
	// slice. Refresh each affected cluster in an independently yielding writer
	// turn so large repositories never recreate one monolithic transaction.
	for clusterID := range affectedClusters {
		if err := s.refreshFaceClusterBounded(ctx, clusterID); err != nil {
			return result, err
		}
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
		RepositoryID:  repositoryID,
		OwnerID:       ownerID,
		IncludeHidden: true,
	})
	if err != nil {
		return result, fmt.Errorf("count rebuilt face clusters: %w", err)
	}
	result.ClustersTotal = int(clusterCount)
	result.ClustersCreated = createdClusters
	result.ClustersReused = len(reusedClusters)
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
