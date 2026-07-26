-- name: AgentFacetQualityStats :one
-- Aesthetic-score distribution over a ref snapshot. SQLite does not ship a
-- percentile aggregate, so choose the first value at each discrete rank.
WITH ranked AS (
    SELECT
        score,
        ROW_NUMBER() OVER (ORDER BY score) AS rank_number,
        COUNT(*) OVER () AS total
    FROM asset_quality_scores
    WHERE asset_id IN (sqlc.slice('asset_ids'))
)
SELECT
    COUNT(*) AS scored_count,
    COALESCE(MIN(CASE WHEN rank_number >= (total + 3) / 4 THEN score END), 0) AS p25,
    COALESCE(MIN(CASE WHEN rank_number >= (total + 1) / 2 THEN score END), 0) AS p50,
    COALESCE(MIN(CASE WHEN rank_number >= (3 * total + 3) / 4 THEN score END), 0) AS p75,
    COALESCE(MIN(CASE WHEN rank_number >= (9 * total + 9) / 10 THEN score END), 0) AS p90
FROM ranked;
