DROP INDEX IF EXISTS face_items_cluster_candidate_idx;
DROP INDEX IF EXISTS face_items_embedding_model_idx;
DROP INDEX IF EXISTS face_cluster_members_face_unique_idx;

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
        SET member_count = member_count - 1,
            updated_at = CURRENT_TIMESTAMP
        WHERE cluster_id = OLD.cluster_id;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
