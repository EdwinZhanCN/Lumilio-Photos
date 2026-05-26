-- Face clustering refactor support: one cluster membership per face and faster scoped clustering queries.

WITH ranked_members AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY face_id
            ORDER BY COALESCE(is_manual, false) DESC, confidence DESC, similarity_score DESC, id ASC
        ) AS rank
    FROM face_cluster_members
)
DELETE FROM face_cluster_members fcm
USING ranked_members ranked
WHERE fcm.id = ranked.id
  AND ranked.rank > 1;

CREATE UNIQUE INDEX IF NOT EXISTS face_cluster_members_face_unique_idx
    ON face_cluster_members(face_id);

CREATE INDEX IF NOT EXISTS face_items_embedding_model_idx
    ON face_items(embedding_model)
    WHERE embedding IS NOT NULL;

CREATE INDEX IF NOT EXISTS face_items_cluster_candidate_idx
    ON face_items(confidence, face_size)
    WHERE embedding IS NOT NULL;

CREATE OR REPLACE FUNCTION update_cluster_member_count()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE face_clusters
        SET member_count = member_count + 1,
            updated_at = CURRENT_TIMESTAMP
        WHERE cluster_id = NEW.cluster_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE face_clusters
        SET member_count = GREATEST(member_count - 1, 0),
            updated_at = CURRENT_TIMESTAMP
        WHERE cluster_id = OLD.cluster_id;
        RETURN OLD;
    ELSIF TG_OP = 'UPDATE' THEN
        IF OLD.cluster_id IS DISTINCT FROM NEW.cluster_id THEN
            UPDATE face_clusters
            SET member_count = GREATEST(member_count - 1, 0),
                updated_at = CURRENT_TIMESTAMP
            WHERE cluster_id = OLD.cluster_id;

            UPDATE face_clusters
            SET member_count = member_count + 1,
                updated_at = CURRENT_TIMESTAMP
            WHERE cluster_id = NEW.cluster_id;
        END IF;
        RETURN NEW;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
