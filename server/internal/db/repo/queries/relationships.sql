-- name: GetAssetWithThumbnails :one
SELECT
    a.*,
    COALESCE((
        SELECT json_agg(
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
        )
        FROM thumbnails t
        WHERE t.asset_id = a.asset_id
    ), '[]'::json) as thumbnails
FROM assets a
WHERE a.asset_id = $1 AND a.is_deleted = false;

-- name: GetAssetWithTags :one
SELECT
    a.*,
    COALESCE((
        SELECT json_agg(
            json_build_object(
                'tag_id', tg.tag_id,
                'tag_name', tg.tag_name,
                'category', tg.category,
                'confidence', at.confidence,
                'source', at.source
            )
            ORDER BY tg.tag_name ASC, tg.tag_id ASC
        )
        FROM asset_tags at
        JOIN tags tg ON at.tag_id = tg.tag_id
        WHERE at.asset_id = a.asset_id
    ), '[]'::json) as tags
FROM assets a
WHERE a.asset_id = $1 AND a.is_deleted = false;

-- name: GetAssetWithRelations :one
SELECT
    a.*,
    COALESCE(thumbnails_rel.thumbnails, '[]'::json) as thumbnails,
    COALESCE(tags_rel.tags, '[]'::json) as tags,
    COALESCE(albums_rel.albums, '[]'::json) as albums,
    COALESCE(species_rel.species_predictions, '[]'::json) as species_predictions,
    ocr_rel.ocr_result,
    face_rel.face_result,
    caption_rel.caption
FROM assets a
LEFT JOIN LATERAL (
    SELECT json_agg(
        jsonb_build_object(
            'thumbnail_id', t.thumbnail_id,
            'size', t.size,
            'storage_path', t.storage_path,
            'mime_type', t.mime_type
        )
        ORDER BY
            CASE t.size
                WHEN 'small' THEN 1
                WHEN 'medium' THEN 2
                WHEN 'large' THEN 3
            END,
            t.thumbnail_id ASC
    ) AS thumbnails
    FROM thumbnails t
    WHERE t.asset_id = a.asset_id
) thumbnails_rel ON true
LEFT JOIN LATERAL (
    SELECT json_agg(
        jsonb_build_object(
            'tag_id', tg.tag_id,
            'tag_name', tg.tag_name,
            'confidence', at.confidence
        )
        ORDER BY tg.tag_name ASC, tg.tag_id ASC
    ) AS tags
    FROM asset_tags at
    JOIN tags tg ON at.tag_id = tg.tag_id
    WHERE at.asset_id = a.asset_id
) tags_rel ON true
LEFT JOIN LATERAL (
    SELECT json_agg(
        jsonb_build_object(
            'album_id', al.album_id,
            'album_name', al.album_name,
            'position', aa.position,
            'added_time', aa.added_time
        )
        ORDER BY aa.position ASC NULLS LAST, aa.added_time ASC, al.album_id ASC
    ) AS albums
    FROM album_assets aa
    JOIN albums al ON aa.album_id = al.album_id
    WHERE aa.asset_id = a.asset_id
) albums_rel ON true
LEFT JOIN LATERAL (
    SELECT json_agg(
        jsonb_build_object(
            'label', sp.label,
            'score', sp.score
        )
        ORDER BY sp.score DESC, sp.label ASC
    ) AS species_predictions
    FROM species_predictions sp
    WHERE sp.asset_id = a.asset_id
) species_rel ON true
LEFT JOIN LATERAL (
    SELECT jsonb_build_object(
        'model_id', ocr.model_id,
        'total_count', ocr.total_count,
        'processing_time_ms', ocr.processing_time_ms,
        'created_at', ocr.created_at,
        'updated_at', ocr.updated_at,
        'text_items', COALESCE((
            SELECT jsonb_agg(
                jsonb_build_object(
                    'id', ocr_ti.id,
                    'text_content', ocr_ti.text_content,
                    'confidence', ocr_ti.confidence,
                    'bounding_box', ocr_ti.bounding_box,
                    'text_length', ocr_ti.text_length,
                    'area_pixels', ocr_ti.area_pixels
                )
                ORDER BY ocr_ti.confidence DESC, ocr_ti.text_length DESC, ocr_ti.id DESC
            )
            FROM ocr_text_items ocr_ti
            WHERE ocr_ti.asset_id = a.asset_id
        ), '[]'::jsonb)
    ) AS ocr_result
    FROM ocr_results ocr
    WHERE ocr.asset_id = a.asset_id
) ocr_rel ON true
LEFT JOIN LATERAL (
    SELECT jsonb_build_object(
        'model_id', fr.model_id,
        'total_faces', fr.total_faces,
        'processing_time_ms', fr.processing_time_ms,
        'created_at', fr.created_at,
        'updated_at', fr.updated_at,
        'faces', COALESCE((
            SELECT jsonb_agg(
                jsonb_build_object(
                    'id', fi.id,
                    'face_id', fi.face_id,
                    'bounding_box', fi.bounding_box,
                    'confidence', fi.confidence,
                    'age_group', fi.age_group,
                    'gender', fi.gender,
                    'ethnicity', fi.ethnicity,
                    'expression', fi.expression,
                    'is_primary', fi.is_primary,
                    'cluster_id', fcm.cluster_id,
                    'cluster_name', fc.cluster_name
                )
                ORDER BY fi.is_primary DESC, fi.confidence DESC, fi.id ASC
            )
            FROM face_items fi
            LEFT JOIN face_cluster_members fcm ON fi.id = fcm.face_id
            LEFT JOIN face_clusters fc ON fcm.cluster_id = fc.cluster_id
            WHERE fi.asset_id = a.asset_id
        ), '[]'::jsonb)
    ) AS face_result
    FROM face_results fr
    WHERE fr.asset_id = a.asset_id
) face_rel ON true
LEFT JOIN LATERAL (
    SELECT jsonb_build_object(
        'model_id', cap.model_id,
        'description', cap.description,
        'summary', cap.summary,
        'confidence', cap.confidence,
        'tokens_generated', cap.tokens_generated,
        'processing_time_ms', cap.processing_time_ms,
        'prompt_used', cap.prompt_used,
        'finish_reason', cap.finish_reason,
        'created_at', cap.created_at,
        'updated_at', cap.updated_at
    ) AS caption
    FROM captions cap
    WHERE cap.asset_id = a.asset_id
) caption_rel ON true
WHERE a.asset_id = $1 AND a.is_deleted = false;
