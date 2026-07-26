package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/google/uuid"
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
	if r == nil || r.pool == nil || r.embed == nil || r.resolveSpace == nil {
		return nil, fmt.Errorf("embedding retriever is not configured")
	}

	embedding, space, err := r.resolveQuerySpace(ctx, req)
	if err != nil {
		return nil, err
	}

	builder := &sqlBuilder{}
	queryVector := dbtypes.NewVector(embedding.Vector)
	vectorPlaceholder := builder.addArg(queryVector)
	knnLimit := req.TopK * 8
	if knnLimit < req.TopK {
		knnLimit = req.TopK
	}
	knnLimitPlaceholder := builder.addArg(knnLimit)
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
  SELECT rowid, distance
  FROM search_embeddings_vec
  WHERE embedding MATCH %s
    AND k = %s
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
`, vectorPlaceholder, knnLimitPlaceholder, joinConditions(conditions), limitPlaceholder)

	rows, err := r.pool.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return nil, fmt.Errorf("embedding retrieve: %w", err)
	}
	defer rows.Close()

	return collectCandidates(rows, SourceEmbedding)
}

func (r *EmbeddingRetriever) CountQuery(ctx context.Context, builder *sqlBuilder, req Request) (string, error) {
	if r == nil || r.pool == nil || r.embed == nil || r.resolveSpace == nil {
		return "", fmt.Errorf("embedding retriever is not configured")
	}

	_, space, err := r.resolveQuerySpace(ctx, req)
	if err != nil {
		return "", err
	}

	spacePlaceholder := builder.addArg(space.ID)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return "", err
	}
	conditions = append(conditions,
		fmt.Sprintf("e.space_id = %s", spacePlaceholder),
	)

	return fmt.Sprintf(`
SELECT a.asset_id
FROM search_embeddings e
JOIN assets a ON a.asset_id = e.asset_id
WHERE %s
GROUP BY a.asset_id
`, joinConditions(conditions)), nil
}

func (r *EmbeddingRetriever) resolveQuerySpace(ctx context.Context, req Request) (QueryEmbedding, repo.EmbeddingSpace, error) {
	embedding, err := r.embed(ctx, req.Query, true)
	if err != nil {
		return QueryEmbedding{}, repo.EmbeddingSpace{}, err
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

func NewOCRRetriever(pool *sql.DB, weight float64) *TextRetriever {
	return &TextRetriever{pool: pool, source: SourceOCR, weight: weight}
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
	case SourceOCR:
		return r.retrieveOCR(ctx, req)
	case SourcePlace:
		return r.retrievePlace(ctx, req)
	default:
		return nil, fmt.Errorf("unknown text retriever source: %s", r.source)
	}
}

func (r *TextRetriever) CountQuery(ctx context.Context, builder *sqlBuilder, req Request) (string, error) {
	if r == nil || r.pool == nil {
		return "", fmt.Errorf("%s retriever is not configured", r.source)
	}
	switch r.source {
	case SourceOCR:
		return r.ocrCountQuery(builder, req)
	case SourcePlace:
		return r.placeCountQuery(builder, req)
	default:
		return "", fmt.Errorf("unknown text retriever source: %s", r.source)
	}
}

func (r *TextRetriever) retrieveOCR(ctx context.Context, req Request) ([]Candidate, error) {
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
	conditions = append(conditions, fmt.Sprintf("ocr_search_fts MATCH %s", queryPlaceholder))
	limitPlaceholder := builder.addArg(req.TopK)

	query := fmt.Sprintf(`
SELECT
  a.asset_id,
  -bm25(ocr_search_fts) AS raw_score,
  NULL AS best_ts
FROM ocr_results r
JOIN ocr_search_fts ON ocr_search_fts.rowid = r.rowid
JOIN assets a ON a.asset_id = r.asset_id
WHERE %s
ORDER BY raw_score DESC, a.asset_id DESC
LIMIT %s
`, joinConditions(conditions), limitPlaceholder)

	rows, err := r.pool.QueryContext(ctx, query, builder.args...)
	if err != nil {
		return nil, fmt.Errorf("ocr retrieve: %w", err)
	}
	defer rows.Close()

	candidates, err := collectCandidates(rows, SourceOCR)
	if err != nil {
		return nil, err
	}

	return candidates, nil
}

func (r *TextRetriever) ocrCountQuery(builder *sqlBuilder, req Request) (string, error) {
	matchQuery := ftsMatchQuery(req.Query)
	if matchQuery == "" {
		return "SELECT NULL AS asset_id WHERE false", nil
	}

	queryPlaceholder := builder.addArg(matchQuery)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return "", err
	}
	conditions = append(conditions, fmt.Sprintf("ocr_search_fts MATCH %s", queryPlaceholder))

	return fmt.Sprintf(`
SELECT a.asset_id
FROM ocr_results r
JOIN ocr_search_fts ON ocr_search_fts.rowid = r.rowid
JOIN assets a ON a.asset_id = r.asset_id
WHERE %s
`, joinConditions(conditions)), nil
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

func (r *TextRetriever) placeCountQuery(builder *sqlBuilder, req Request) (string, error) {
	matchQuery := ftsMatchQuery(req.Query)
	if matchQuery == "" {
		return "SELECT NULL AS asset_id WHERE false", nil
	}
	queryPlaceholder := builder.addArg(matchQuery)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return "", err
	}
	conditions = append(conditions, fmt.Sprintf("location_search_fts MATCH %s", queryPlaceholder))

	return fmt.Sprintf(`
SELECT a.asset_id
FROM location_search_fts
JOIN location_clusters lc ON lc.rowid = location_search_fts.rowid
JOIN location_cluster_assets lca ON lca.cluster_id = lc.cluster_id
JOIN assets a ON a.asset_id = lca.asset_id
WHERE %s
`, joinConditions(conditions)), nil
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
