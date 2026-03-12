UPDATE face_clusters fc
SET representative_face_id = members.face_id
FROM (
    SELECT cluster_id, MIN(face_id) AS face_id
    FROM face_cluster_members
    GROUP BY cluster_id
) members
WHERE fc.cluster_id = members.cluster_id
  AND fc.representative_face_id IS NULL;

DELETE FROM face_clusters
WHERE representative_face_id IS NULL;

ALTER TABLE face_clusters
DROP CONSTRAINT IF EXISTS face_clusters_representative_face_id_fkey;

ALTER TABLE face_clusters
ALTER COLUMN representative_face_id SET NOT NULL;

ALTER TABLE face_clusters
ADD CONSTRAINT face_clusters_representative_face_id_fkey
FOREIGN KEY (representative_face_id) REFERENCES face_items(id);
