package search

import (
	"context"
	"fmt"
	"strings"

	"server/internal/db/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

type EmbeddingRetriever struct {
	pool         *pgxpool.Pool
	embed        EmbedQueryFunc
	resolveSpace ResolveEmbeddingSpaceFunc
	weight       float64
}

func NewEmbeddingRetriever(pool *pgxpool.Pool, embed EmbedQueryFunc, resolveSpace ResolveEmbeddingSpaceFunc, weight float64) *EmbeddingRetriever {
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
	queryVector := pgvector.NewVector(embedding.Vector)
	vectorPlaceholder := builder.addArg(&queryVector)
	spacePlaceholder := builder.addArg(space.ID)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return nil, err
	}
	conditions = append(conditions,
		fmt.Sprintf("e.space_id = %s", spacePlaceholder),
	)
	// L2 distance over the fixed-dimension search_embeddings table (vectors are
	// unit-length, so L2 ranks identically to cosine). Assets may carry multiple
	// vectors (video frames), so rank each asset by its best (nearest) frame via
	// MIN — a max-pool over the per-asset frame set.
	distanceExpr := fmt.Sprintf("(e.vector::vector(%d) <-> %s::vector(%d))", space.Dimensions, vectorPlaceholder, space.Dimensions)
	limitPlaceholder := builder.addArg(req.TopK)

	query := fmt.Sprintf(`
SELECT
  a.asset_id,
  MIN(%s)::float8 AS raw_score,
  (array_agg(e.frame_ts_ms ORDER BY %s ASC NULLS LAST))[1] AS best_ts
FROM search_embeddings e
JOIN assets a ON a.asset_id = e.asset_id
WHERE %s
GROUP BY a.asset_id
ORDER BY raw_score, a.asset_id DESC
LIMIT %s
`, distanceExpr, distanceExpr, joinConditions(conditions), limitPlaceholder)

	rows, err := r.pool.Query(ctx, query, builder.args...)
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
	pool   *pgxpool.Pool
	source string
	weight float64
}

func NewOCRRetriever(pool *pgxpool.Pool, weight float64) *TextRetriever {
	return &TextRetriever{pool: pool, source: SourceOCR, weight: weight}
}

func NewPlaceRetriever(pool *pgxpool.Pool, weight float64) *TextRetriever {
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
	tokenized := TokenizeQuery(req.Query)
	if tokenized == "" {
		return nil, nil
	}

	builder := &sqlBuilder{}
	queryPlaceholder := builder.addArg(tokenized)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return nil, err
	}
	conditions = append(conditions,
		fmt.Sprintf("r.full_text %%> %s", queryPlaceholder))
	limitPlaceholder := builder.addArg(req.TopK)

	query := fmt.Sprintf(`
SELECT
  a.asset_id,
  word_similarity(%s, r.full_text)::float8 AS raw_score,
  NULL::int AS best_ts
FROM ocr_results r
JOIN assets a ON a.asset_id = r.asset_id
WHERE %s
ORDER BY raw_score DESC, a.asset_id DESC
LIMIT %s
`, queryPlaceholder, joinConditions(conditions), limitPlaceholder)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ocr retrieve begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "SET LOCAL pg_trgm.word_similarity_threshold = 0.15"); err != nil {
		return nil, fmt.Errorf("ocr retrieve set threshold: %w", err)
	}

	rows, err := tx.Query(ctx, query, builder.args...)
	if err != nil {
		return nil, fmt.Errorf("ocr retrieve: %w", err)
	}
	defer rows.Close()

	candidates, err := collectCandidates(rows, SourceOCR)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("ocr retrieve commit: %w", err)
	}
	return candidates, nil
}

func (r *TextRetriever) ocrCountQuery(builder *sqlBuilder, req Request) (string, error) {
	tokenized := TokenizeQuery(req.Query)
	if tokenized == "" {
		return "SELECT NULL::uuid AS asset_id WHERE false", nil
	}

	queryPlaceholder := builder.addArg(tokenized)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return "", err
	}
	conditions = append(conditions,
		fmt.Sprintf("r.full_text %%> %s", queryPlaceholder))

	return fmt.Sprintf(`
SELECT a.asset_id
FROM ocr_results r
JOIN assets a ON a.asset_id = r.asset_id
WHERE %s
`, joinConditions(conditions)), nil
}

func (r *TextRetriever) retrievePlace(ctx context.Context, req Request) ([]Candidate, error) {
	builder := &sqlBuilder{}
	queryPlaceholder := builder.addArg(req.Query)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, "lc.search_vector @@ q.query")
	limitPlaceholder := builder.addArg(req.TopK)

	query := fmt.Sprintf(`
WITH q AS (SELECT plainto_tsquery('simple', %s) AS query)
SELECT
  a.asset_id,
  MAX(ts_rank_cd(lc.search_vector, q.query))::float8 AS raw_score,
  NULL::int AS best_ts
FROM q
JOIN location_clusters lc ON lc.search_vector @@ q.query
JOIN location_cluster_assets lca ON lca.cluster_id = lc.cluster_id
JOIN assets a ON a.asset_id = lca.asset_id
WHERE %s
GROUP BY a.asset_id
ORDER BY raw_score DESC, a.asset_id DESC
LIMIT %s
`, queryPlaceholder, joinConditions(conditions), limitPlaceholder)

	rows, err := r.pool.Query(ctx, query, builder.args...)
	if err != nil {
		return nil, fmt.Errorf("place retrieve: %w", err)
	}
	defer rows.Close()

	return collectCandidates(rows, SourcePlace)
}

func (r *TextRetriever) placeCountQuery(builder *sqlBuilder, req Request) (string, error) {
	queryPlaceholder := builder.addArg(req.Query)
	conditions, err := buildAssetFilterConditions(builder, req.Filter, "a")
	if err != nil {
		return "", err
	}
	conditions = append(conditions, fmt.Sprintf("lc.search_vector @@ plainto_tsquery('simple', %s)", queryPlaceholder))

	return fmt.Sprintf(`
SELECT a.asset_id
FROM location_clusters lc
JOIN location_cluster_assets lca ON lca.cluster_id = lc.cluster_id
JOIN assets a ON a.asset_id = lca.asset_id
WHERE %s
`, joinConditions(conditions)), nil
}

func collectCandidates(rows pgx.Rows, source string) ([]Candidate, error) {
	candidates := []Candidate{}
	rank := 1
	for rows.Next() {
		var assetID pgtype.UUID
		var rawScore float64
		var bestTs *int32
		if err := rows.Scan(&assetID, &rawScore, &bestTs); err != nil {
			return nil, fmt.Errorf("scan %s candidate: %w", source, err)
		}
		if !assetID.Valid {
			continue
		}
		candidates = append(candidates, Candidate{
			AssetID:  uuid.UUID(assetID.Bytes),
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

func HydrateAssets(ctx context.Context, pool *pgxpool.Pool, rankedIDs []uuid.UUID, includeDeleted bool) ([]repo.Asset, error) {
	if len(rankedIDs) == 0 {
		return []repo.Asset{}, nil
	}
	pgIDs := make([]pgtype.UUID, 0, len(rankedIDs))
	for _, id := range rankedIDs {
		if id == uuid.Nil {
			continue
		}
		pgIDs = append(pgIDs, pgtype.UUID{Bytes: id, Valid: true})
	}
	if len(pgIDs) == 0 {
		return []repo.Asset{}, nil
	}

	query := `
SELECT a.*
FROM assets a
WHERE a.asset_id = ANY($1::uuid[])`
	if !includeDeleted {
		query += `
  AND a.is_deleted = false`
	}

	rows, err := pool.Query(ctx, query, pgIDs)
	if err != nil {
		return nil, fmt.Errorf("hydrate ranked assets: %w", err)
	}
	defer rows.Close()

	assets, err := pgx.CollectRows(rows, pgx.RowToStructByName[repo.Asset])
	if err != nil {
		return nil, fmt.Errorf("decode ranked assets: %w", err)
	}

	byID := make(map[uuid.UUID]repo.Asset, len(assets))
	for _, asset := range assets {
		if asset.AssetID.Valid {
			byID[uuid.UUID(asset.AssetID.Bytes)] = asset
		}
	}

	ordered := make([]repo.Asset, 0, len(rankedIDs))
	for _, id := range rankedIDs {
		if asset, ok := byID[id]; ok {
			ordered = append(ordered, asset)
		}
	}
	return ordered, nil
}

func hasTextQuery(req Request) bool {
	return strings.TrimSpace(req.Query) != ""
}
