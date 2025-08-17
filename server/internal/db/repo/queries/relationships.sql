-- name: GetAssetWithThumbnails :one
SELECT
    a.*,
    COALESCE(
        json_agg(
            json_build_object(
                'thumbnail_id', t.thumbnail_id,
                'size', t.size,
                'storage_path', t.storage_path,
                'mime_type', t.mime_type,
                'created_at', t.created_at
            ) ORDER BY
            CASE t.size
                WHEN 'small' THEN 1
                WHEN 'medium' THEN 2
                WHEN 'large' THEN 3
            END
        ) FILTER (WHERE t.thumbnail_id IS NOT NULL),
        '[]'
    ) as thumbnails
FROM assets a
LEFT JOIN thumbnails t ON a.asset_id = t.asset_id
WHERE a.asset_id = $1 AND a.is_deleted = false
GROUP BY a.asset_id;

-- name: GetAssetWithTags :one
SELECT
    a.*,
    COALESCE(
        json_agg(
            json_build_object(
                'tag_id', tg.tag_id,
                'tag_name', tg.tag_name,
                'category', tg.category,
                'confidence', at.confidence,
                'source', at.source
            )
        ) FILTER (WHERE tg.tag_id IS NOT NULL),
        '[]'
    ) as tags
FROM assets a
LEFT JOIN asset_tags at ON a.asset_id = at.asset_id
LEFT JOIN tags tg ON at.tag_id = tg.tag_id
WHERE a.asset_id = $1 AND a.is_deleted = false
GROUP BY a.asset_id;

-- name: GetAssetWithRelations :one
SELECT
    a.*,
    COALESCE(
        json_agg(DISTINCT
            json_build_object(
                'thumbnail_id', t.thumbnail_id,
                'size', t.size,
                'storage_path', t.storage_path,
                'mime_type', t.mime_type
            )
        ) FILTER (WHERE t.thumbnail_id IS NOT NULL),
        '[]'
    ) as thumbnails,
    COALESCE(
        json_agg(DISTINCT
            json_build_object(
                'tag_id', tg.tag_id,
                'tag_name', tg.tag_name,
                'confidence', at.confidence
            )
        ) FILTER (WHERE tg.tag_id IS NOT NULL),
        '[]'
    ) as tags
FROM assets a
LEFT JOIN thumbnails t ON a.asset_id = t.asset_id
LEFT JOIN asset_tags at ON a.asset_id = at.asset_id
LEFT JOIN tags tg ON at.tag_id = tg.tag_id
WHERE a.asset_id = $1 AND a.is_deleted = false
GROUP BY a.asset_id;
