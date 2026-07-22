-- The first account represents the owner of the local host. Repositories stay
-- shared storage and do not introduce a second, per-repository ownership model;
-- their default_owner_id is only the fallback for ownerless filesystem scans.
-- Explicit upload and cloud owners are intentionally preserved.
DO $$
DECLARE
    host_owner_id integer;
BEGIN
    SELECT user_id
    INTO host_owner_id
    FROM users
    ORDER BY created_at ASC, user_id ASC
    LIMIT 1;

    -- A fresh database has no user while migrations run. First-user setup will
    -- claim any repositories that were attached before bootstrap completed.
    IF host_owner_id IS NULL THEN
        RETURN;
    END IF;

    -- Remove the old accidental per-repository default: every repository uses
    -- the same Host Owner when an ingest source does not name a user.
    UPDATE repositories
    SET default_owner_id = host_owner_id,
        updated_at = CURRENT_TIMESTAMP
    WHERE default_owner_id IS DISTINCT FROM host_owner_id;

    UPDATE assets
    SET owner_id = host_owner_id
    WHERE owner_id IS NULL;

    -- Asset insertion stamps these structural rows, so historical ownerless
    -- assets need their derived ownership repaired as well.
    UPDATE media_items mi
    SET owner_id = a.owner_id,
        updated_at = CURRENT_TIMESTAMP
    FROM assets a
    WHERE mi.owner_id IS NULL
      AND mi.primary_asset_id = a.asset_id
      AND a.owner_id IS NOT NULL;

    UPDATE asset_stacks stack
    SET owner_id = COALESCE(
            (
                SELECT mi.owner_id
                FROM asset_stack_members asm
                JOIN media_items mi ON mi.media_item_id = asm.media_item_id
                WHERE asm.stack_id = stack.stack_id
                  AND mi.owner_id IS NOT NULL
                ORDER BY asm.position ASC, asm.media_item_id ASC
                LIMIT 1
            ),
            host_owner_id
        ),
        updated_at = CURRENT_TIMESTAMP
    WHERE stack.owner_id IS NULL;

    UPDATE duplicate_groups duplicate_group
    SET owner_id = COALESCE(
            (
                SELECT asset.owner_id
                FROM duplicate_group_assets member
                JOIN assets asset ON asset.asset_id = member.asset_id
                WHERE member.group_id = duplicate_group.group_id
                  AND asset.owner_id IS NOT NULL
                ORDER BY member.asset_id ASC
                LIMIT 1
            ),
            host_owner_id
        ),
        updated_at = CURRENT_TIMESTAMP
    WHERE duplicate_group.owner_id IS NULL;

    UPDATE face_clusters cluster
    SET owner_id = COALESCE(
            (
                SELECT asset.owner_id
                FROM face_cluster_members membership
                JOIN face_items face ON face.id = membership.face_id
                JOIN assets asset ON asset.asset_id = face.asset_id
                WHERE membership.cluster_id = cluster.cluster_id
                  AND asset.owner_id IS NOT NULL
                ORDER BY membership.face_id ASC
                LIMIT 1
            ),
            host_owner_id
        ),
        updated_at = CURRENT_TIMESTAMP
    WHERE cluster.owner_id IS NULL;

    -- NULL-owner and Host-Owner location rows can share the same repository /
    -- geohash because NULL used to be a separate unique key. Preserve the
    -- already-owned row (and its geocoded label), discard only the duplicate,
    -- then rebuild mappings and aggregate geometry from the repaired assets.
    DELETE FROM location_cluster_assets membership
    USING location_clusters cluster
    WHERE membership.cluster_id = cluster.cluster_id
      AND (cluster.owner_id IS NULL OR cluster.owner_id = host_owner_id);

    DELETE FROM location_clusters source
    WHERE source.owner_id IS NULL
      AND EXISTS (
          SELECT 1
          FROM location_clusters target
          WHERE target.owner_id = host_owner_id
            AND target.repository_id = source.repository_id
            AND target.geohash = source.geohash
      );

    UPDATE location_clusters
    SET owner_id = host_owner_id,
        updated_at = CURRENT_TIMESTAMP
    WHERE owner_id IS NULL;

    INSERT INTO location_cluster_assets (cluster_id, asset_id)
    SELECT cluster.cluster_id, asset.asset_id
    FROM location_clusters cluster
    JOIN assets asset
      ON asset.owner_id = cluster.owner_id
     AND asset.repository_id = cluster.repository_id
     AND asset.gps_geohash_7 = cluster.geohash
    WHERE cluster.owner_id = host_owner_id
      AND asset.is_deleted = false
      AND asset.type = 'PHOTO'
      AND asset.gps_latitude IS NOT NULL
      AND asset.gps_longitude IS NOT NULL
    ON CONFLICT (cluster_id, asset_id) DO NOTHING;

    UPDATE location_clusters cluster
    SET centroid_latitude = aggregate.centroid_latitude,
        centroid_longitude = aggregate.centroid_longitude,
        photo_count = aggregate.photo_count,
        updated_at = CURRENT_TIMESTAMP
    FROM (
        SELECT membership.cluster_id,
               AVG(asset.gps_latitude)::double precision AS centroid_latitude,
               AVG(asset.gps_longitude)::double precision AS centroid_longitude,
               COUNT(*)::integer AS photo_count
        FROM location_cluster_assets membership
        JOIN assets asset ON asset.asset_id = membership.asset_id
        GROUP BY membership.cluster_id
    ) aggregate
    WHERE cluster.cluster_id = aggregate.cluster_id
      AND cluster.owner_id = host_owner_id;
END $$;
