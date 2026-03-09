-- Remove duplicate thumbnail rows before enforcing uniqueness.
WITH ranked AS (
    SELECT
        thumbnail_id,
        ROW_NUMBER() OVER (
            PARTITION BY asset_id, size
            ORDER BY created_at DESC, thumbnail_id DESC
        ) AS row_num
    FROM thumbnails
)
DELETE FROM thumbnails
WHERE thumbnail_id IN (
    SELECT thumbnail_id
    FROM ranked
    WHERE row_num > 1
);

ALTER TABLE thumbnails
ADD CONSTRAINT thumbnails_asset_id_size_key UNIQUE (asset_id, size);
