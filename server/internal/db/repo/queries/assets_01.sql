-- name: GetMediaItemRefsUnified :many
-- Agent ref materialization: same filter semantics as GetMediaItemsUnified but
-- returns one ordered (media_item_id, primary_asset_id) pair per logical media
-- item (capture time desc). The limit is the ref snapshot cap; callers detect
-- truncation by requesting cap+1.
WITH filter_params AS (
  SELECT
    CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json,
    CAST(sqlc.narg('asset_types') AS TEXT) AS asset_types_json,
    CAST(sqlc.narg('tag_names') AS TEXT) AS tag_names_json,
    CAST(sqlc.narg('stack_kinds') AS TEXT) AS stack_kinds_json
)
SELECT facts.media_item_id, facts.primary_asset_id
FROM media_item_browse_facts facts
JOIN assets pa ON pa.asset_id = facts.primary_asset_id
WHERE pa.is_deleted = COALESCE(sqlc.narg('is_deleted'), false)
  AND (
    (SELECT asset_ids_json FROM filter_params) IS NULL
    OR EXISTS (
      SELECT 1
      FROM media_item_assets mia_scope
      WHERE mia_scope.media_item_id = facts.media_item_id
        AND mia_scope.asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
    )
  )
  AND (
    sqlc.narg('query') IS NULL
    OR EXISTS (
      SELECT 1
      FROM media_item_assets mia_query
      JOIN assets component_query ON component_query.asset_id = mia_query.asset_id
      WHERE mia_query.media_item_id = facts.media_item_id
        AND component_query.original_filename LIKE '%' || sqlc.narg('query') || '%'
    )
  )
  AND (sqlc.narg('asset_type') IS NULL OR pa.type = sqlc.narg('asset_type'))
  AND (
    (SELECT asset_types_json FROM filter_params) IS NULL
    OR pa.type IN (SELECT value FROM json_each((SELECT asset_types_json FROM filter_params)))
  )
  AND (sqlc.narg('owner_id') IS NULL OR facts.owner_id = sqlc.narg('owner_id'))
  AND (sqlc.narg('repository_id') IS NULL OR EXISTS (
    SELECT 1 FROM active_asset_occurrences occurrence
    WHERE occurrence.asset_id = pa.asset_id
      AND occurrence.repository_id = sqlc.narg('repository_id')
  ))
  AND (
    sqlc.narg('folder_path') IS NULL
    OR EXISTS (
      SELECT 1 FROM active_asset_occurrence_paths occurrence_path
      WHERE occurrence_path.asset_id = pa.asset_id
        AND (sqlc.narg('repository_id') IS NULL OR occurrence_path.repository_id = sqlc.narg('repository_id'))
        AND CASE
          WHEN sqlc.narg('folder_path') = '' THEN
            COALESCE(sqlc.narg('folder_recursive'), true)
            OR instr(occurrence_path.relative_path, '/') = 0
          ELSE
            occurrence_path.relative_path LIKE sqlc.narg('folder_path') || '/%'
            AND (
              COALESCE(sqlc.narg('folder_recursive'), true)
              OR instr(substr(occurrence_path.relative_path, length(sqlc.narg('folder_path')) + 2), '/') = 0
            )
        END
    )
  )
  AND (
    sqlc.narg('tag_name') IS NULL
    OR EXISTS (
      SELECT 1
      FROM asset_tags at
      JOIN tags t ON t.tag_id = at.tag_id
      WHERE at.asset_id = pa.asset_id
        AND t.tag_name = sqlc.narg('tag_name')
        AND (sqlc.narg('tag_source') IS NULL OR at.source = sqlc.narg('tag_source'))
    )
  )
  AND (
    (SELECT tag_names_json FROM filter_params) IS NULL
    OR (
      SELECT COUNT(DISTINCT t2.tag_name)
      FROM asset_tags at2
      JOIN tags t2 ON t2.tag_id = at2.tag_id
      WHERE at2.asset_id = pa.asset_id
        AND t2.tag_name IN (SELECT value FROM json_each((SELECT tag_names_json FROM filter_params)))
    ) = json_array_length((SELECT tag_names_json FROM filter_params))
  )
  AND (
    sqlc.narg('person_id') IS NULL
    OR EXISTS (
      SELECT 1
      FROM face_cluster_members fcm
      JOIN face_items fi_person ON fi_person.id = fcm.face_id
      WHERE fcm.cluster_id = sqlc.narg('person_id')
        AND fi_person.asset_id = pa.asset_id
    )
  )
  AND (
    sqlc.narg('album_id') IS NULL
    OR EXISTS (
      SELECT 1
      FROM album_assets aa
      WHERE aa.asset_id = pa.asset_id
        AND aa.album_id = sqlc.narg('album_id')
    )
  )
  AND (
    sqlc.narg('filename_val') IS NULL
    OR EXISTS (
      SELECT 1
      FROM media_item_assets mia_name
      JOIN assets component_name ON component_name.asset_id = mia_name.asset_id
      WHERE mia_name.media_item_id = facts.media_item_id
        AND CASE COALESCE(sqlc.narg('filename_operator'), 'contains')
          WHEN 'matches' THEN component_name.original_filename LIKE sqlc.narg('filename_val')
          WHEN 'starts_with' THEN component_name.original_filename LIKE sqlc.narg('filename_val') || '%'
          WHEN 'ends_with' THEN component_name.original_filename LIKE '%' || sqlc.narg('filename_val')
          ELSE component_name.original_filename LIKE '%' || sqlc.narg('filename_val') || '%'
        END
    )
  )
  AND (sqlc.narg('date_from') IS NULL OR COALESCE(pa.taken_time, pa.upload_time) >= sqlc.narg('date_from'))
  AND (sqlc.narg('date_to') IS NULL OR COALESCE(pa.taken_time, pa.upload_time) <= sqlc.narg('date_to'))
  AND (
    sqlc.narg('composition') IS NULL
    OR CASE sqlc.narg('composition')
      WHEN 'contains_raw' THEN facts.has_raw = 1
      WHEN 'jpeg_raw' THEN facts.has_raw = 1 AND facts.has_jpeg = 1
      WHEN 'raw_unpaired' THEN facts.has_raw = 1 AND facts.has_jpeg = 0
      WHEN 'no_raw' THEN facts.has_raw = 0
      WHEN 'live_photo' THEN facts.has_live_motion = 1
      ELSE false
    END
  )
  AND (
    sqlc.narg('stack_membership') IS NULL
    OR CASE sqlc.narg('stack_membership')
      WHEN 'stacked' THEN facts.stack_id IS NOT NULL
      WHEN 'unstacked' THEN facts.stack_id IS NULL
      ELSE false
    END
  )
  AND (
    (SELECT stack_kinds_json FROM filter_params) IS NULL
    OR facts.stack_kind IN (SELECT value FROM json_each((SELECT stack_kinds_json FROM filter_params)))
  )
  AND (sqlc.narg('rating') IS NULL OR
    CASE
      WHEN sqlc.narg('rating') = 0 THEN pa.rating IS NULL OR pa.rating = 0
      ELSE pa.rating = sqlc.narg('rating')
    END
  )
  AND (sqlc.narg('liked') IS NULL OR
    CASE
      WHEN sqlc.narg('liked') = false THEN pa.liked IS NULL OR pa.liked = false
      ELSE pa.liked = true
    END
  )
  AND (sqlc.narg('camera_model') IS NULL OR json_extract(pa.specific_metadata, char(36) || '.camera_model') = sqlc.narg('camera_model'))
  AND (sqlc.narg('lens_model') IS NULL OR json_extract(pa.specific_metadata, char(36) || '.lens_model') = sqlc.narg('lens_model'))
  AND (
    sqlc.narg('place') IS NULL
    OR EXISTS (
      SELECT 1
      FROM location_cluster_assets lca
      JOIN location_clusters lc ON lc.cluster_id = lca.cluster_id
      JOIN location_search_fts lsf ON lsf.rowid = lc.rowid
      WHERE lca.asset_id = pa.asset_id
        AND location_search_fts MATCH sqlc.narg('place')
    )
  )
ORDER BY COALESCE(pa.taken_time, pa.upload_time) DESC, facts.media_item_id DESC
LIMIT sqlc.arg('limit');
