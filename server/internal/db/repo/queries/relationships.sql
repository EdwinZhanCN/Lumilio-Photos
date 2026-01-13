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
GROUP BY a.asset_id, a.rating, a.liked;

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
GROUP BY a.asset_id, a.rating, a.liked;

-- name: GetAssetWithRelations :one
SELECT
    a.*,
    COALESCE(
        json_agg(DISTINCT
            jsonb_build_object(
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
            jsonb_build_object(
                'tag_id', tg.tag_id,
                'tag_name', tg.tag_name,
                'confidence', at.confidence
            )
        ) FILTER (WHERE tg.tag_id IS NOT NULL),
        '[]'
    ) as tags,
    COALESCE(
        json_agg(DISTINCT
            jsonb_build_object(
                'album_id', al.album_id,
                'album_name', al.album_name,
                'position', aa.position,
                'added_time', aa.added_time
            )
        ) FILTER (WHERE al.album_id IS NOT NULL),
        '[]'
    ) as albums,
    COALESCE(
        json_agg(DISTINCT
            jsonb_build_object(
                'label', sp.label,
                'score', sp.score
            )
        ) FILTER (WHERE sp.label IS NOT NULL),
        '[]'
    ) as species_predictions,
    COALESCE(
        jsonb_build_object(
            'model_id', ocr.model_id,
            'total_count', ocr.total_count,
            'processing_time_ms', ocr.processing_time_ms,
            'created_at', ocr.created_at,
            'updated_at', ocr.updated_at,
            'text_items', COALESCE(
                json_agg(
                    jsonb_build_object(
                        'id', ocr_ti.id,
                        'text_content', ocr_ti.text_content,
                        'confidence', ocr_ti.confidence,
                        'bounding_box', ocr_ti.bounding_box,
                        'text_length', ocr_ti.text_length,
                        'area_pixels', ocr_ti.area_pixels
                    )
                ) FILTER (WHERE ocr_ti.id IS NOT NULL),
                '[]'::json
            )
        ) FILTER (WHERE ocr.asset_id IS NOT NULL),
        NULL
    ) as ocr_result,
    COALESCE(
        jsonb_build_object(
            'model_id', fr.model_id,
            'total_faces', fr.total_faces,
            'processing_time_ms', fr.processing_time_ms,
            'created_at', fr.created_at,
            'updated_at', fr.updated_at,
            'faces', COALESCE(
                json_agg(
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
                ) FILTER (WHERE fi.id IS NOT NULL),
                '[]'::json
            )
        ) FILTER (WHERE fr.asset_id IS NOT NULL),
        NULL
    ) as face_result,
    COALESCE(
        jsonb_build_object(
            'model_id', ai_desc.model_id,
            'description', ai_desc.description,
            'summary', ai_desc.summary,
            'confidence', ai_desc.confidence,
            'tokens_generated', ai_desc.tokens_generated,
            'processing_time_ms', ai_desc.processing_time_ms,
            'prompt_used', ai_desc.prompt_used,
            'finish_reason', ai_desc.finish_reason,
            'created_at', ai_desc.created_at,
            'updated_at', ai_desc.updated_at
        ) FILTER (WHERE ai_desc.asset_id IS NOT NULL),
        NULL
    ) as ai_description
FROM assets a
LEFT JOIN thumbnails t ON a.asset_id = t.asset_id
LEFT JOIN asset_tags at ON a.asset_id = at.asset_id
LEFT JOIN tags tg ON at.tag_id = tg.tag_id
LEFT JOIN album_assets aa ON a.asset_id = aa.asset_id
LEFT JOIN albums al ON aa.album_id = al.album_id
LEFT JOIN species_predictions sp ON a.asset_id = sp.asset_id
LEFT JOIN ocr_results ocr ON a.asset_id = ocr.asset_id
LEFT JOIN ocr_text_items ocr_ti ON a.asset_id = ocr_ti.asset_id
LEFT JOIN face_results fr ON a.asset_id = fr.asset_id
LEFT JOIN face_items fi ON a.asset_id = fi.asset_id
LEFT JOIN face_cluster_members fcm ON fi.id = fcm.face_id
LEFT JOIN face_clusters fc ON fcm.cluster_id = fc.cluster_id
LEFT JOIN ai_descriptions ai_desc ON a.asset_id = ai_desc.asset_id
WHERE a.asset_id = $1 AND a.is_deleted = false
GROUP BY a.asset_id, a.rating, a.liked,
         ocr.asset_id, ocr.model_id, ocr.total_count, ocr.processing_time_ms, ocr.created_at, ocr.updated_at,
         fr.asset_id, fr.model_id, fr.total_faces, fr.processing_time_ms, fr.created_at, fr.updated_at,
         ai_desc.asset_id, ai_desc.model_id, ai_desc.description, ai_desc.summary,
         ai_desc.confidence, ai_desc.tokens_generated, ai_desc.processing_time_ms,
         ai_desc.prompt_used, ai_desc.finish_reason, ai_desc.created_at, ai_desc.updated_at;
