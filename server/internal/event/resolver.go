package event

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("event not found")

type Summary struct {
	EventID                 string
	RedirectedFrom          string
	StartAt                 int64
	EndAt                   int64
	Timezone                *string
	TitleOverride           *string
	GeneratedCoverMediaItem *string
	CoverOverrideMediaItem  *string
	CoverMediaItem          *string
	CoverAssetID            *string
	Hidden                  bool
	MediaCount              int
	DisplayableCount        int
	CanonicalMediaCount     int
	ProjectedMediaCount     int
}

type ResolvedAsset struct {
	Position    int64
	MediaItemID string
	AssetID     string
}

type Resolver struct {
	db *sql.DB
}

func NewResolver(db *sql.DB) *Resolver { return &Resolver{db: db} }

func (r *Resolver) Resolve(ctx context.Context, ownerID int32, eventID string) (Summary, error) {
	return resolveWith(ctx, r.db, ownerID, eventID)
}

// ProjectToRepository turns an owner-scoped Event summary into the read-only
// representation used by repository-scoped lists. Event identity and user
// state remain owner-scoped; membership counts and the cover are selected only
// from media in repositoryID.
func (r *Resolver) ProjectToRepository(ctx context.Context, ownerID int32, summary Summary, repositoryID string) (Summary, error) {
	const query = `
WITH scoped_members AS (
  SELECT emi.media_item_id, emi.position,
         COALESCE(
           CASE WHEN primary_asset.is_deleted = 0 THEN primary_asset.asset_id END,
           (
             SELECT a.asset_id
             FROM media_item_assets mia
             JOIN assets a ON a.asset_id = mia.asset_id
             WHERE mia.media_item_id = emi.media_item_id
               AND a.owner_id = emi.owner_id AND a.is_deleted = 0
             ORDER BY mia.position, mia.asset_id
             LIMIT 1
           )
         ) AS representative_asset_id
  FROM event_media_items emi
  JOIN media_items mi
    ON mi.media_item_id = emi.media_item_id AND mi.owner_id = emi.owner_id
  LEFT JOIN assets primary_asset
    ON primary_asset.asset_id = mi.primary_asset_id AND primary_asset.owner_id = emi.owner_id
  WHERE emi.event_id = ? AND emi.owner_id = ? AND mi.repository_id = ?
),
scoped_cover AS (
  SELECT sm.media_item_id, sm.representative_asset_id
  FROM scoped_members sm
  WHERE sm.representative_asset_id IS NOT NULL
  ORDER BY CASE WHEN sm.media_item_id = ? THEN 0 ELSE 1 END,
           sm.position, sm.media_item_id
  LIMIT 1
)
SELECT (SELECT media_item_id FROM scoped_cover),
       (SELECT representative_asset_id FROM scoped_cover),
       count(sm.media_item_id),
       count(sm.representative_asset_id)
FROM scoped_members sm
`
	preferredCover := ""
	if summary.CoverMediaItem != nil {
		preferredCover = *summary.CoverMediaItem
	}
	if err := r.db.QueryRowContext(
		ctx,
		query,
		summary.EventID,
		ownerID,
		repositoryID,
		preferredCover,
	).Scan(
		&summary.CoverMediaItem,
		&summary.CoverAssetID,
		&summary.MediaCount,
		&summary.DisplayableCount,
	); err != nil {
		return Summary{}, fmt.Errorf("project Event to repository: %w", err)
	}
	summary.ProjectedMediaCount = summary.MediaCount
	if summary.MediaCount == 0 {
		return Summary{}, ErrNotFound
	}
	return summary, nil
}

func ResolveTx(ctx context.Context, tx *sql.Tx, ownerID int32, eventID string) (Summary, error) {
	return resolveWith(ctx, tx, ownerID, eventID)
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func resolveWith(ctx context.Context, db queryer, ownerID int32, eventID string) (Summary, error) {
	const query = `
WITH resolved AS (
  SELECT CASE WHEN e.status = 'redirected' THEN er.new_event_id ELSE e.event_id END AS event_id,
         CASE WHEN e.status = 'redirected' THEN e.event_id ELSE NULL END AS redirected_from
  FROM events e
  LEFT JOIN event_redirects er
    ON er.old_event_id = e.event_id AND er.owner_id = e.owner_id
  WHERE e.event_id = ? AND e.owner_id = ?
), member_assets AS (
  SELECT emi.event_id, emi.owner_id, emi.media_item_id, emi.position,
         COALESCE(
           CASE WHEN primary_asset.is_deleted = 0 THEN primary_asset.asset_id END,
           (
             SELECT a.asset_id
             FROM media_item_assets mia
             JOIN assets a ON a.asset_id = mia.asset_id
             WHERE mia.media_item_id = emi.media_item_id
               AND a.owner_id = emi.owner_id AND a.is_deleted = 0
             ORDER BY mia.position, mia.asset_id
             LIMIT 1
           )
         ) AS representative_asset_id
  FROM event_media_items emi
  JOIN media_items mi
    ON mi.media_item_id = emi.media_item_id AND mi.owner_id = emi.owner_id
  LEFT JOIN assets primary_asset
    ON primary_asset.asset_id = mi.primary_asset_id AND primary_asset.owner_id = emi.owner_id
  WHERE emi.event_id = (SELECT event_id FROM resolved)
    AND emi.owner_id = ?
)
SELECT e.event_id, resolved.redirected_from, e.start_at, e.end_at, e.timezone,
       e.title_override,
       e.generated_cover_media_item_id,
       e.cover_override_media_item_id,
       COALESCE(e.cover_override_media_item_id, e.generated_cover_media_item_id),
       (
         SELECT representative_asset_id
         FROM member_assets
         WHERE representative_asset_id IS NOT NULL
         ORDER BY CASE WHEN media_item_id=COALESCE(
           e.cover_override_media_item_id,e.generated_cover_media_item_id
         ) THEN 0 ELSE 1 END,position,media_item_id
         LIMIT 1
       ),
       e.is_hidden,
       count(member_assets.media_item_id),
       count(member_assets.representative_asset_id)
FROM resolved
JOIN events e ON e.event_id = resolved.event_id AND e.owner_id = ?
LEFT JOIN member_assets
  ON member_assets.event_id = e.event_id AND member_assets.owner_id = e.owner_id
WHERE e.status = 'active'
GROUP BY e.event_id`
	var summary Summary
	var redirectedFrom sql.NullString
	if err := db.QueryRowContext(ctx, query, eventID, ownerID, ownerID, ownerID).Scan(
		&summary.EventID, &redirectedFrom, &summary.StartAt, &summary.EndAt,
		&summary.Timezone, &summary.TitleOverride, &summary.GeneratedCoverMediaItem,
		&summary.CoverOverrideMediaItem, &summary.CoverMediaItem,
		&summary.CoverAssetID, &summary.Hidden, &summary.MediaCount, &summary.DisplayableCount,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Summary{}, ErrNotFound
		}
		return Summary{}, fmt.Errorf("resolve Event: %w", err)
	}
	summary.RedirectedFrom = redirectedFrom.String
	summary.CanonicalMediaCount = summary.MediaCount
	summary.ProjectedMediaCount = summary.MediaCount
	return summary, nil
}

func (r *Resolver) OrderedAssets(ctx context.Context, ownerID int32, eventID string, limit int) ([]ResolvedAsset, int, error) {
	return orderedAssetsWith(ctx, r.db, ownerID, eventID, limit)
}

// OrderedAssetsForRepository applies Browse Scope after resolving the
// owner-wide Event.  It is intentionally a projection filter and never feeds
// back into reconciliation or Event identity.
func (r *Resolver) OrderedAssetsForRepository(ctx context.Context, ownerID int32, eventID, repositoryID string, limit int) ([]ResolvedAsset, int, error) {
	return orderedAssetsScopedWith(ctx, r.db, ownerID, eventID, repositoryID, limit)
}

func OrderedAssetsTx(ctx context.Context, tx *sql.Tx, ownerID int32, eventID string, limit int) ([]ResolvedAsset, int, error) {
	return orderedAssetsWith(ctx, tx, ownerID, eventID, limit)
}

func orderedAssetsWith(ctx context.Context, db queryer, ownerID int32, eventID string, limit int) ([]ResolvedAsset, int, error) {
	return orderedAssetsScopedWith(ctx, db, ownerID, eventID, "", limit)
}

func orderedAssetsScopedWith(ctx context.Context, db queryer, ownerID int32, eventID, repositoryID string, limit int) ([]ResolvedAsset, int, error) {
	summary, err := resolveWith(ctx, db, ownerID, eventID)
	if err != nil {
		return nil, 0, err
	}
	const query = `
SELECT emi.position, emi.media_item_id,
       COALESCE(
         CASE WHEN primary_asset.is_deleted = 0 THEN primary_asset.asset_id END,
         (
           SELECT a.asset_id
           FROM media_item_assets mia
           JOIN assets a ON a.asset_id = mia.asset_id
           WHERE mia.media_item_id = emi.media_item_id
             AND a.owner_id = emi.owner_id AND a.is_deleted = 0
           ORDER BY mia.position, mia.asset_id
           LIMIT 1
         )
       ) AS asset_id
FROM event_media_items emi
JOIN media_items mi
  ON mi.media_item_id = emi.media_item_id AND mi.owner_id = emi.owner_id
LEFT JOIN assets primary_asset
  ON primary_asset.asset_id = mi.primary_asset_id AND primary_asset.owner_id = emi.owner_id
WHERE emi.event_id = ? AND emi.owner_id = ?
  AND (? = '' OR mi.repository_id = ?)
ORDER BY emi.position, emi.media_item_id`
	rows, err := db.QueryContext(ctx, query, summary.EventID, ownerID, repositoryID, repositoryID)
	if err != nil {
		return nil, 0, fmt.Errorf("resolve Event assets: %w", err)
	}
	defer rows.Close()
	var result []ResolvedAsset
	omitted := 0
	for rows.Next() {
		var row ResolvedAsset
		var assetID sql.NullString
		if err := rows.Scan(&row.Position, &row.MediaItemID, &assetID); err != nil {
			return nil, 0, err
		}
		if !assetID.Valid {
			omitted++
			continue
		}
		row.AssetID = assetID.String
		if limit <= 0 || len(result) < limit {
			result = append(result, row)
		}
	}
	return result, omitted, rows.Err()
}
