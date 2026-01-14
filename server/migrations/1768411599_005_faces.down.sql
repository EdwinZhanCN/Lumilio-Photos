-- Reverse migration - drops all objects created in up migration
-- WARNING: This will permanently delete data

DROP FUNCTION IF EXISTS update_cluster_member_count();
DROP FUNCTION IF EXISTS update_face_results_updated_at();
DROP INDEX IF EXISTS face_cluster_members_similarity_idx;
DROP INDEX IF EXISTS face_cluster_members_face_idx;
DROP INDEX IF EXISTS face_cluster_members_cluster_idx;
DROP INDEX IF EXISTS face_clusters_confirmed_idx;
DROP INDEX IF EXISTS face_clusters_representative_idx;
DROP INDEX IF EXISTS face_items_embedding_idx;
DROP INDEX IF EXISTS face_items_is_primary_idx;
DROP INDEX IF EXISTS face_items_expression_idx;
DROP INDEX IF EXISTS face_items_ethnicity_idx;
DROP INDEX IF EXISTS face_items_gender_idx;
DROP INDEX IF EXISTS face_items_age_group_idx;
DROP INDEX IF EXISTS face_items_confidence_idx;
DROP INDEX IF EXISTS face_items_face_id_idx;
DROP INDEX IF EXISTS face_items_asset_id_idx;
DROP INDEX IF EXISTS face_results_created_at_idx;
DROP INDEX IF EXISTS face_results_model_id_idx;
DROP INDEX IF EXISTS face_results_asset_id_idx;
DROP TABLE IF EXISTS face_cluster_members CASCADE;
DROP TABLE IF EXISTS face_clusters CASCADE;
DROP TABLE IF EXISTS face_items CASCADE;
DROP TABLE IF EXISTS face_results CASCADE;
