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
    CASE
        WHEN ocr.asset_id IS NOT NULL THEN jsonb_build_object(
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
        )
        ELSE NULL
    END as ocr_result,
    CASE
        WHEN fr.asset_id IS NOT NULL THEN jsonb_build_object(
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
        )
        ELSE NULL
    END as face_result,
    CASE
        WHEN cap.asset_id IS NOT NULL THEN jsonb_build_object(
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
        )
        ELSE NULL
    END as caption
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
LEFT JOIN captions cap ON a.asset_id = cap.asset_id
WHERE a.asset_id = $1 AND a.is_deleted = false
GROUP BY a.asset_id, a.rating, a.liked,
         ocr.asset_id, ocr.model_id, ocr.total_count, ocr.processing_time_ms, ocr.created_at, ocr.updated_at,
         fr.asset_id, fr.model_id, fr.total_faces, fr.processing_time_ms, fr.created_at, fr.updated_at,
         cap.asset_id, cap.model_id, cap.description, cap.summary,
         cap.confidence, cap.tokens_generated, cap.processing_time_ms,
         cap.prompt_used, cap.finish_reason, cap.created_at, cap.updated_at;
