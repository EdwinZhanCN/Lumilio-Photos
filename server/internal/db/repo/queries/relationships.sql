-- name: GetAssetWithThumbnails :one
SELECT
    a.*,
    COALESCE((
        SELECT json_group_array(
            json_object(
                'thumbnail_id', ordered.thumbnail_id,
                'size', ordered.size,
                'storage_path', ordered.storage_path,
                'mime_type', ordered.mime_type,
                'created_at', ordered.created_at
            )
        )
        FROM (
            SELECT t.*
            FROM thumbnails t
            WHERE t.asset_id = a.asset_id
            ORDER BY CASE t.size
                WHEN 'small' THEN 1
                WHEN 'medium' THEN 2
                WHEN 'large' THEN 3
            END, t.thumbnail_id
        ) AS ordered
    ), '[]') as thumbnails
FROM assets a
WHERE a.asset_id = ?1;

-- name: GetAssetWithTags :one
SELECT
    a.*,
    COALESCE((
        SELECT json_group_array(
            json_object(
                'tag_id', ordered.tag_id,
                'tag_name', ordered.tag_name,
                'category', ordered.category,
                'confidence', ordered.confidence,
                'source', ordered.source
            )
        )
        FROM (
            SELECT tg.tag_id, tg.tag_name, tg.category, at.confidence, at.source
            FROM asset_tags at
            JOIN tags tg ON at.tag_id = tg.tag_id
            WHERE at.asset_id = a.asset_id
            ORDER BY tg.tag_name, tg.tag_id
        ) AS ordered
    ), '[]') as tags
FROM assets a
WHERE a.asset_id = ?1;

-- name: GetAssetWithRelations :one
SELECT
    a.*,
    content.full_hash AS content_hash,
    content.file_size,
    COALESCE((
        SELECT json_group_array(json_object(
            'thumbnail_id', ordered.thumbnail_id,
            'size', ordered.size,
            'storage_path', ordered.storage_path,
            'mime_type', ordered.mime_type
        ))
        FROM (
            SELECT t.*
            FROM thumbnails t
            WHERE t.asset_id = a.asset_id
            ORDER BY CASE t.size
                WHEN 'small' THEN 1
                WHEN 'medium' THEN 2
                WHEN 'large' THEN 3
            END, t.thumbnail_id
        ) AS ordered
    ), '[]') AS thumbnails,
    COALESCE((
        SELECT json_group_array(json_object(
            'tag_id', ordered.tag_id,
            'tag_name', ordered.tag_name,
            'confidence', ordered.confidence,
            'source', ordered.source
        ))
        FROM (
            SELECT tg.tag_id, tg.tag_name, at.confidence, at.source
            FROM asset_tags at
            JOIN tags tg ON at.tag_id = tg.tag_id
            WHERE at.asset_id = a.asset_id
            ORDER BY tg.tag_name, tg.tag_id
        ) AS ordered
    ), '[]') AS tags,
    COALESCE((
        SELECT json_group_array(json_object(
            'album_id', ordered.album_id,
            'album_name', ordered.album_name,
            'position', ordered.position,
            'added_time', ordered.added_time
        ))
        FROM (
            SELECT al.album_id, al.album_name, aa.position, aa.added_time
            FROM album_assets aa
            JOIN albums al ON aa.album_id = al.album_id
            WHERE aa.asset_id = a.asset_id
            ORDER BY aa.position IS NULL, aa.position, aa.added_time, al.album_id
        ) AS ordered
    ), '[]') AS albums,
    COALESCE((
        SELECT json_group_array(json_object('label', ordered.label, 'score', ordered.score))
        FROM (
            SELECT sp.label, sp.score
            FROM species_predictions sp
            WHERE sp.asset_id = a.asset_id
            ORDER BY sp.score DESC, sp.label
        ) AS ordered
    ), '[]') AS species_predictions,
    (
        SELECT json_object(
            'model_id', ocr.model_id,
            'total_count', ocr.total_count,
            'processing_time_ms', ocr.processing_time_ms,
            'created_at', ocr.created_at,
            'updated_at', ocr.updated_at,
            'text_items', COALESCE((
                SELECT json_group_array(json_object(
                    'id', ordered.id,
                    'text_content', ordered.text_content,
                    'confidence', ordered.confidence,
                    'bounding_box', json(ordered.bounding_box),
                    'text_length', ordered.text_length,
                    'area_pixels', ordered.area_pixels
                ))
                FROM (
                    SELECT ocr_ti.*
                    FROM ocr_text_items ocr_ti
                    WHERE ocr_ti.asset_id = a.asset_id
                    ORDER BY ocr_ti.id ASC
                ) AS ordered
            ), '[]')
        )
        FROM ocr_results ocr
        WHERE ocr.asset_id = a.asset_id
    ) AS ocr_result,
    (
        SELECT json_object(
            'model_id', fr.model_id,
            'total_faces', fr.total_faces,
            'processing_time_ms', fr.processing_time_ms,
            'created_at', fr.created_at,
            'updated_at', fr.updated_at,
            'faces', COALESCE((
                SELECT json_group_array(json_object(
                    'id', ordered.id,
                    'face_id', ordered.face_id,
                    'bounding_box', json(ordered.bounding_box),
                    'confidence', ordered.confidence,
                    'age_group', ordered.age_group,
                    'gender', ordered.gender,
                    'ethnicity', ordered.ethnicity,
                    'expression', ordered.expression,
                    'is_primary', ordered.is_primary,
                    'cluster_id', ordered.cluster_id,
                    'cluster_name', ordered.cluster_name
                ))
                FROM (
                    SELECT fi.*, fcm.cluster_id, fc.cluster_name
                    FROM face_items fi
                    LEFT JOIN face_cluster_members fcm ON fi.id = fcm.face_id
                    LEFT JOIN face_clusters fc ON fcm.cluster_id = fc.cluster_id
                    WHERE fi.asset_id = a.asset_id
                    ORDER BY fi.is_primary DESC, fi.confidence DESC, fi.id
                ) AS ordered
            ), '[]')
        )
        FROM face_results fr
        WHERE fr.asset_id = a.asset_id
    ) AS face_result
FROM assets a
JOIN content_objects content ON content.content_id = a.content_id
WHERE a.asset_id = ?1;
