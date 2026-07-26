-- name: GetCollapsedBrowseItemsUnified :many
WITH filter_params AS (
  SELECT
    CAST(sqlc.narg('asset_ids') AS TEXT) AS asset_ids_json,
    CAST(sqlc.narg('asset_types') AS TEXT) AS asset_types_json,
    CAST(sqlc.narg('tag_names') AS TEXT) AS tag_names_json
),
filtered AS (
  SELECT
    a.asset_id,
    a.upload_time,
    COALESCE(a.taken_time, a.upload_time) AS captured_time,
    asm.stack_id,
    asm.position
  FROM assets a
  JOIN media_items mi ON mi.primary_asset_id = a.asset_id
  LEFT JOIN asset_stack_members asm ON asm.media_item_id = mi.media_item_id
  WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted'), false)
    AND (
      (SELECT asset_ids_json FROM filter_params) IS NULL
      OR a.asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
    )
    AND (sqlc.narg('query') IS NULL OR a.original_filename LIKE '%' || sqlc.narg('query') || '%')
    AND (sqlc.narg('asset_type') IS NULL OR a.type = sqlc.narg('asset_type'))
    AND (
      (SELECT asset_types_json FROM filter_params) IS NULL
      OR a.type IN (SELECT value FROM json_each((SELECT asset_types_json FROM filter_params)))
    )
    AND (sqlc.narg('owner_id') IS NULL OR a.owner_id = sqlc.narg('owner_id'))
    AND (sqlc.narg('repository_id') IS NULL OR a.repository_id = sqlc.narg('repository_id'))
    AND (
      sqlc.narg('folder_path') IS NULL
      OR (
        CASE
          WHEN sqlc.narg('folder_path') = '' THEN
            CASE WHEN COALESCE(sqlc.narg('folder_recursive'), true) THEN true
              ELSE instr(a.storage_path, '/') = 0
            END
          ELSE
            CASE WHEN COALESCE(sqlc.narg('folder_recursive'), true) THEN
              a.storage_path LIKE sqlc.narg('folder_path') || '/%'
            ELSE
              a.storage_path LIKE sqlc.narg('folder_path') || '/%'
              AND a.storage_path NOT LIKE sqlc.narg('folder_path') || '/%/%'
            END
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
          AND fi_person.asset_id = a.asset_id
      )
    )
    AND (
      sqlc.narg('album_id') IS NULL
      OR EXISTS (
        SELECT 1
        FROM album_assets aa
        WHERE aa.asset_id = a.asset_id
          AND aa.album_id = sqlc.narg('album_id')
      )
    )
    AND (
      sqlc.narg('tag_name') IS NULL
      OR EXISTS (
        SELECT 1
        FROM asset_tags at
        JOIN tags t ON t.tag_id = at.tag_id
        WHERE at.asset_id = a.asset_id
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
        WHERE at2.asset_id = a.asset_id
          AND t2.tag_name IN (SELECT value FROM json_each((SELECT tag_names_json FROM filter_params)))
      ) = json_array_length((SELECT tag_names_json FROM filter_params))
    )
    AND (sqlc.narg('filename_val') IS NULL OR
      CASE COALESCE(sqlc.narg('filename_operator'), 'contains')
        WHEN 'matches' THEN a.original_filename LIKE sqlc.narg('filename_val')
        WHEN 'starts_with' THEN a.original_filename LIKE sqlc.narg('filename_val') || '%'
        WHEN 'ends_with' THEN a.original_filename LIKE '%' || sqlc.narg('filename_val')
        ELSE a.original_filename LIKE '%' || sqlc.narg('filename_val') || '%'
      END
    )
    AND (sqlc.narg('date_from') IS NULL OR COALESCE(a.taken_time, a.upload_time) >= sqlc.narg('date_from'))
    AND (sqlc.narg('date_to') IS NULL OR COALESCE(a.taken_time, a.upload_time) <= sqlc.narg('date_to'))
    AND (sqlc.narg('is_raw') IS NULL OR
      CASE
        WHEN sqlc.narg('is_raw') = true THEN json_extract(a.specific_metadata, char(36) || '.is_raw') = 1
        ELSE json_extract(a.specific_metadata, char(36) || '.is_raw') = 0 OR json_extract(a.specific_metadata, char(36) || '.is_raw') IS NULL
      END
    )
    AND (sqlc.narg('rating') IS NULL OR
      CASE
        WHEN sqlc.narg('rating') = 0 THEN a.rating IS NULL OR a.rating = 0
        ELSE a.rating = sqlc.narg('rating')
      END
    )
    AND (sqlc.narg('liked') IS NULL OR
      CASE
        WHEN sqlc.narg('liked') = false THEN a.liked IS NULL OR a.liked = false
        ELSE a.liked = true
      END
    )
    AND (sqlc.narg('camera_model') IS NULL OR json_extract(a.specific_metadata, char(36) || '.camera_model') = sqlc.narg('camera_model'))
    AND (sqlc.narg('lens_model') IS NULL OR json_extract(a.specific_metadata, char(36) || '.lens_model') = sqlc.narg('lens_model'))
    AND (
      sqlc.narg('location_north') IS NULL
      OR sqlc.narg('location_south') IS NULL
      OR sqlc.narg('location_east') IS NULL
      OR sqlc.narg('location_west') IS NULL
      OR (
        a.gps_latitude IS NOT NULL
        AND a.gps_longitude IS NOT NULL
        AND a.gps_latitude
          BETWEEN min(sqlc.narg('location_south'), sqlc.narg('location_north'))
          AND max(sqlc.narg('location_south'), sqlc.narg('location_north'))
        AND (
          CASE
            WHEN sqlc.narg('location_west') <= sqlc.narg('location_east') THEN
              a.gps_longitude BETWEEN sqlc.narg('location_west') AND sqlc.narg('location_east')
            ELSE
              a.gps_longitude >= sqlc.narg('location_west')
              OR a.gps_longitude <= sqlc.narg('location_east')
          END
        )
      )
    )
),
stack_cover_candidates AS (
  SELECT
    asm.stack_id,
    COALESCE(cover_item.primary_asset_id, mi.primary_asset_id) AS cover_asset_id,
    ROW_NUMBER() OVER (
      PARTITION BY asm.stack_id
      ORDER BY asm.position IS NULL, asm.position, asm.media_item_id
    ) AS cover_rank
  FROM asset_stack_members asm
  JOIN asset_stacks s ON s.stack_id = asm.stack_id
  JOIN media_items mi ON mi.media_item_id = asm.media_item_id
  LEFT JOIN media_items cover_item ON cover_item.media_item_id = s.cover_media_item_id
  JOIN assets a ON a.asset_id = mi.primary_asset_id
  WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted'), false)
    AND (
      (SELECT asset_ids_json FROM filter_params) IS NULL
      OR a.asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
    )
),
stack_covers AS (
  SELECT stack_id, cover_asset_id
  FROM stack_cover_candidates
  WHERE cover_rank = 1
),
stack_members_all AS (
  SELECT ordered.stack_id, json_group_array(ordered.primary_asset_id) AS member_asset_ids
  FROM (
    SELECT asm.stack_id, mi.primary_asset_id
    FROM asset_stack_members asm
    JOIN media_items mi ON mi.media_item_id = asm.media_item_id
    JOIN assets a ON a.asset_id = mi.primary_asset_id
    WHERE a.is_deleted = COALESCE(sqlc.narg('is_deleted'), false)
      AND (
        (SELECT asset_ids_json FROM filter_params) IS NULL
        OR a.asset_id IN (SELECT value FROM json_each((SELECT asset_ids_json FROM filter_params)))
      )
    ORDER BY asm.stack_id, asm.position IS NULL, asm.position, asm.media_item_id
  ) AS ordered
  GROUP BY ordered.stack_id
),
browse_items AS (
  SELECT
    CASE WHEN f.stack_id IS NULL THEN 'asset' ELSE 'stack' END AS item_type,
    f.stack_id,
    CASE WHEN f.stack_id IS NULL THEN f.asset_id ELSE sc.cover_asset_id END AS cover_asset_id,
    sma.member_asset_ids,
    json_group_array(f.asset_id) AS matched_asset_ids
  FROM (
    SELECT *
    FROM filtered
    ORDER BY stack_id, position IS NULL, position, asset_id
  ) AS f
  LEFT JOIN stack_covers sc ON sc.stack_id = f.stack_id
  LEFT JOIN stack_members_all sma ON sma.stack_id = f.stack_id
  GROUP BY
    CASE WHEN f.stack_id IS NULL THEN 'asset' ELSE 'stack' END,
    f.stack_id,
    CASE WHEN f.stack_id IS NULL THEN f.asset_id ELSE sc.cover_asset_id END,
    sma.member_asset_ids
),
paged AS (
  SELECT
    bi.item_type,
    bi.stack_id,
    bi.cover_asset_id,
    bi.member_asset_ids,
    bi.matched_asset_ids,
    CASE
      WHEN sqlc.narg('sort_by') = 'recently_added' THEN cover.upload_time
      ELSE COALESCE(cover.taken_time, cover.upload_time)
    END AS sort_time
  FROM browse_items bi
  JOIN assets cover ON cover.asset_id = bi.cover_asset_id
  ORDER BY sort_time DESC, cover.asset_id DESC
  LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset')
)
SELECT
  p.item_type,
  p.stack_id,
  p.cover_asset_id,
  p.member_asset_ids,
  p.matched_asset_ids,
  sqlc.embed(cover)
FROM paged p
JOIN assets cover ON cover.asset_id = p.cover_asset_id
ORDER BY p.sort_time DESC, p.cover_asset_id DESC;
