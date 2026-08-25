-- name: GetCollapsedBrowseItemsUnified :many
-- Collapsed presentation browse: one row per presentation stack (any member
-- matched) and one row per unstacked matching media item. The cover row is
-- the stack's designated cover media item when visible, otherwise the
-- lowest-position visible member.
WITH filter_params AS (
  SELECT
    CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json,
    CAST(sqlc.narg('asset_types') AS TEXT) AS asset_types_json,
    CAST(sqlc.narg('tag_names') AS TEXT) AS tag_names_json,
    CAST(sqlc.narg('stack_kinds') AS TEXT) AS stack_kinds_json
),
eligible AS (
  SELECT
    facts.media_item_id,
    facts.primary_asset_id,
    facts.stack_id,
    facts.stack_position,
    facts.stack_kind
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
      sqlc.narg('location_north') IS NULL
      OR sqlc.narg('location_south') IS NULL
      OR sqlc.narg('location_east') IS NULL
      OR sqlc.narg('location_west') IS NULL
      OR (
        pa.gps_latitude IS NOT NULL
        AND pa.gps_longitude IS NOT NULL
        AND pa.gps_latitude
          BETWEEN min(sqlc.narg('location_south'), sqlc.narg('location_north'))
          AND max(sqlc.narg('location_south'), sqlc.narg('location_north'))
        AND (
          CASE
            WHEN sqlc.narg('location_west') <= sqlc.narg('location_east') THEN
              pa.gps_longitude BETWEEN sqlc.narg('location_west') AND sqlc.narg('location_east')
            ELSE
              pa.gps_longitude >= sqlc.narg('location_west')
              OR pa.gps_longitude <= sqlc.narg('location_east')
          END
        )
      )
    )
),
stack_covers AS (
  SELECT ranked.stack_id, ranked.media_item_id AS cover_media_item_id
  FROM (
    SELECT
      asm.stack_id,
      asm.media_item_id,
      ROW_NUMBER() OVER (
        PARTITION BY asm.stack_id
        ORDER BY
          (asm.media_item_id = s.cover_media_item_id) DESC,
          asm.position IS NULL,
          asm.position,
          asm.media_item_id
      ) AS cover_rank
    FROM asset_stack_members asm
    JOIN asset_stacks s ON s.stack_id = asm.stack_id
    JOIN media_items mi ON mi.media_item_id = asm.media_item_id
    JOIN assets a ON a.asset_id = mi.primary_asset_id
    WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted'), false)
  ) ranked
  WHERE ranked.cover_rank = 1
),
stack_members_all AS (
  SELECT
    ordered.stack_id,
    json_group_array(json_object(
      'media_item_id', ordered.media_item_id,
      'primary_asset_id', ordered.primary_asset_id
    )) AS member_items
  FROM (
    SELECT asm.stack_id, asm.media_item_id, mi.primary_asset_id
    FROM asset_stack_members asm
    JOIN media_items mi ON mi.media_item_id = asm.media_item_id
    JOIN assets a ON a.asset_id = mi.primary_asset_id
    WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted'), false)
    ORDER BY asm.stack_id, asm.position IS NULL, asm.position, asm.media_item_id
  ) AS ordered
  GROUP BY ordered.stack_id
),
stack_matches AS (
  SELECT
    matched.stack_id,
    matched.stack_kind,
    json_group_array(json_object(
      'media_item_id', matched.media_item_id,
      'primary_asset_id', matched.primary_asset_id
    )) AS matched_items
  FROM (
    SELECT e.stack_id, e.stack_kind, e.media_item_id, e.primary_asset_id
    FROM eligible e
    WHERE e.stack_id IS NOT NULL
    ORDER BY e.stack_id, e.stack_position IS NULL, e.stack_position, e.media_item_id
  ) AS matched
  GROUP BY matched.stack_id, matched.stack_kind
),
browse_rows AS (
  SELECT
    'media_item' AS item_type,
    NULL AS stack_id,
    NULL AS stack_kind,
    e.media_item_id AS cover_media_item_id,
    NULL AS member_items,
    NULL AS matched_items
  FROM eligible e
  WHERE e.stack_id IS NULL
  UNION ALL
  SELECT
    'stack' AS item_type,
    sm.stack_id,
    sm.stack_kind,
    sc.cover_media_item_id,
    sma.member_items,
    sm.matched_items
  FROM stack_matches sm
  JOIN stack_covers sc ON sc.stack_id = sm.stack_id
  LEFT JOIN stack_members_all sma ON sma.stack_id = sm.stack_id
),
paged AS (
  SELECT
    br.item_type,
    br.stack_id,
    br.stack_kind,
    br.cover_media_item_id,
    br.member_items,
    br.matched_items,
    cover_facts.media_kind AS cover_media_kind,
    cover_facts.primary_asset_id AS cover_primary_asset_id,
    cover_facts.component_count AS cover_component_count,
    cover_facts.has_raw AS cover_has_raw,
    cover_facts.has_jpeg AS cover_has_jpeg,
    cover_facts.has_edited AS cover_has_edited,
    cover_facts.has_live_motion AS cover_has_live_motion,
    CASE
      WHEN sqlc.narg('sort_by') = 'recently_added' THEN cover_pa.upload_time
      ELSE COALESCE(cover_pa.taken_time, cover_pa.upload_time)
    END AS sort_time
  FROM browse_rows br
  JOIN media_item_browse_facts cover_facts ON cover_facts.media_item_id = br.cover_media_item_id
  JOIN assets cover_pa ON cover_pa.asset_id = cover_facts.primary_asset_id
  ORDER BY sort_time DESC, br.cover_media_item_id DESC
  LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset')
)
SELECT
  p.item_type,
  p.stack_id,
  p.stack_kind,
  p.cover_media_item_id,
  p.cover_media_kind,
  CAST(p.cover_component_count AS INTEGER) AS cover_component_count,
  CAST(p.cover_has_raw AS INTEGER) AS cover_has_raw,
  CAST(p.cover_has_jpeg AS INTEGER) AS cover_has_jpeg,
  CAST(p.cover_has_edited AS INTEGER) AS cover_has_edited,
  CAST(p.cover_has_live_motion AS INTEGER) AS cover_has_live_motion,
  p.member_items,
  p.matched_items,
  sqlc.embed(cover_pa)
FROM paged p
JOIN assets cover_pa ON cover_pa.asset_id = p.cover_primary_asset_id
ORDER BY p.sort_time DESC, p.cover_media_item_id DESC;
