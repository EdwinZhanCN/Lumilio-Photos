package search

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/search/bleveocr"

	"github.com/google/uuid"
)

const (
	// Vec1 has no sqlite-vec-style 4096 hard limit. Keep a generous explicit
	// work bound so a single request cannot allocate an unbounded result heap.
	// The asset request is expanded because videos may contribute many frames.
	maxVec1KNNVectors       = 65_536
	embeddingKNNExpansion   = 8
	maxANNAssetCandidateSet = maxVec1KNNVectors / embeddingKNNExpansion
)

type EmbeddingRetriever struct {
	pool         *sql.DB
	embed        EmbedQueryFunc
	resolveSpace ResolveEmbeddingSpaceFunc
	weight       float64
}

func NewEmbeddingRetriever(pool *sql.DB, embed EmbedQueryFunc, resolveSpace ResolveEmbeddingSpaceFunc, weight float64) *EmbeddingRetriever {
	return &EmbeddingRetriever{pool: pool, embed: embed, resolveSpace: resolveSpace, weight: weight}
}

func (r *EmbeddingRetriever) Source() string  { return SourceEmbedding }
func (r *EmbeddingRetriever) Weight() float64 { return r.weight }

func (r *EmbeddingRetriever) Retrieve(ctx context.Context, req Request) ([]Candidate, error) {
	if r == nil || r.pool == nil || r.resolveSpace == nil {
		return nil, fmt.Errorf("embedding retriever is not configured")
	}
	if req.QueryEmbedding == nil && r.embed == nil {
		return nil, fmt.Errorf("embedding retriever is not configured")
	}

	embedding, space, err := r.resolveQuerySpace(ctx, req)
	if err != nil {
		return nil, err
	}

	builder := &sqlBuilder{}
	queryVector := dbtypes.NewVector(embedding.Vector)
	vectorPlaceholder := builder.addArg(queryVector)
	knnLimit := expandedEmbeddingKNNLimit(req.TopK)
	vec1ParamsPlaceholder := builder.addArg(fmt.Sprintf(`{"k":%d,"nprobe":0.15}`, knnLimit))
	vec1Conditions, err := buildVec1FilterConditions(builder, req.Filter, space.ID)
	if err != nil {
		return nil, err
	}
	spacePlaceholder := builder.addArg(space.ID)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return nil, err
	}
	conditions = append(conditions,
		fmt.Sprintf("e.space_id = %s", spacePlaceholder),
	)
	limitPlaceholder := builder.addArg(req.TopK)

	query := fmt.Sprintf(`
WITH nearest AS (
  SELECT
    e.id AS rowid,
    vec1_l2_distance(e.vector, %s) AS distance
  FROM search_embeddings_vec(%s, %s) AS v
  JOIN search_embeddings e ON e.id = v.rowid
  WHERE %s
),
ranked AS (
  SELECT
    a.asset_id,
    e.frame_ts_ms,
    nearest.distance,
    ROW_NUMBER() OVER (
      PARTITION BY a.asset_id
      ORDER BY nearest.distance, e.frame_ts_ms IS NULL, e.frame_ts_ms
    ) AS distance_rank
  FROM nearest
  JOIN search_embeddings e ON e.id = nearest.rowid
  JOIN assets a ON a.asset_id = e.asset_id
  WHERE %s
)
SELECT
  asset_id,
  MIN(distance) AS raw_score,
  MAX(CASE WHEN distance_rank = 1 THEN frame_ts_ms END) AS best_ts
FROM ranked
GROUP BY asset_id
ORDER BY raw_score, asset_id DESC
LIMIT %s
`,
		vectorPlaceholder,
		vectorPlaceholder,
		vec1ParamsPlaceholder,
		joinConditions(vec1Conditions),
		joinConditions(conditions),
		limitPlaceholder,
	)

	rows, err := r.pool.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return nil, fmt.Errorf("embedding retrieve: %w", err)
	}
	defer rows.Close()

	candidates, err := collectCandidates(rows, SourceEmbedding)
	if err != nil {
		return nil, err
	}
	squareRootCandidateDistances(candidates)
	return candidates, nil
}

func expandedEmbeddingKNNLimit(topK int) int {
	if topK <= 0 {
		return 1
	}
	if topK > maxVec1KNNVectors/embeddingKNNExpansion {
		return maxVec1KNNVectors
	}
	return topK * embeddingKNNExpansion
}

// buildVec1FilterConditions pushes the high-value authorization and media
// predicates into Vec1. More relational filters are still checked against the
// authoritative tables after candidate selection.
func buildVec1FilterConditions(builder *sqlBuilder, filter Filter, spaceID int64) ([]string, error) {
	isDeleted := false
	if filter.IsDeleted != nil {
		isDeleted = *filter.IsDeleted
	}
	conditions := []string{
		fmt.Sprintf("v.space_id = %s", builder.addArg(spaceID)),
		fmt.Sprintf("v.is_deleted = %s", builder.addArg(isDeleted)),
	}
	if filter.OwnerID != nil {
		conditions = append(conditions, fmt.Sprintf("v.owner_id = %s", builder.addArg(*filter.OwnerID)))
	}
	if filter.AssetType != nil {
		conditions = append(conditions, fmt.Sprintf("v.asset_type = %s", builder.addArg(*filter.AssetType)))
	}
	if len(filter.AssetTypes) > 0 {
		placeholder, err := builder.addJSONArg(filter.AssetTypes)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, fmt.Sprintf("v.asset_type IN (SELECT value FROM json_each(%s))", placeholder))
	}
	return conditions, nil
}

func squareRootCandidateDistances(candidates []Candidate) {
	for index := range candidates {
		candidates[index].RawScore = math.Sqrt(candidates[index].RawScore)
	}
}

func (r *EmbeddingRetriever) resolveQuerySpace(ctx context.Context, req Request) (QueryEmbedding, repo.EmbeddingSpace, error) {
	var embedding QueryEmbedding
	if req.QueryEmbedding != nil {
		embedding = *req.QueryEmbedding
	} else {
		var err error
		embedding, err = r.embed(ctx, req.Query, true)
		if err != nil {
			return QueryEmbedding{}, repo.EmbeddingSpace{}, err
		}
	}
	if len(embedding.Vector) == 0 {
		return QueryEmbedding{}, repo.EmbeddingSpace{}, fmt.Errorf("query embedding is empty")
	}

	space, err := r.resolveSpace(ctx, embedding.Model, len(embedding.Vector))
	if err != nil {
		return QueryEmbedding{}, repo.EmbeddingSpace{}, err
	}
	if space.ID <= 0 || space.Dimensions <= 0 {
		return QueryEmbedding{}, repo.EmbeddingSpace{}, fmt.Errorf("invalid embedding search space")
	}
	return embedding, space, nil
}

type TextRetriever struct {
	pool   *sql.DB
	source string
	weight float64
}

func NewPlaceRetriever(pool *sql.DB, weight float64) *TextRetriever {
	return &TextRetriever{pool: pool, source: SourcePlace, weight: weight}
}

func (r *TextRetriever) Source() string  { return r.source }
func (r *TextRetriever) Weight() float64 { return r.weight }

func (r *TextRetriever) Retrieve(ctx context.Context, req Request) ([]Candidate, error) {
	if r == nil || r.pool == nil {
		return nil, fmt.Errorf("%s retriever is not configured", r.source)
	}
	switch r.source {
	case SourcePlace:
		return r.retrievePlace(ctx, req)
	default:
		return nil, fmt.Errorf("unknown text retriever source: %s", r.source)
	}
}

func (r *TextRetriever) retrievePlace(ctx context.Context, req Request) ([]Candidate, error) {
	matchQuery := ftsMatchQuery(req.Query)
	if matchQuery == "" {
		return nil, nil
	}
	builder := &sqlBuilder{}
	queryPlaceholder := builder.addArg(matchQuery)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return nil, err
	}
	limitPlaceholder := builder.addArg(req.TopK)

	query := fmt.Sprintf(`
WITH matched_locations AS (
  SELECT lc.cluster_id, -bm25(location_search_fts) AS score
  FROM location_search_fts
  JOIN location_clusters lc ON lc.rowid = location_search_fts.rowid
  WHERE location_search_fts MATCH %s
)
SELECT
  a.asset_id,
  MAX(ml.score) AS raw_score,
  NULL AS best_ts
FROM matched_locations ml
JOIN location_cluster_assets lca ON lca.cluster_id = ml.cluster_id
JOIN assets a ON a.asset_id = lca.asset_id
WHERE %s
GROUP BY a.asset_id
ORDER BY raw_score DESC, a.asset_id DESC
LIMIT %s
`, queryPlaceholder, joinConditions(conditions), limitPlaceholder)

	rows, err := r.pool.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return nil, fmt.Errorf("place retrieve: %w", err)
	}
	defer rows.Close()

	return collectCandidates(rows, SourcePlace)
}

const bleveOCRPageSize = 256

type BleveOCRRetriever struct {
	pool   *sql.DB
	index  *bleveocr.Index
	weight float64
}

func NewBleveOCRRetriever(pool *sql.DB, index *bleveocr.Index, weight float64) *BleveOCRRetriever {
	return &BleveOCRRetriever{pool: pool, index: index, weight: weight}
}

func (r *BleveOCRRetriever) Source() string  { return SourceOCR }
func (r *BleveOCRRetriever) Weight() float64 { return r.weight }

func (r *BleveOCRRetriever) Retrieve(ctx context.Context, req Request) ([]Candidate, error) {
	if r == nil || r.pool == nil || r.index == nil {
		return nil, fmt.Errorf("OCR Bleve retriever is not configured")
	}
	if req.TopK <= 0 || strings.TrimSpace(req.Query) == "" {
		return []Candidate{}, nil
	}

	filters := bleveocr.BasicFilters{
		OwnerID:    req.Filter.OwnerID,
		AssetType:  req.Filter.AssetType,
		AssetTypes: req.Filter.AssetTypes,
		IsDeleted:  req.Filter.IsDeleted != nil && *req.Filter.IsDeleted,
	}
	if req.Filter.RepositoryID != nil {
		repositoryID := req.Filter.RepositoryID.String()
		filters.RepositoryID = &repositoryID
	}

	candidates := make([]Candidate, 0, req.TopK)
	seen := make(map[uuid.UUID]struct{})
	for _, mode := range []bleveocr.QueryMode{bleveocr.QueryStrict, bleveocr.QueryRelaxed} {
		from := 0
		for len(candidates) < req.TopK {
			size := bleveOCRPageSize
			page, err := r.index.SearchPage(ctx, req.Query, filters, mode, from, size)
			if err != nil {
				return nil, err
			}
			if len(page.Hits) == 0 {
				break
			}

			hits := make([]bleveocr.Hit, 0, len(page.Hits))
			for _, hit := range page.Hits {
				assetID, err := uuid.Parse(hit.AssetID)
				if err != nil {
					return nil, fmt.Errorf("parse OCR Bleve asset id %q: %w", hit.AssetID, err)
				}
				if _, exists := seen[assetID]; exists {
					continue
				}
				seen[assetID] = struct{}{}
				hits = append(hits, hit)
			}

			allowed, err := r.filterCandidates(ctx, hits, req.Filter)
			if err != nil {
				return nil, err
			}
			for _, hit := range hits {
				assetID := uuid.MustParse(hit.AssetID)
				if _, ok := allowed[assetID]; !ok {
					continue
				}
				candidates = append(candidates, Candidate{
					AssetID:  assetID,
					Source:   SourceOCR,
					Rank:     len(candidates) + 1,
					RawScore: hit.Score,
				})
				if len(candidates) == req.TopK {
					break
				}
			}

			from += len(page.Hits)
			if uint64(from) >= page.Total {
				break
			}
		}
		if len(candidates) >= req.TopK {
			break
		}
	}
	return candidates, nil
}

func (r *BleveOCRRetriever) filterCandidates(ctx context.Context, hits []bleveocr.Hit, filter Filter) (map[uuid.UUID]struct{}, error) {
	allowed := make(map[uuid.UUID]struct{}, len(hits))
	if len(hits) == 0 {
		return allowed, nil
	}
	ids := make([]uuid.UUID, 0, len(hits))
	for _, hit := range hits {
		assetID, err := uuid.Parse(hit.AssetID)
		if err != nil {
			return nil, fmt.Errorf("parse OCR Bleve candidate %q: %w", hit.AssetID, err)
		}
		ids = append(ids, assetID)
	}

	builder := &sqlBuilder{}
	idsPlaceholder, err := builder.addJSONArg(ids)
	if err != nil {
		return nil, err
	}
	conditions, err := buildAssetFilterConditions(builder, filter, "a")
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, fmt.Sprintf(
		"a.asset_id IN (SELECT value FROM json_each(%s))",
		idsPlaceholder,
	))
	query := fmt.Sprintf("SELECT a.asset_id FROM assets a WHERE %s", joinConditions(conditions))
	rows, err := r.pool.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return nil, fmt.Errorf("post-filter OCR Bleve candidates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var assetID uuid.UUID
		if err := rows.Scan(&assetID); err != nil {
			return nil, fmt.Errorf("scan post-filtered OCR candidate: %w", err)
		}
		allowed[assetID] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate post-filtered OCR candidates: %w", err)
	}
	return allowed, nil
}

func collectCandidates(rows *sql.Rows, source string) ([]Candidate, error) {
	candidates := []Candidate{}
	rank := 1
	for rows.Next() {
		var assetID uuid.UUID
		var rawScore float64
		var bestTs *int32
		if err := rows.Scan(&assetID, &rawScore, &bestTs); err != nil {
			return nil, fmt.Errorf("scan %s candidate: %w", source, err)
		}
		candidates = append(candidates, Candidate{
			AssetID:  assetID,
			Source:   source,
			Rank:     rank,
			RawScore: rawScore,
			BestTsMs: bestTs,
		})
		rank++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s candidates: %w", source, err)
	}
	return candidates, nil
}

func HydrateAssets(ctx context.Context, pool *sql.DB, rankedIDs []uuid.UUID, includeDeleted bool) ([]repo.Asset, error) {
	if len(rankedIDs) == 0 {
		return []repo.Asset{}, nil
	}
	ids := make([]uuid.UUID, 0, len(rankedIDs))
	for _, id := range rankedIDs {
		if id == uuid.Nil {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return []repo.Asset{}, nil
	}

	queries := repo.New(pool)
	var assets []repo.Asset
	var err error
	if includeDeleted {
		assets, err = queries.GetAssetsByIDsAny(ctx, ids)
	} else {
		assets, err = queries.GetAssetsByIDs(ctx, ids)
	}
	if err != nil {
		return nil, fmt.Errorf("decode ranked assets: %w", err)
	}

	byID := make(map[uuid.UUID]repo.Asset, len(assets))
	for _, asset := range assets {
		byID[asset.AssetID] = asset
	}

	ordered := make([]repo.Asset, 0, len(rankedIDs))
	for _, id := range rankedIDs {
		if asset, ok := byID[id]; ok {
			ordered = append(ordered, asset)
		}
	}
	return ordered, nil
}

func ftsMatchQuery(raw string) string {
	fields := strings.Fields(TokenizeQuery(raw))
	if len(fields) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(fields))
	for _, field := range fields {
		quoted = append(quoted, `"`+strings.ReplaceAll(field, `"`, `""`)+`"`)
	}
	return strings.Join(quoted, " AND ")
}

func hasTextQuery(req Request) bool {
	return strings.TrimSpace(req.Query) != ""
}
