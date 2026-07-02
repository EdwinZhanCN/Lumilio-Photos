package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/bits"
	"sort"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/utils/phash"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	// DuplicateDetectionVersion is bumped whenever detection semantics change.
	DuplicateDetectionVersion = "duplicates-v2"

	// PHashDuplicateThreshold is the maximum Hamming distance (bits) between two
	// 64-bit perceptual hashes that we still consider visually duplicate.
	// 0 means identical perceptual hash, ~6 is widely cited as a reasonable
	// "looks duplicate" cutoff that still distinguishes burst frames.
	PHashDuplicateThreshold = 6

	// DuplicateMethodExact / Phash / Mixed describe the evidence backing a group.
	DuplicateMethodExact = "exact"
	DuplicateMethodPHash = "phash"
	DuplicateMethodMixed = "mixed"

	DuplicateStatusPending   = "pending"
	DuplicateStatusMerged    = "merged"
	DuplicateStatusDismissed = "dismissed"

	DuplicateRoleKeeper    = "keeper"
	DuplicateRoleDuplicate = "duplicate"
	DuplicateRoleCandidate = "candidate"
)

// ErrDuplicateGroupNotFound is returned when a duplicate group does not exist
// or is no longer pending (e.g. already merged or dismissed).
var ErrDuplicateGroupNotFound = errors.New("duplicate group not found")

// ErrDuplicateGroupAlreadyResolved is returned when callers try to operate on
// a group that has already been merged or dismissed.
var ErrDuplicateGroupAlreadyResolved = errors.New("duplicate group already resolved")

// ErrDuplicateKeeperInvalid is returned when the keeper asset is not part of
// the group, or the keeper/duplicates sets overlap.
var ErrDuplicateKeeperInvalid = errors.New("invalid keeper or duplicate selection")

// DuplicateService groups perceptual hash + exact hash duplicate detection and
// the metadata-preserving merge flow used by the Utilities Rail.
type DuplicateService interface {
	// DetectForRepository rebuilds the pending duplicate graph for a repository
	// by combining exact-hash and pHash edges, running union-find over them,
	// and persisting one duplicate_group per connected component.
	DetectForRepository(ctx context.Context, repositoryID uuid.UUID) (DuplicateDetectionResult, error)

	// GetSummary returns the metrics shown on the Utilities Rail card.
	// ownerID scopes the metrics to one owner's groups; nil means no owner
	// scope (admin).
	GetSummary(ctx context.Context, repositoryID *uuid.UUID, ownerID *int32) (DuplicateSummary, error)

	// ListGroups returns paginated duplicate groups for a repository/status.
	ListGroups(ctx context.Context, params ListDuplicateGroupsParams) (ListDuplicateGroupsResult, error)

	// GetGroup loads a single duplicate group with all assets and edges.
	// requireOwner, when non-nil, makes foreign (or NULL-owner) groups return
	// ErrDuplicateGroupNotFound so their existence is not leaked.
	GetGroup(ctx context.Context, groupID uuid.UUID, requireOwner *int32) (DuplicateGroupDetail, error)

	// MergeGroup is the Apple Photos-style merge: keeper retains visual + selected
	// metadata is unioned onto keeper, and all other duplicates are soft deleted.
	MergeGroup(ctx context.Context, params MergeGroupParams) (MergeGroupResult, error)

	// DismissGroup marks a group as user-acknowledged and not a duplicate.
	// requireOwner follows the same semantics as GetGroup.
	DismissGroup(ctx context.Context, groupID uuid.UUID, requireOwner *int32) error
}

// DuplicateDetectionResult is returned after a detection run finishes.
type DuplicateDetectionResult struct {
	Groups         int
	ExactGroups    int
	PHashGroups    int
	MixedGroups    int
	AssetsAffected int
	GeneratedAt    time.Time
}

// DuplicateSummary describes pending/recoverable counts for the Utilities card.
type DuplicateSummary struct {
	PendingGroups     int64
	MergedGroups      int64
	DismissedGroups   int64
	PendingAssets     int64
	RecoverableAssets int64
	RecoverableBytes  int64
	LastDetectedAt    *time.Time
}

// ListDuplicateGroupsParams is the input for ListGroups.
type ListDuplicateGroupsParams struct {
	RepositoryID *uuid.UUID
	OwnerID      *int32 // nil = no owner scope (admin)
	Status       string
	Limit        int
	Offset       int
}

// ListDuplicateGroupsResult is the output of ListGroups.
type ListDuplicateGroupsResult struct {
	Groups []DuplicateGroupDetail
	Total  int64
}

// DuplicateGroupDetail bundles a group with its assets and edges.
type DuplicateGroupDetail struct {
	Group  repo.DuplicateGroup
	Assets []repo.DuplicateGroupAsset
	Edges  []repo.DuplicateGroupEdge
}

// MergeMetadataPolicy controls which fields flow from duplicates onto keeper.
// Defaults mirror Apple Photos: union albums/tags, prefer keeper for description,
// take MAX rating, OR liked, do NOT migrate faces (keeper retains its own faces).
type MergeMetadataPolicy struct {
	Albums      bool // default true
	Tags        bool // default true
	Rating      bool // default true (MAX)
	Liked       bool // default true (OR)
	Description bool // default true (keeper preferred, fallback to duplicate)
	// Faces is intentionally separate because re-parenting face_items only makes
	// sense for byte-identical duplicates. v1 keeps this off by default.
	Faces bool
}

// DefaultMergePolicy returns the Apple Photos style defaults.
func DefaultMergePolicy() MergeMetadataPolicy {
	return MergeMetadataPolicy{
		Albums:      true,
		Tags:        true,
		Rating:      true,
		Liked:       true,
		Description: true,
		Faces:       false,
	}
}

// MergeGroupParams is the input for MergeGroup.
type MergeGroupParams struct {
	GroupID           uuid.UUID
	KeeperAssetID     uuid.UUID
	DuplicateAssetIDs []uuid.UUID // optional; defaults to all non-keeper members
	Policy            MergeMetadataPolicy
	RequireOwner      *int32 // non-nil: foreign/NULL-owner groups are treated as not found
}

// MergeGroupResult summarizes a merge for the API caller.
type MergeGroupResult struct {
	GroupID          uuid.UUID
	KeeperAssetID    uuid.UUID
	MergedDuplicates []uuid.UUID
	RecoveredBytes   int64
}

// AssetDeleter abstracts the assetService.DeleteAsset path so the duplicate
// service does not import the full asset service and can move files to trash.
type AssetDeleter interface {
	DeleteAsset(ctx context.Context, id uuid.UUID) error
}

type duplicateService struct {
	queries      *repo.Queries
	pool         *pgxpool.Pool
	logger       *zap.Logger
	assetDeleter AssetDeleter
}

// NewDuplicateService constructs the default DuplicateService implementation.
func NewDuplicateService(
	queries *repo.Queries,
	pool *pgxpool.Pool,
	logger *zap.Logger,
	assetDeleter AssetDeleter,
) DuplicateService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &duplicateService{
		queries:      queries,
		pool:         pool,
		logger:       logger,
		assetDeleter: assetDeleter,
	}
}

// ----------------------------------------------------------------------------
// Detection
// ----------------------------------------------------------------------------

type detectionAsset struct {
	id         uuid.UUID
	owner      *int32
	fileSize   int64
	takenTime  time.Time
	uploadTime time.Time
	rating     int32
	phash      uint64
	hasPHash   bool
}

// detectionOwnerKey collapses a nullable owner into a comparable grouping key.
// NULL owners group together (and the resulting group stays admin-only).
func detectionOwnerKey(owner *int32) int64 {
	if owner == nil {
		return -1
	}
	return int64(*owner)
}

type duplicateEdge struct {
	a, b     uuid.UUID
	method   string
	distance float64
	conf     float64
}

// DetectForRepository implements DuplicateService.
func (s *duplicateService) DetectForRepository(ctx context.Context, repositoryID uuid.UUID) (DuplicateDetectionResult, error) {
	logger := s.logger.With(zap.String("repository_id", repositoryID.String()))
	pgRepoID := uuidToPG(repositoryID)

	// 1. Gather exact-hash candidates, pHash embeddings, and stack membership.
	exactRows, err := s.queries.GetExactDuplicateCandidates(ctx, pgRepoID)
	if err != nil {
		return DuplicateDetectionResult{}, fmt.Errorf("load exact candidates: %w", err)
	}
	phashRows, err := s.queries.ListPHashEmbeddingsForRepository(ctx, pgRepoID)
	if err != nil {
		return DuplicateDetectionResult{}, fmt.Errorf("load phash embeddings: %w", err)
	}
	stackRows, err := s.queries.GetStackMembershipForRepository(ctx, pgRepoID)
	if err != nil {
		return DuplicateDetectionResult{}, fmt.Errorf("load stack membership: %w", err)
	}

	stackOf := make(map[uuid.UUID]uuid.UUID, len(stackRows))
	for _, row := range stackRows {
		stackOf[pgToUUID(row.AssetID)] = pgToUUID(row.StackID)
	}

	// 2. Index every photo we saw, regardless of which edge introduced it,
	// so the same asset reachable from exact and pHash collapses into one node.
	assets := make(map[uuid.UUID]*detectionAsset)
	for _, row := range exactRows {
		id := pgToUUID(row.AssetID)
		da := assets[id]
		if da == nil {
			da = &detectionAsset{id: id}
			assets[id] = da
		}
		da.owner = row.OwnerID
		da.fileSize = row.FileSize
		da.takenTime = timestamptzOrZero(row.TakenTime)
		da.uploadTime = timestamptzOrZero(row.UploadTime)
		if row.Rating != nil {
			da.rating = *row.Rating
		}
	}
	for _, row := range phashRows {
		id := pgToUUID(row.AssetID)
		da := assets[id]
		if da == nil {
			da = &detectionAsset{id: id}
			assets[id] = da
		}
		if da.owner == nil {
			da.owner = row.OwnerID
		}
		if da.fileSize == 0 {
			da.fileSize = row.FileSize
		}
		if da.takenTime.IsZero() {
			da.takenTime = timestamptzOrZero(row.TakenTime)
		}
		if da.uploadTime.IsZero() {
			da.uploadTime = timestamptzOrZero(row.UploadTime)
		}
		if row.Rating != nil && da.rating == 0 {
			da.rating = *row.Rating
		}
		if row.Vector != nil {
			if h, ok := vectorToPHash(row.Vector.Slice()); ok {
				da.phash = h
				da.hasPHash = true
			}
		}
	}

	// 3. Collect edges. Each edge is canonicalized to (min, max) endpoints so
	// union-find treats both methods uniformly and exact/pHash edges naturally
	// merge into a single connected component when they share an asset.
	edges := make([]duplicateEdge, 0)
	edges = append(edges, buildExactEdges(exactRows, stackOf)...)
	edges = append(edges, buildPHashEdges(phashRows, stackOf)...)

	if len(edges) == 0 {
		// No duplicates: clear out any previous pending state and exit.
		if err := s.queries.DeletePendingDuplicateGroupsByRepository(ctx, pgRepoID); err != nil {
			return DuplicateDetectionResult{}, fmt.Errorf("clear pending groups: %w", err)
		}
		logger.Info("duplicate detection complete: no edges",
			zap.Int("exact_candidates", len(exactRows)),
			zap.Int("phash_assets", len(phashRows)),
		)
		return DuplicateDetectionResult{GeneratedAt: time.Now()}, nil
	}

	// 4. Union-find over all edges.
	uf := newUnionFind()
	for _, e := range edges {
		uf.union(e.a, e.b)
	}

	// 5. Bucket assets and edges per component.
	type component struct {
		members []*detectionAsset
		edges   []duplicateEdge
		methods map[string]struct{}
	}
	components := make(map[uuid.UUID]*component)
	for _, e := range edges {
		root := uf.find(e.a)
		c := components[root]
		if c == nil {
			c = &component{methods: make(map[string]struct{})}
			components[root] = c
		}
		c.edges = append(c.edges, e)
		c.methods[e.method] = struct{}{}
	}
	for id, da := range assets {
		root, ok := uf.findIfExists(id)
		if !ok {
			continue
		}
		c := components[root]
		if c == nil {
			continue
		}
		c.members = append(c.members, da)
	}

	// 6. Persist groups in a single transaction so the UI never sees a partial
	// graph if the detection run fails halfway.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DuplicateDetectionResult{}, fmt.Errorf("begin detection tx: %w", err)
	}
	defer tx.Rollback(ctx)
	txQueries := s.queries.WithTx(tx)

	if err := txQueries.DeletePendingDuplicateGroupsByRepository(ctx, pgRepoID); err != nil {
		return DuplicateDetectionResult{}, fmt.Errorf("clear pending groups: %w", err)
	}

	result := DuplicateDetectionResult{GeneratedAt: time.Now()}
	for _, c := range components {
		if len(c.members) < 2 {
			continue
		}
		method := determineComponentMethod(c.methods)
		switch method {
		case DuplicateMethodExact:
			result.ExactGroups++
		case DuplicateMethodPHash:
			result.PHashGroups++
		case DuplicateMethodMixed:
			result.MixedGroups++
		}
		result.Groups++
		result.AssetsAffected += len(c.members)

		recommended := pickRecommendedKeeper(c.members)
		totalSize := int64(0)
		for _, m := range c.members {
			totalSize += m.fileSize
		}

		// Edges never cross owners, so every member of a component shares
		// the same owner; stamp it as the group owner (NULL = admin-only).
		groupID, err := txQueries.CreateDuplicateGroup(ctx, repo.CreateDuplicateGroupParams{
			RepositoryID:             pgRepoID,
			OwnerID:                  c.members[0].owner,
			Method:                   method,
			AssetCount:               int32(len(c.members)),
			TotalSize:                totalSize,
			RecommendedKeeperAssetID: uuidToPG(recommended),
			DetectionVersion:         DuplicateDetectionVersion,
		})
		if err != nil {
			return DuplicateDetectionResult{}, fmt.Errorf("create duplicate group: %w", err)
		}

		for _, m := range c.members {
			role := DuplicateRoleCandidate
			if m.id == recommended {
				// Mark the recommendation as keeper to keep the API stable; the user
				// can override at merge time and we will reset roles then.
				role = DuplicateRoleKeeper
			}
			if err := txQueries.InsertDuplicateGroupAsset(ctx, repo.InsertDuplicateGroupAssetParams{
				GroupID:  groupID,
				AssetID:  uuidToPG(m.id),
				Role:     role,
				FileSize: m.fileSize,
			}); err != nil {
				return DuplicateDetectionResult{}, fmt.Errorf("insert duplicate asset: %w", err)
			}
		}

		for _, e := range c.edges {
			a, b := orderEdge(e.a, e.b)
			if err := txQueries.InsertDuplicateGroupEdge(ctx, repo.InsertDuplicateGroupEdgeParams{
				GroupID:    groupID,
				AssetIDA:   uuidToPG(a),
				AssetIDB:   uuidToPG(b),
				Method:     e.method,
				Distance:   e.distance,
				Confidence: e.conf,
			}); err != nil {
				return DuplicateDetectionResult{}, fmt.Errorf("insert duplicate edge: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return DuplicateDetectionResult{}, fmt.Errorf("commit detection tx: %w", err)
	}

	logger.Info("duplicate detection complete",
		zap.Int("groups", result.Groups),
		zap.Int("exact_groups", result.ExactGroups),
		zap.Int("phash_groups", result.PHashGroups),
		zap.Int("mixed_groups", result.MixedGroups),
		zap.Int("affected_assets", result.AssetsAffected),
	)
	return result, nil
}

// buildExactEdges turns rows sharing (owner, hash, file_size) into edges. To
// keep the edge set linear we connect each non-first row to the first row in
// the run; union-find handles full connectivity. Owner is part of the grouping
// key so a group never spans owners. Pairs in the same photo stack are skipped.
func buildExactEdges(rows []repo.GetExactDuplicateCandidatesRow, stackOf map[uuid.UUID]uuid.UUID) []duplicateEdge {
	if len(rows) == 0 {
		return nil
	}
	var edges []duplicateEdge
	type key struct {
		owner int64
		hash  string
		size  int64
	}
	var anchor uuid.UUID
	var have bool
	var prev key
	for _, r := range rows {
		k := key{owner: detectionOwnerKey(r.OwnerID), hash: derefString(r.Hash), size: r.FileSize}
		id := pgToUUID(r.AssetID)
		if !have || k != prev {
			anchor = id
			prev = k
			have = true
			continue
		}
		if sameStackPair(anchor, id, stackOf) {
			continue
		}
		edges = append(edges, duplicateEdge{
			a: anchor, b: id,
			method:   DuplicateMethodExact,
			distance: 0,
			conf:     1.0,
		})
	}
	return edges
}

// buildPHashEdges loads pHash uint64 values for each photo and produces edges
// for pairs within PHashDuplicateThreshold Hamming distance. We use a 16-bit
// prefix bucket as a cheap candidate filter: any two 64-bit hashes within k
// bits must share at least one 16-bit chunk identical when k <= 6 (pigeonhole
// on 4 chunks of 16 bits). For larger thresholds this filter degrades, which
// is fine — we still verify distance below. Pairs in the same photo stack or
// with different owners are skipped: duplicate edges never cross owners.
func buildPHashEdges(rows []repo.ListPHashEmbeddingsForRepositoryRow, stackOf map[uuid.UUID]uuid.UUID) []duplicateEdge {
	type item struct {
		id    uuid.UUID
		owner int64
		hash  uint64
	}
	items := make([]item, 0, len(rows))
	for _, r := range rows {
		if r.Vector == nil {
			continue
		}
		h, ok := vectorToPHash(r.Vector.Slice())
		if !ok {
			continue
		}
		items = append(items, item{id: pgToUUID(r.AssetID), owner: detectionOwnerKey(r.OwnerID), hash: h})
	}
	if len(items) < 2 {
		return nil
	}

	// Bucket by each of the 4 16-bit chunks; collect candidate pairs from each.
	type pairKey struct {
		a, b uuid.UUID
	}
	candidates := make(map[pairKey]struct{})
	for chunk := 0; chunk < 4; chunk++ {
		shift := uint(chunk * 16)
		buckets := make(map[uint16][]int)
		for idx, it := range items {
			prefix := uint16(it.hash >> shift)
			buckets[prefix] = append(buckets[prefix], idx)
		}
		for _, idxs := range buckets {
			if len(idxs) < 2 {
				continue
			}
			for i := 0; i < len(idxs); i++ {
				for j := i + 1; j < len(idxs); j++ {
					if items[idxs[i]].owner != items[idxs[j]].owner {
						continue
					}
					a, b := items[idxs[i]].id, items[idxs[j]].id
					if a == b {
						continue
					}
					if bytes.Compare(a[:], b[:]) > 0 {
						a, b = b, a
					}
					candidates[pairKey{a, b}] = struct{}{}
				}
			}
		}
	}

	idIndex := make(map[uuid.UUID]uint64, len(items))
	for _, it := range items {
		idIndex[it.id] = it.hash
	}

	edges := make([]duplicateEdge, 0, len(candidates))
	for pk := range candidates {
		ha, okA := idIndex[pk.a]
		hb, okB := idIndex[pk.b]
		if !okA || !okB {
			continue
		}
		dist := bits.OnesCount64(ha ^ hb)
		if dist > PHashDuplicateThreshold {
			continue
		}
		if sameStackPair(pk.a, pk.b, stackOf) {
			continue
		}
		conf := 1.0 - float64(dist)/64.0
		edges = append(edges, duplicateEdge{
			a: pk.a, b: pk.b,
			method:   DuplicateMethodPHash,
			distance: float64(dist),
			conf:     conf,
		})
	}
	return edges
}

// sameStackPair reports whether both assets belong to the same non-empty stack.
func sameStackPair(a, b uuid.UUID, stackOf map[uuid.UUID]uuid.UUID) bool {
	if len(stackOf) == 0 {
		return false
	}
	sa, okA := stackOf[a]
	sb, okB := stackOf[b]
	if !okA || !okB {
		return false
	}
	return sa == sb
}

// pickRecommendedKeeper picks the member most likely to be the user's
// preferred original: largest file_size first, then highest rating, then
// earliest taken_time, with id tiebreaker for determinism.
func pickRecommendedKeeper(members []*detectionAsset) uuid.UUID {
	if len(members) == 0 {
		return uuid.Nil
	}
	sort.Slice(members, func(i, j int) bool {
		mi, mj := members[i], members[j]
		if mi.fileSize != mj.fileSize {
			return mi.fileSize > mj.fileSize
		}
		if mi.rating != mj.rating {
			return mi.rating > mj.rating
		}
		ti, tj := mi.takenTime, mj.takenTime
		if ti.IsZero() {
			ti = mi.uploadTime
		}
		if tj.IsZero() {
			tj = mj.uploadTime
		}
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return bytes.Compare(mi.id[:], mj.id[:]) < 0
	})
	return members[0].id
}

// vectorToPHash reconstructs the 64-bit perceptual hash from the 0/1 float
// vector stored in `embeddings.vector` (see phashToVector in phash_worker.go).
func vectorToPHash(vec []float32) (uint64, bool) {
	return phash.FromVector(vec)
}

func determineComponentMethod(methods map[string]struct{}) string {
	_, hasExact := methods[DuplicateMethodExact]
	_, hasPHash := methods[DuplicateMethodPHash]
	switch {
	case hasExact && hasPHash:
		return DuplicateMethodMixed
	case hasExact:
		return DuplicateMethodExact
	default:
		return DuplicateMethodPHash
	}
}

func orderEdge(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if bytes.Compare(a[:], b[:]) <= 0 {
		return a, b
	}
	return b, a
}

// ----------------------------------------------------------------------------
// Read APIs
// ----------------------------------------------------------------------------

func (s *duplicateService) GetSummary(ctx context.Context, repositoryID *uuid.UUID, ownerID *int32) (DuplicateSummary, error) {
	row, err := s.queries.GetDuplicateSummary(ctx, repo.GetDuplicateSummaryParams{
		RepositoryID: optionalUUID(repositoryID),
		OwnerID:      ownerID,
	})
	if err != nil {
		return DuplicateSummary{}, err
	}
	summary := DuplicateSummary{
		PendingGroups:     row.PendingGroups,
		MergedGroups:      row.MergedGroups,
		DismissedGroups:   row.DismissedGroups,
		PendingAssets:     row.PendingAssets,
		RecoverableAssets: row.RecoverableAssets,
		RecoverableBytes:  row.RecoverableBytes,
	}
	if row.LastDetectedAt.Valid {
		t := row.LastDetectedAt.Time
		summary.LastDetectedAt = &t
	}
	return summary, nil
}

func (s *duplicateService) ListGroups(ctx context.Context, params ListDuplicateGroupsParams) (ListDuplicateGroupsResult, error) {
	limit := params.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	pgRepo := optionalUUID(params.RepositoryID)
	var pgStatus *string
	if params.Status != "" {
		s := params.Status
		pgStatus = &s
	}

	groups, err := s.queries.ListDuplicateGroups(ctx, repo.ListDuplicateGroupsParams{
		RepositoryID: pgRepo,
		OwnerID:      params.OwnerID,
		Status:       pgStatus,
		Limit:        int32(limit),
		Offset:       int32(offset),
	})
	if err != nil {
		return ListDuplicateGroupsResult{}, err
	}
	total, err := s.queries.CountDuplicateGroups(ctx, repo.CountDuplicateGroupsParams{
		RepositoryID: pgRepo,
		OwnerID:      params.OwnerID,
		Status:       pgStatus,
	})
	if err != nil {
		return ListDuplicateGroupsResult{}, err
	}

	if len(groups) == 0 {
		return ListDuplicateGroupsResult{Total: total}, nil
	}

	ids := make([]pgtype.UUID, 0, len(groups))
	for _, g := range groups {
		ids = append(ids, g.GroupID)
	}
	assetRows, err := s.queries.GetDuplicateGroupAssetsBatch(ctx, ids)
	if err != nil {
		return ListDuplicateGroupsResult{}, err
	}
	assetsByGroup := make(map[uuid.UUID][]repo.DuplicateGroupAsset, len(groups))
	for _, row := range assetRows {
		id := pgToUUID(row.GroupID)
		assetsByGroup[id] = append(assetsByGroup[id], row)
	}

	result := ListDuplicateGroupsResult{Total: total, Groups: make([]DuplicateGroupDetail, 0, len(groups))}
	for _, g := range groups {
		detail := DuplicateGroupDetail{Group: g}
		detail.Assets = assetsByGroup[pgToUUID(g.GroupID)]
		// Edges are only fetched on demand for the detail page to keep the list
		// payload small. Callers that need edges should use GetGroup.
		result.Groups = append(result.Groups, detail)
	}
	return result, nil
}

func (s *duplicateService) GetGroup(ctx context.Context, groupID uuid.UUID, requireOwner *int32) (DuplicateGroupDetail, error) {
	pgID := uuidToPG(groupID)
	group, err := s.queries.GetDuplicateGroupByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DuplicateGroupDetail{}, ErrDuplicateGroupNotFound
		}
		return DuplicateGroupDetail{}, err
	}
	if !duplicateGroupOwnedBy(group, requireOwner) {
		return DuplicateGroupDetail{}, ErrDuplicateGroupNotFound
	}
	assets, err := s.queries.GetDuplicateGroupAssets(ctx, pgID)
	if err != nil {
		return DuplicateGroupDetail{}, err
	}
	edges, err := s.queries.GetDuplicateGroupEdges(ctx, pgID)
	if err != nil {
		return DuplicateGroupDetail{}, err
	}
	return DuplicateGroupDetail{Group: group, Assets: assets, Edges: edges}, nil
}

// ----------------------------------------------------------------------------
// Merge / Dismiss
// ----------------------------------------------------------------------------

func (s *duplicateService) MergeGroup(ctx context.Context, params MergeGroupParams) (MergeGroupResult, error) {
	if params.GroupID == uuid.Nil || params.KeeperAssetID == uuid.Nil {
		return MergeGroupResult{}, ErrDuplicateKeeperInvalid
	}

	pgGroupID := uuidToPG(params.GroupID)
	group, err := s.queries.GetDuplicateGroupByID(ctx, pgGroupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MergeGroupResult{}, ErrDuplicateGroupNotFound
		}
		return MergeGroupResult{}, err
	}
	if !duplicateGroupOwnedBy(group, params.RequireOwner) {
		return MergeGroupResult{}, ErrDuplicateGroupNotFound
	}
	if group.Status != DuplicateStatusPending {
		return MergeGroupResult{}, ErrDuplicateGroupAlreadyResolved
	}

	assetRows, err := s.queries.GetDuplicateGroupAssets(ctx, pgGroupID)
	if err != nil {
		return MergeGroupResult{}, fmt.Errorf("load group members: %w", err)
	}

	members := make(map[uuid.UUID]repo.DuplicateGroupAsset, len(assetRows))
	for _, r := range assetRows {
		members[pgToUUID(r.AssetID)] = r
	}

	if _, ok := members[params.KeeperAssetID]; !ok {
		return MergeGroupResult{}, ErrDuplicateKeeperInvalid
	}

	// If caller did not specify which assets to delete, default to every other
	// member of the group (Apple Photos style "merge all").
	duplicates := params.DuplicateAssetIDs
	if len(duplicates) == 0 {
		duplicates = make([]uuid.UUID, 0, len(members)-1)
		for id := range members {
			if id != params.KeeperAssetID {
				duplicates = append(duplicates, id)
			}
		}
	} else {
		for _, id := range duplicates {
			if id == params.KeeperAssetID {
				return MergeGroupResult{}, ErrDuplicateKeeperInvalid
			}
			if _, ok := members[id]; !ok {
				return MergeGroupResult{}, ErrDuplicateKeeperInvalid
			}
		}
	}
	if len(duplicates) == 0 {
		return MergeGroupResult{}, ErrDuplicateKeeperInvalid
	}

	policy := params.Policy
	if !policy.Albums && !policy.Tags && !policy.Rating && !policy.Liked && !policy.Description && !policy.Faces {
		policy = DefaultMergePolicy()
	}

	// Stage 1: metadata merge in a single transaction so partial failures leave
	// no half-merged keeper.
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MergeGroupResult{}, fmt.Errorf("begin merge tx: %w", err)
	}
	defer tx.Rollback(ctx)
	txQueries := s.queries.WithTx(tx)

	keeperAsset, err := txQueries.GetAssetByID(ctx, uuidToPG(params.KeeperAssetID))
	if err != nil {
		return MergeGroupResult{}, fmt.Errorf("load keeper asset: %w", err)
	}

	mergedRating, mergedLiked, mergedDescription, err := computeKeeperPreferences(
		ctx,
		txQueries,
		keeperAsset,
		duplicates,
		policy,
	)
	if err != nil {
		return MergeGroupResult{}, fmt.Errorf("compute merged preferences: %w", err)
	}

	for _, dupID := range duplicates {
		if policy.Albums {
			if err := txQueries.MergeAlbumAssetsForDuplicate(ctx, repo.MergeAlbumAssetsForDuplicateParams{
				KeeperAssetID:    uuidToPG(params.KeeperAssetID),
				DuplicateAssetID: uuidToPG(dupID),
			}); err != nil {
				return MergeGroupResult{}, fmt.Errorf("merge albums for %s: %w", dupID, err)
			}
		}
		if policy.Tags {
			if err := txQueries.MergeAssetTagsForDuplicate(ctx, repo.MergeAssetTagsForDuplicateParams{
				KeeperAssetID:    uuidToPG(params.KeeperAssetID),
				DuplicateAssetID: uuidToPG(dupID),
			}); err != nil {
				return MergeGroupResult{}, fmt.Errorf("merge tags for %s: %w", dupID, err)
			}
		}
		// Face re-parenting is only safe for exact duplicates (bytes-identical
		// images share bounding boxes). We hold the flag in policy but require
		// `exact` group method as an extra guard.
		if policy.Faces && group.Method == DuplicateMethodExact {
			if err := txQueries.MergeFaceClustersForDuplicate(ctx, repo.MergeFaceClustersForDuplicateParams{
				KeeperAssetID:    uuidToPG(params.KeeperAssetID),
				DuplicateAssetID: uuidToPG(dupID),
			}); err != nil {
				return MergeGroupResult{}, fmt.Errorf("merge faces for %s: %w", dupID, err)
			}
		}
	}

	if err := txQueries.ApplyMergedKeeperPreferences(ctx, repo.ApplyMergedKeeperPreferencesParams{
		MergedRating:      mergedRating,
		MergedLiked:       mergedLiked,
		MergedDescription: mergedDescription,
		KeeperAssetID:     uuidToPG(params.KeeperAssetID),
	}); err != nil {
		return MergeGroupResult{}, fmt.Errorf("apply keeper preferences: %w", err)
	}

	if err := txQueries.UpdateDuplicateGroupKeeperRole(ctx, repo.UpdateDuplicateGroupKeeperRoleParams{
		KeeperAssetID: uuidToPG(params.KeeperAssetID),
		GroupID:       pgGroupID,
	}); err != nil {
		return MergeGroupResult{}, fmt.Errorf("update keeper role: %w", err)
	}

	if err := txQueries.MarkDuplicateGroupMerged(ctx, repo.MarkDuplicateGroupMergedParams{
		GroupID:       pgGroupID,
		KeeperAssetID: uuidToPG(params.KeeperAssetID),
	}); err != nil {
		return MergeGroupResult{}, fmt.Errorf("mark group merged: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return MergeGroupResult{}, fmt.Errorf("commit merge tx: %w", err)
	}

	// Stage 2: soft-delete the duplicate assets outside the transaction so we
	// can also move the files into the per-repository trash bin via the asset
	// service. Each delete is independent and idempotent enough that partial
	// failure leaves the group in a recoverable state (status=merged, some
	// assets still alive); the user can dismiss/re-detect to clean up.
	recovered := int64(0)
	deleted := make([]uuid.UUID, 0, len(duplicates))
	for _, dupID := range duplicates {
		if s.assetDeleter != nil {
			if err := s.assetDeleter.DeleteAsset(ctx, dupID); err != nil {
				s.logger.Warn("soft delete duplicate failed",
					zap.String("asset_id", dupID.String()),
					zap.Error(err),
				)
				continue
			}
		}
		if m, ok := members[dupID]; ok {
			recovered += m.FileSize
		}
		deleted = append(deleted, dupID)
	}

	return MergeGroupResult{
		GroupID:          params.GroupID,
		KeeperAssetID:    params.KeeperAssetID,
		MergedDuplicates: deleted,
		RecoveredBytes:   recovered,
	}, nil
}

func (s *duplicateService) DismissGroup(ctx context.Context, groupID uuid.UUID, requireOwner *int32) error {
	pgID := uuidToPG(groupID)
	group, err := s.queries.GetDuplicateGroupByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrDuplicateGroupNotFound
		}
		return err
	}
	if !duplicateGroupOwnedBy(group, requireOwner) {
		return ErrDuplicateGroupNotFound
	}
	if group.Status != DuplicateStatusPending {
		return ErrDuplicateGroupAlreadyResolved
	}
	return s.queries.MarkDuplicateGroupDismissed(ctx, pgID)
}

// duplicateGroupOwnedBy applies the whole-entity ownership rule: a nil
// requirement (admin) passes everything; otherwise the group's owner must
// match exactly, and NULL-owner groups stay admin-only.
func duplicateGroupOwnedBy(group repo.DuplicateGroup, requireOwner *int32) bool {
	if requireOwner == nil {
		return true
	}
	return group.OwnerID != nil && *group.OwnerID == *requireOwner
}

// computeKeeperPreferences gathers MAX(rating), OR(liked), and a description
// fallback by reading the duplicate assets. Each policy flag gates whether
// the corresponding value is returned, so the SQL UPDATE leaves untouched
// fields alone.
func computeKeeperPreferences(
	ctx context.Context,
	q *repo.Queries,
	keeper repo.Asset,
	duplicates []uuid.UUID,
	policy MergeMetadataPolicy,
) (*int32, *bool, *string, error) {
	var (
		rating      *int32
		liked       *bool
		description *string
	)

	if policy.Rating && keeper.Rating != nil {
		v := *keeper.Rating
		rating = &v
	}
	if policy.Liked && keeper.Liked != nil {
		v := *keeper.Liked
		liked = &v
	}

	for _, id := range duplicates {
		dup, err := q.GetAssetByID(ctx, uuidToPG(id))
		if err != nil {
			return nil, nil, nil, fmt.Errorf("load duplicate %s: %w", id, err)
		}
		if policy.Rating && dup.Rating != nil {
			if rating == nil || *dup.Rating > *rating {
				v := *dup.Rating
				rating = &v
			}
		}
		if policy.Liked && dup.Liked != nil && *dup.Liked {
			t := true
			liked = &t
		}
		if policy.Description && description == nil {
			if desc := descriptionFromMetadata(dup.SpecificMetadata); desc != "" {
				v := desc
				description = &v
			}
		}
	}

	if policy.Description && description != nil {
		if keeperDesc := descriptionFromMetadata(keeper.SpecificMetadata); keeperDesc != "" {
			// Keeper already has a description; do not overwrite it. SQL guards
			// against this too, but skip the parameter so the UPDATE is a no-op
			// for cleanliness.
			description = nil
		}
	}

	return rating, liked, description, nil
}

// ----------------------------------------------------------------------------
// Internal helpers
// ----------------------------------------------------------------------------

type unionFind struct {
	parent map[uuid.UUID]uuid.UUID
	rank   map[uuid.UUID]int
}

func newUnionFind() *unionFind {
	return &unionFind{
		parent: make(map[uuid.UUID]uuid.UUID),
		rank:   make(map[uuid.UUID]int),
	}
}

func (u *unionFind) find(x uuid.UUID) uuid.UUID {
	p, ok := u.parent[x]
	if !ok {
		u.parent[x] = x
		u.rank[x] = 0
		return x
	}
	if p == x {
		return x
	}
	root := u.find(p)
	u.parent[x] = root
	return root
}

func (u *unionFind) findIfExists(x uuid.UUID) (uuid.UUID, bool) {
	if _, ok := u.parent[x]; !ok {
		return uuid.Nil, false
	}
	return u.find(x), true
}

func (u *unionFind) union(a, b uuid.UUID) {
	ra := u.find(a)
	rb := u.find(b)
	if ra == rb {
		return
	}
	if u.rank[ra] < u.rank[rb] {
		ra, rb = rb, ra
	}
	u.parent[rb] = ra
	if u.rank[ra] == u.rank[rb] {
		u.rank[ra]++
	}
}

func uuidToPG(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func pgToUUID(id pgtype.UUID) uuid.UUID {
	if !id.Valid {
		return uuid.Nil
	}
	return uuid.UUID(id.Bytes)
}

func optionalUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return uuidToPG(*id)
}

func timestamptzOrZero(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// descriptionFromMetadata extracts the top-level "description" field from any
// SpecificMetadata payload (Photo / Video / Audio). It is safe on nil or
// malformed JSON and returns "" in that case.
func descriptionFromMetadata(meta dbtypes.SpecificMetadata) string {
	if len(meta) == 0 {
		return ""
	}
	var flat map[string]any
	if err := json.Unmarshal([]byte(meta), &flat); err != nil {
		return ""
	}
	if v, ok := flat["description"].(string); ok {
		return v
	}
	return ""
}
