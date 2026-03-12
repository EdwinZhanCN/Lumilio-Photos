ALTER TABLE face_clusters
DROP CONSTRAINT IF EXISTS face_clusters_representative_face_id_fkey;

ALTER TABLE face_clusters
ALTER COLUMN representative_face_id DROP NOT NULL;

ALTER TABLE face_clusters
ADD CONSTRAINT face_clusters_representative_face_id_fkey
FOREIGN KEY (representative_face_id) REFERENCES face_items(id) ON DELETE SET NULL;
