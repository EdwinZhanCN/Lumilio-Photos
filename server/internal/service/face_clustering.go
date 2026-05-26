package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

type faceClusterScope struct {
	RepositoryID   pgtype.UUID
	OwnerID        *int32
	EmbeddingModel *string
}

type incrementalFaceNeighbor struct {
	item       repo.FaceItem
	similarity float32
}

type faceClusterCandidate struct {
	item         repo.FaceItem
	repositoryID pgtype.UUID
	ownerID      *int32
	vector       []float32
}

type faceClusterPersistPlan struct {
	members              []faceClusterCandidate
	medoidIndex          int
	reusedClusterID      int32
	reuseExistingCluster bool
	name                 *string
	confirmed            bool
}

type oldFaceClusterAssignment struct {
	clusterID   int32
	name        *string
	isConfirmed bool
}

type hdbscanCluster struct {
	indices                 []int
	medoidIndex             int
	averageMedoidSimilarity float64
}

func (s *faceService) assignFaceToClusterDBSCAN(ctx context.Context, q *repo.Queries, item repo.FaceItem, asset repo.Asset) error {
	scope := faceClusterScope{
		RepositoryID:   asset.RepositoryID,
		OwnerID:        cloneInt32Ptr(asset.OwnerID),
		EmbeddingModel: normalizedName(item.EmbeddingModel),
	}
	queryVector := pgvector.NewVector(item.Embedding.Slice())
	minFaceSize := faceClusterMinAreaPixels

	rows, err := q.GetIncrementalFaceNeighbors(ctx, repo.GetIncrementalFaceNeighborsParams{
		EmbeddingQuery: &queryVector,
		ID:             item.ID,
		RepositoryID:   scope.RepositoryID,
		OwnerID:        scope.OwnerID,
		EmbeddingModel: scope.EmbeddingModel,
		MinConfidence:  faceClusterMinConfidence,
		MinFaceSize:    &minFaceSize,
		MinSimilarity:  float64(faceClusterMinSimilarity),
		Limit:          faceClusterIncrementalNeighborLimit,
	})
	if err != nil {
		return fmt.Errorf("load dbscan face neighbors: %w", err)
	}

	neighbors := make([]incrementalFaceNeighbor, 0, len(rows))
	neighborIDs := make([]int32, 0, len(rows))
	for _, row := range rows {
		neighbor := incrementalFaceNeighbor{
			item:       faceItemFromIncrementalNeighbor(row),
			similarity: float32(row.Similarity),
		}
		neighbors = append(neighbors, neighbor)
		neighborIDs = append(neighborIDs, neighbor.item.ID)
	}

	if len(neighbors) == 0 {
		return nil
	}

	membershipByFaceID, err := loadFaceMembershipsByFaceID(ctx, q, neighborIDs)
	if err != nil {
		return err
	}

	isNewCore := len(neighbors)+1 >= faceClusterDBSCANMinPoints
	coreCache := make(map[int32]bool, len(neighbors))
	clusterMatches := make(map[int32]float32)
	unclusteredNeighbors := make([]incrementalFaceNeighbor, 0)

	for _, neighbor := range neighbors {
		membership, hasMembership := membershipByFaceID[neighbor.item.ID]
		if !hasMembership {
			unclusteredNeighbors = append(unclusteredNeighbors, neighbor)
			continue
		}

		isCore, err := s.isCoreFaceDBSCAN(ctx, q, neighbor.item, scope, coreCache)
		if err != nil {
			return err
		}
		if isCore || isNewCore {
			if neighbor.similarity > clusterMatches[membership.ClusterID] {
				clusterMatches[membership.ClusterID] = neighbor.similarity
			}
		}
	}

	if !isNewCore && len(clusterMatches) == 0 {
		return nil
	}

	var targetClusterID int32
	var targetSimilarity float32
	createdTarget := false
	if len(clusterMatches) == 0 {
		cluster, err := s.createClusterForFaceWithQueries(ctx, q, item, nil, false)
		if err != nil {
			return err
		}
		targetClusterID = cluster.ClusterID
		targetSimilarity = 1.0
		createdTarget = true
	} else {
		targetClusterID, targetSimilarity = bestClusterMatch(clusterMatches)
	}

	if !createdTarget {
		if _, err := q.AssignFaceClusterMemberExclusive(ctx, repo.AssignFaceClusterMemberExclusiveParams{
			ClusterID:       targetClusterID,
			FaceID:          item.ID,
			SimilarityScore: clampSimilarity32(targetSimilarity),
			Confidence:      clampSimilarity32(targetSimilarity),
			IsManual:        boolPtr(false),
		}); err != nil {
			return fmt.Errorf("assign dbscan face member: %w", err)
		}
	}

	if isNewCore {
		sourceClusters := sortedClusterIDs(clusterMatches)
		for _, sourceClusterID := range sourceClusters {
			if sourceClusterID == targetClusterID {
				continue
			}
			if err := q.MergeFaceClusters(ctx, repo.MergeFaceClustersParams{
				ClusterID:   targetClusterID,
				ClusterID_2: sourceClusterID,
			}); err != nil {
				return fmt.Errorf("merge dbscan-connected face clusters: %w", err)
			}
			if err := q.DeleteFaceCluster(ctx, sourceClusterID); err != nil {
				return fmt.Errorf("delete merged face cluster %d: %w", sourceClusterID, err)
			}
		}

		for _, neighbor := range unclusteredNeighbors {
			if _, err := q.AssignFaceClusterMemberExclusive(ctx, repo.AssignFaceClusterMemberExclusiveParams{
				ClusterID:       targetClusterID,
				FaceID:          neighbor.item.ID,
				SimilarityScore: clampSimilarity32(neighbor.similarity),
				Confidence:      clampSimilarity32(neighbor.similarity),
				IsManual:        boolPtr(false),
			}); err != nil {
				return fmt.Errorf("attach dbscan border neighbor: %w", err)
			}
		}
	}

	if err := s.refreshClusterRepresentativeWithQueries(ctx, q, targetClusterID); err != nil {
		return err
	}
	return q.DeleteEmptyUnconfirmedFaceClusters(ctx)
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
	minFaceSize := faceClusterMinAreaPixels
	count, err := q.CountIncrementalFaceNeighbors(ctx, repo.CountIncrementalFaceNeighborsParams{
		ID:             item.ID,
		RepositoryID:   scope.RepositoryID,
		OwnerID:        scope.OwnerID,
		EmbeddingModel: scope.EmbeddingModel,
		MinConfidence:  faceClusterMinConfidence,
		MinFaceSize:    &minFaceSize,
		EmbeddingQuery: &queryVector,
		MinSimilarity:  float64(faceClusterMinSimilarity),
	})
	if err != nil {
		return false, fmt.Errorf("count dbscan face neighbors: %w", err)
	}

	isCore := int(count)+1 >= faceClusterDBSCANMinPoints
	cache[item.ID] = isCore
	return isCore, nil
}

func (s *faceService) RebuildFaceClusters(ctx context.Context, repositoryID pgtype.UUID, ownerID *int32) (FaceClusterRebuildResult, error) {
	startedAt := time.Now()
	result := FaceClusterRebuildResult{
		Algorithm:    "hdbscan-mutual-reachability-v1",
		RepositoryID: optionalUUIDToString(repositoryID),
	}

	minFaceSize := faceClusterMinAreaPixels
	candidateRows, err := s.queries.GetFaceClusteringCandidates(ctx, repo.GetFaceClusteringCandidatesParams{
		RepositoryID:  repositoryID,
		OwnerID:       ownerID,
		MinConfidence: faceClusterMinConfidence,
		MinFaceSize:   &minFaceSize,
	})
	if err != nil {
		return result, fmt.Errorf("load face clustering candidates: %w", err)
	}
	result.CandidateFaces = len(candidateRows)

	oldAssignments, err := s.loadOldFaceClusterAssignments(ctx, repositoryID, ownerID)
	if err != nil {
		return result, err
	}

	groups := groupFaceCandidates(candidateRows)
	plans := make([]faceClusterPersistPlan, 0)
	for _, group := range groups {
		clusters := runHDBSCAN(group)
		for _, cluster := range clusters {
			if len(cluster.indices) < faceClusterHDBSCANMinClusterSize || cluster.averageMedoidSimilarity < faceClusterMinMedoidSimilarity {
				continue
			}

			members := make([]faceClusterCandidate, 0, len(cluster.indices))
			for _, index := range cluster.indices {
				members = append(members, group[index])
			}

			reusedClusterID, name, confirmed, reuseExisting := chooseReusableCluster(members, oldAssignments, plans)
			plans = append(plans, faceClusterPersistPlan{
				members:              members,
				medoidIndex:          cluster.medoidIndex,
				reusedClusterID:      reusedClusterID,
				reuseExistingCluster: reuseExisting,
				name:                 name,
				confirmed:            confirmed,
			})
			result.ClusteredFaces += len(members)
		}
	}
	result.NoiseFaces = result.CandidateFaces - result.ClusteredFaces

	if err := s.withTx(ctx, func(q *repo.Queries) error {
		if err := q.DeleteFaceClusterMembersForScope(ctx, repo.DeleteFaceClusterMembersForScopeParams{
			RepositoryID: repositoryID,
			OwnerID:      ownerID,
		}); err != nil {
			return fmt.Errorf("delete old face cluster memberships: %w", err)
		}

		for _, plan := range plans {
			clusterID, reused, err := s.persistHDBSCANCluster(ctx, q, plan)
			if err != nil {
				return err
			}
			if reused {
				result.ClustersReused++
			} else {
				result.ClustersCreated++
			}
			if err := s.refreshClusterRepresentativeWithQueries(ctx, q, clusterID); err != nil {
				return err
			}
		}

		if err := q.DeleteEmptyUnconfirmedFaceClusters(ctx); err != nil {
			return fmt.Errorf("delete empty face clusters: %w", err)
		}
		return nil
	}); err != nil {
		return result, err
	}

	result.ClustersTotal = result.ClustersCreated + result.ClustersReused
	result.DurationMs = time.Since(startedAt).Milliseconds()
	return result, nil
}

func (s *faceService) loadOldFaceClusterAssignments(ctx context.Context, repositoryID pgtype.UUID, ownerID *int32) (map[int32]oldFaceClusterAssignment, error) {
	rows, err := s.queries.GetFaceClusterAssignmentsForScope(ctx, repo.GetFaceClusterAssignmentsForScopeParams{
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
	})
	if err != nil {
		return nil, fmt.Errorf("load existing face cluster assignments: %w", err)
	}

	assignments := make(map[int32]oldFaceClusterAssignment, len(rows))
	for _, row := range rows {
		assignments[row.FaceID] = oldFaceClusterAssignment{
			clusterID:   row.ClusterID,
			name:        normalizedName(row.ClusterName),
			isConfirmed: row.IsConfirmed != nil && *row.IsConfirmed,
		}
	}
	return assignments, nil
}

func (s *faceService) persistHDBSCANCluster(ctx context.Context, q *repo.Queries, plan faceClusterPersistPlan) (int32, bool, error) {
	if len(plan.members) == 0 {
		return 0, false, fmt.Errorf("cannot persist empty face cluster")
	}

	representative := selectRepresentativeCandidate(plan.members)
	clusterID := plan.reusedClusterID
	reused := false
	if plan.reuseExistingCluster && clusterID > 0 {
		remainingMembers, err := q.GetFaceClusterMembers(ctx, clusterID)
		if err != nil {
			return 0, false, fmt.Errorf("load reusable face cluster %d: %w", clusterID, err)
		}
		if len(remainingMembers) == 0 {
			reused = true
		}
	}

	if !reused {
		cluster, err := q.CreateFaceCluster(ctx, repo.CreateFaceClusterParams{
			ClusterName:          normalizedName(plan.name),
			RepresentativeFaceID: &representative.item.ID,
			ConfidenceScore:      &representative.item.Confidence,
			IsConfirmed:          boolPtr(plan.confirmed),
		})
		if err != nil {
			return 0, false, fmt.Errorf("create hdbscan face cluster: %w", err)
		}
		clusterID = cluster.ClusterID
	}

	medoid := plan.members[0]
	if plan.medoidIndex >= 0 && plan.medoidIndex < len(plan.members) {
		medoid = plan.members[plan.medoidIndex]
	}
	for _, member := range plan.members {
		similarity := clampSimilarity32(float32(cosineSimilarity(member.vector, medoid.vector)))
		if _, err := q.AssignFaceClusterMemberExclusive(ctx, repo.AssignFaceClusterMemberExclusiveParams{
			ClusterID:       clusterID,
			FaceID:          member.item.ID,
			SimilarityScore: similarity,
			Confidence:      similarity,
			IsManual:        boolPtr(false),
		}); err != nil {
			return 0, false, fmt.Errorf("persist hdbscan cluster member: %w", err)
		}
	}

	return clusterID, reused, nil
}

func runHDBSCAN(points []faceClusterCandidate) []hdbscanCluster {
	if len(points) < faceClusterHDBSCANMinClusterSize {
		return nil
	}

	edges, componentRoots := buildMutualReachabilityForest(points)
	if len(edges) == 0 && len(points) >= faceClusterHDBSCANMinClusterSize {
		cluster := hdbscanClusterFromIndices(points, contiguousIndices(len(points)))
		if cluster.averageMedoidSimilarity >= float64(faceClusterMinSimilarity) {
			return []hdbscanCluster{cluster}
		}
		return nil
	}

	roots := buildHDBSCANTree(points, edges, componentRoots)
	selected := make([]hdbscanCluster, 0)
	for _, root := range roots {
		if root == nil || root.size < faceClusterHDBSCANMinClusterSize {
			continue
		}
		selectedNodes := selectStableHDBSCANNodes(root, 0, true)
		for _, node := range selectedNodes {
			cluster := hdbscanClusterFromIndices(points, node.points)
			if cluster.averageMedoidSimilarity >= faceClusterMinMedoidSimilarity {
				selected = append(selected, cluster)
			}
		}
	}
	return selected
}

type hdbscanEdge struct {
	i        int
	j        int
	distance float64
	weight   float64
}

type neighborDistance struct {
	index    int
	distance float64
}

func buildMutualReachabilityForest(points []faceClusterCandidate) ([]hdbscanEdge, []int) {
	n := len(points)
	if n <= 1 {
		return nil, contiguousIndices(n)
	}

	minSamples := minInt(faceClusterHDBSCANMinSamples, n-1)
	kNearest := minInt(maxInt(faceClusterHDBSCANKNearestNeighbors, minSamples), n-1)
	coreDistances := make([]float64, n)
	rawEdges := make(map[[2]int]float64, n*kNearest)

	for i := range points {
		neighbors := make([]neighborDistance, 0, n-1)
		for j := range points {
			if i == j {
				continue
			}
			distance := cosineDistance(points[i].vector, points[j].vector)
			neighbors = append(neighbors, neighborDistance{index: j, distance: distance})
		}
		sort.Slice(neighbors, func(a, b int) bool {
			if neighbors[a].distance == neighbors[b].distance {
				return neighbors[a].index < neighbors[b].index
			}
			return neighbors[a].distance < neighbors[b].distance
		})

		coreIndex := minInt(minSamples-1, len(neighbors)-1)
		if coreIndex >= 0 {
			coreDistances[i] = neighbors[coreIndex].distance
		}

		for rank := 0; rank < minInt(kNearest, len(neighbors)); rank++ {
			neighbor := neighbors[rank]
			if neighbor.distance > faceClusterHDBSCANMaxDistance {
				break
			}
			left, right := orderedPair(i, neighbor.index)
			key := [2]int{left, right}
			if existing, ok := rawEdges[key]; !ok || neighbor.distance < existing {
				rawEdges[key] = neighbor.distance
			}
		}
	}

	edges := make([]hdbscanEdge, 0, len(rawEdges))
	for key, distance := range rawEdges {
		weight := math.Max(distance, math.Max(coreDistances[key[0]], coreDistances[key[1]]))
		edges = append(edges, hdbscanEdge{i: key[0], j: key[1], distance: distance, weight: weight})
	}
	sort.Slice(edges, func(a, b int) bool {
		if edges[a].weight == edges[b].weight {
			if edges[a].i == edges[b].i {
				return edges[a].j < edges[b].j
			}
			return edges[a].i < edges[b].i
		}
		return edges[a].weight < edges[b].weight
	})

	mst, roots := minimumSpanningForest(n, edges)
	return mst, roots
}

func minimumSpanningForest(n int, edges []hdbscanEdge) ([]hdbscanEdge, []int) {
	dsu := newIntDisjointSet(n)
	mst := make([]hdbscanEdge, 0, maxInt(0, n-1))
	for _, edge := range edges {
		if dsu.union(edge.i, edge.j) {
			mst = append(mst, edge)
		}
	}
	rootSet := make(map[int]struct{})
	for i := 0; i < n; i++ {
		rootSet[dsu.find(i)] = struct{}{}
	}
	roots := make([]int, 0, len(rootSet))
	for root := range rootSet {
		roots = append(roots, root)
	}
	sort.Ints(roots)
	return mst, roots
}

type hdbscanTreeNode struct {
	points []int
	left   *hdbscanTreeNode
	right  *hdbscanTreeNode
	lambda float64
	size   int
}

func buildHDBSCANTree(points []faceClusterCandidate, mst []hdbscanEdge, componentRoots []int) []*hdbscanTreeNode {
	n := len(points)
	dsu := newIntDisjointSet(n)
	componentNode := make([]*hdbscanTreeNode, n)
	for i := 0; i < n; i++ {
		componentNode[i] = &hdbscanTreeNode{points: []int{i}, lambda: lambdaFromDistance(0), size: 1}
	}

	for _, edge := range mst {
		leftRoot := dsu.find(edge.i)
		rightRoot := dsu.find(edge.j)
		if leftRoot == rightRoot {
			continue
		}
		leftNode := componentNode[leftRoot]
		rightNode := componentNode[rightRoot]
		mergedPoints := append(append([]int{}, leftNode.points...), rightNode.points...)
		sort.Ints(mergedPoints)
		parent := &hdbscanTreeNode{
			points: mergedPoints,
			left:   leftNode,
			right:  rightNode,
			lambda: lambdaFromDistance(edge.weight),
			size:   len(mergedPoints),
		}
		newRoot := dsu.unionAndRoot(leftRoot, rightRoot)
		componentNode[newRoot] = parent
	}

	rootSet := make(map[int]*hdbscanTreeNode)
	for _, root := range componentRoots {
		currentRoot := dsu.find(root)
		rootSet[currentRoot] = componentNode[currentRoot]
	}
	roots := make([]*hdbscanTreeNode, 0, len(rootSet))
	for _, node := range rootSet {
		roots = append(roots, node)
	}
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].points[0] < roots[j].points[0]
	})
	return roots
}

func selectStableHDBSCANNodes(node *hdbscanTreeNode, birthLambda float64, _ bool) []*hdbscanTreeNode {
	if node == nil || node.size < faceClusterHDBSCANMinClusterSize {
		return nil
	}
	if node.left == nil && node.right == nil {
		return nil
	}

	deathLambda := math.Max(node.lambda, birthLambda)
	selfStability := float64(node.size) * (deathLambda - birthLambda)
	children := make([]*hdbscanTreeNode, 0)
	childStability := 0.0
	for _, child := range []*hdbscanTreeNode{node.left, node.right} {
		if child == nil || child.size < faceClusterHDBSCANMinClusterSize {
			continue
		}
		selected := selectStableHDBSCANNodes(child, deathLambda, false)
		if len(selected) == 0 && child.size >= faceClusterHDBSCANMinClusterSize {
			selected = []*hdbscanTreeNode{child}
		}
		for _, selectedChild := range selected {
			children = append(children, selectedChild)
			childStability += float64(selectedChild.size) * math.Max(0, selectedChild.lambda-deathLambda)
		}
	}

	if len(children) > 0 && childStability > selfStability {
		return children
	}
	return []*hdbscanTreeNode{node}
}

func hdbscanClusterFromIndices(points []faceClusterCandidate, indices []int) hdbscanCluster {
	cluster := hdbscanCluster{indices: append([]int{}, indices...), medoidIndex: 0}
	if len(indices) == 0 {
		return cluster
	}
	sort.Ints(cluster.indices)

	bestAverage := -1.0
	bestLocalIndex := 0
	for localIndex, candidateIndex := range cluster.indices {
		total := 0.0
		for _, otherIndex := range cluster.indices {
			total += cosineSimilarity(points[candidateIndex].vector, points[otherIndex].vector)
		}
		average := total / float64(len(cluster.indices))
		if average > bestAverage || (average == bestAverage && points[candidateIndex].item.ID < points[cluster.indices[bestLocalIndex]].item.ID) {
			bestAverage = average
			bestLocalIndex = localIndex
		}
	}
	cluster.medoidIndex = bestLocalIndex
	cluster.averageMedoidSimilarity = bestAverage
	return cluster
}

func groupFaceCandidates(rows []repo.GetFaceClusteringCandidatesRow) [][]faceClusterCandidate {
	groupsByKey := make(map[string][]faceClusterCandidate)
	for _, row := range rows {
		candidate := faceClusterCandidate{
			item:         faceItemFromClusteringCandidate(row),
			repositoryID: row.RepositoryID,
			ownerID:      cloneInt32Ptr(row.OwnerID),
			vector:       row.Embedding.Slice(),
		}
		key := faceCandidateGroupKey(candidate)
		groupsByKey[key] = append(groupsByKey[key], candidate)
	}

	keys := make([]string, 0, len(groupsByKey))
	for key := range groupsByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	groups := make([][]faceClusterCandidate, 0, len(keys))
	for _, key := range keys {
		group := groupsByKey[key]
		sort.Slice(group, func(i, j int) bool { return group[i].item.ID < group[j].item.ID })
		groups = append(groups, group)
	}
	return groups
}

func faceCandidateGroupKey(candidate faceClusterCandidate) string {
	parts := []string{pgUUIDToString(candidate.repositoryID), "owner", "nil", "model", "nil"}
	if candidate.ownerID != nil {
		parts[2] = fmt.Sprintf("%d", *candidate.ownerID)
	}
	if candidate.item.EmbeddingModel != nil {
		parts[4] = strings.TrimSpace(*candidate.item.EmbeddingModel)
	}
	return strings.Join(parts, ":")
}

func chooseReusableCluster(members []faceClusterCandidate, oldAssignments map[int32]oldFaceClusterAssignment, existingPlans []faceClusterPersistPlan) (int32, *string, bool, bool) {
	usedClusters := make(map[int32]struct{}, len(existingPlans))
	for _, plan := range existingPlans {
		if plan.reuseExistingCluster && plan.reusedClusterID > 0 {
			usedClusters[plan.reusedClusterID] = struct{}{}
		}
	}

	type candidateReuse struct {
		clusterID  int32
		count      int
		name       *string
		confirmed  bool
		bestFaceID int32
	}
	byCluster := make(map[int32]candidateReuse)
	for _, member := range members {
		assignment, ok := oldAssignments[member.item.ID]
		if !ok || assignment.clusterID <= 0 {
			continue
		}
		if _, used := usedClusters[assignment.clusterID]; used {
			continue
		}
		current := byCluster[assignment.clusterID]
		current.clusterID = assignment.clusterID
		current.count++
		current.name = assignment.name
		current.confirmed = assignment.isConfirmed
		if current.bestFaceID == 0 || member.item.ID < current.bestFaceID {
			current.bestFaceID = member.item.ID
		}
		byCluster[assignment.clusterID] = current
	}

	var best candidateReuse
	for _, current := range byCluster {
		if best.clusterID == 0 || current.confirmed && !best.confirmed || current.confirmed == best.confirmed && current.count > best.count || current.confirmed == best.confirmed && current.count == best.count && current.clusterID < best.clusterID {
			best = current
		}
	}
	if best.clusterID == 0 {
		return 0, nil, false, false
	}
	return best.clusterID, best.name, best.confirmed, true
}

func selectRepresentativeCandidate(candidates []faceClusterCandidate) faceClusterCandidate {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if isBetterRepresentative(candidate.item, best.item) {
			best = candidate
		}
	}
	return best
}

func isBetterRepresentative(left, right repo.FaceItem) bool {
	leftPrimary := left.IsPrimary != nil && *left.IsPrimary
	rightPrimary := right.IsPrimary != nil && *right.IsPrimary
	if leftPrimary != rightPrimary {
		return leftPrimary
	}
	if left.Confidence != right.Confidence {
		return left.Confidence > right.Confidence
	}
	leftSize := int32(0)
	if left.FaceSize != nil {
		leftSize = *left.FaceSize
	}
	rightSize := int32(0)
	if right.FaceSize != nil {
		rightSize = *right.FaceSize
	}
	if leftSize != rightSize {
		return leftSize > rightSize
	}
	return left.ID < right.ID
}

func loadFaceMembershipsByFaceID(ctx context.Context, q *repo.Queries, faceIDs []int32) (map[int32]repo.GetFaceClusterMembershipsByFaceIDsRow, error) {
	result := make(map[int32]repo.GetFaceClusterMembershipsByFaceIDsRow)
	if len(faceIDs) == 0 {
		return result, nil
	}
	rows, err := q.GetFaceClusterMembershipsByFaceIDs(ctx, faceIDs)
	if err != nil {
		return nil, fmt.Errorf("load face cluster memberships: %w", err)
	}
	for _, row := range rows {
		if _, exists := result[row.FaceID]; !exists {
			result[row.FaceID] = row
		}
	}
	return result, nil
}

func bestClusterMatch(matches map[int32]float32) (int32, float32) {
	var bestID int32
	var bestSimilarity float32
	for clusterID, similarity := range matches {
		if bestID == 0 || similarity > bestSimilarity || similarity == bestSimilarity && clusterID < bestID {
			bestID = clusterID
			bestSimilarity = similarity
		}
	}
	return bestID, bestSimilarity
}

func sortedClusterIDs(matches map[int32]float32) []int32 {
	ids := make([]int32, 0, len(matches))
	for id := range matches {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if matches[ids[i]] == matches[ids[j]] {
			return ids[i] < ids[j]
		}
		return matches[ids[i]] > matches[ids[j]]
	})
	return ids
}

func faceItemFromIncrementalNeighbor(row repo.GetIncrementalFaceNeighborsRow) repo.FaceItem {
	return repo.FaceItem{
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
	}
}

func faceItemFromClusteringCandidate(row repo.GetFaceClusteringCandidatesRow) repo.FaceItem {
	return repo.FaceItem{
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
	}
}

func cosineDistance(left, right []float32) float64 {
	return 1 - cosineSimilarity(left, right)
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

func lambdaFromDistance(distance float64) float64 {
	if distance <= 1e-9 {
		return 1e9
	}
	return 1 / distance
}

func orderedPair(left, right int) (int, int) {
	if left < right {
		return left, right
	}
	return right, left
}

func contiguousIndices(n int) []int {
	indices := make([]int, n)
	for i := 0; i < n; i++ {
		indices[i] = i
	}
	return indices
}

func cloneInt32Ptr(value *int32) *int32 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

type intDisjointSet struct {
	parent []int
	rank   []int
}

func newIntDisjointSet(size int) *intDisjointSet {
	parent := make([]int, size)
	rank := make([]int, size)
	for i := range parent {
		parent[i] = i
	}
	return &intDisjointSet{parent: parent, rank: rank}
}

func (d *intDisjointSet) find(value int) int {
	if d.parent[value] != value {
		d.parent[value] = d.find(d.parent[value])
	}
	return d.parent[value]
}

func (d *intDisjointSet) union(left, right int) bool {
	leftRoot := d.find(left)
	rightRoot := d.find(right)
	if leftRoot == rightRoot {
		return false
	}
	d.unionAndRoot(leftRoot, rightRoot)
	return true
}

func (d *intDisjointSet) unionAndRoot(leftRoot, rightRoot int) int {
	leftRoot = d.find(leftRoot)
	rightRoot = d.find(rightRoot)
	if leftRoot == rightRoot {
		return leftRoot
	}
	if d.rank[leftRoot] < d.rank[rightRoot] {
		d.parent[leftRoot] = rightRoot
		return rightRoot
	}
	d.parent[rightRoot] = leftRoot
	if d.rank[leftRoot] == d.rank[rightRoot] {
		d.rank[leftRoot]++
	}
	return leftRoot
}
