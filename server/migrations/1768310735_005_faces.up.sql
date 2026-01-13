-- Face Detection and Recognition Table
-- Store face detection results and recognition information

-- Create face detection results main table
CREATE TABLE face_results (
    asset_id UUID NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    model_id VARCHAR(100) NOT NULL,           -- Face detection model identifier
    total_faces INTEGER NOT NULL DEFAULT 0,   -- Total number of detected faces
    processing_time_ms INTEGER,               -- Processing time in milliseconds
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (asset_id)
);

-- Create face items table (for each detected face)
CREATE TABLE face_items (
    id SERIAL PRIMARY KEY,
    asset_id UUID NOT NULL REFERENCES face_results(asset_id) ON DELETE CASCADE,
    face_id VARCHAR(100),                     -- Face identifier for recognition tracking
    bounding_box JSONB NOT NULL,              -- Face bounding box: {x, y, width, height}
    confidence REAL NOT NULL,                 -- Detection confidence (0.0-1.0)
    age_group VARCHAR(20),                    -- Age group: child, teen, adult, senior
    gender VARCHAR(20),                       -- Gender: male, female
    ethnicity VARCHAR(30),                    -- Ethnicity prediction
    expression VARCHAR(30),                   -- Expression: neutral, happy, sad, angry, etc.
    face_size INTEGER,                        -- Face size in pixels (approximate)
    face_image_path VARCHAR(512),             -- Path to cropped face image
    embedding VECTOR(512),                    -- Face embedding vector for recognition
    embedding_model VARCHAR(100),             -- Model used for embedding generation
    is_primary BOOLEAN DEFAULT FALSE,         -- Mark as primary face in the image
    quality_score REAL,                       -- Face quality score (0.0-1.0)
    blur_score REAL,                          -- Blur detection score (0.0-1.0, lower is less blurry)
    pose_angles JSONB,                        -- Head pose angles: {yaw, pitch, roll}
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Create face clusters table (for face recognition groups)
CREATE TABLE face_clusters (
    cluster_id SERIAL PRIMARY KEY,
    cluster_name VARCHAR(255),                -- User-assigned name for the person
    representative_face_id INTEGER NOT NULL REFERENCES face_items(id),
    confidence_score REAL DEFAULT 0.0,        -- Cluster confidence score
    member_count INTEGER DEFAULT 0,           -- Number of faces in this cluster
    is_confirmed BOOLEAN DEFAULT FALSE,       -- Whether this cluster has been confirmed by user
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Create face cluster membership table
CREATE TABLE face_cluster_members (
    id SERIAL PRIMARY KEY,
    cluster_id INTEGER NOT NULL REFERENCES face_clusters(cluster_id) ON DELETE CASCADE,
    face_id INTEGER NOT NULL REFERENCES face_items(id) ON DELETE CASCADE,
    similarity_score REAL NOT NULL,           -- Similarity score to cluster representative
    confidence REAL NOT NULL,                 -- Assignment confidence
    is_manual BOOLEAN DEFAULT FALSE,          -- Whether this assignment was manual
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(cluster_id, face_id)
);

-- Index optimization
-- Main table indexes
CREATE INDEX face_results_asset_id_idx ON face_results(asset_id);
CREATE INDEX face_results_model_id_idx ON face_results(model_id);
CREATE INDEX face_results_created_at_idx ON face_results(created_at);

-- Face items table indexes
CREATE INDEX face_items_asset_id_idx ON face_items(asset_id);
CREATE INDEX face_items_face_id_idx ON face_items(face_id) WHERE face_id IS NOT NULL;
CREATE INDEX face_items_confidence_idx ON face_items(confidence);
CREATE INDEX face_items_age_group_idx ON face_items(age_group);
CREATE INDEX face_items_gender_idx ON face_items(gender);
CREATE INDEX face_items_ethnicity_idx ON face_items(ethnicity);
CREATE INDEX face_items_expression_idx ON face_items(expression);
CREATE INDEX face_items_is_primary_idx ON face_items(is_primary) WHERE is_primary = true;

-- Face embedding similarity index
CREATE INDEX face_items_embedding_idx ON face_items USING hnsw (embedding vector_cosine_ops)
WITH (m = 16, ef_construction = 200);

-- Cluster-related indexes
CREATE INDEX face_clusters_representative_idx ON face_clusters(representative_face_id);
CREATE INDEX face_clusters_confirmed_idx ON face_clusters(is_confirmed) WHERE is_confirmed = true;
CREATE INDEX face_cluster_members_cluster_idx ON face_cluster_members(cluster_id);
CREATE INDEX face_cluster_members_face_idx ON face_cluster_members(face_id);
CREATE INDEX face_cluster_members_similarity_idx ON face_cluster_members(similarity_score);

-- Update timestamp trigger for face_results
CREATE OR REPLACE FUNCTION update_face_results_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE face_results
    SET updated_at = CURRENT_TIMESTAMP
    WHERE asset_id = NEW.asset_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER face_items_update_trigger
    AFTER INSERT OR UPDATE OR DELETE ON face_items
    FOR EACH ROW
    EXECUTE FUNCTION update_face_results_updated_at();

-- Update cluster member count trigger
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

CREATE TRIGGER face_cluster_members_count_trigger
    AFTER INSERT OR DELETE ON face_cluster_members
    FOR EACH ROW
    EXECUTE FUNCTION update_cluster_member_count();

-- Add constraints
ALTER TABLE face_results ADD CONSTRAINT chk_total_faces_nonnegative
CHECK (total_faces >= 0);

ALTER TABLE face_items ADD CONSTRAINT chk_confidence_range
CHECK (confidence >= 0.0 AND confidence <= 1.0);

ALTER TABLE face_items ADD CONSTRAINT chk_quality_range
CHECK (quality_score >= 0.0 AND quality_score <= 1.0);

ALTER TABLE face_items ADD CONSTRAINT chk_blur_range
CHECK (blur_score >= 0.0 AND blur_score <= 1.0);

ALTER TABLE face_cluster_members ADD CONSTRAINT chk_similarity_range
CHECK (similarity_score >= 0.0 AND similarity_score <= 1.0);

ALTER TABLE face_cluster_members ADD CONSTRAINT chk_assignment_confidence_range
CHECK (confidence >= 0.0 AND confidence <= 1.0);

-- Comments
COMMENT ON TABLE face_results IS 'Face detection results main table, storing face detection summary for each image';
COMMENT ON TABLE face_items IS 'Detected face items table, storing detailed information for each face';
COMMENT ON TABLE face_clusters IS 'Face recognition clusters table, grouping similar faces together';
COMMENT ON TABLE face_cluster_members IS 'Face cluster membership table, linking faces to clusters';

COMMENT ON COLUMN face_results.model_id IS 'Face detection model identifier, such as "retinaface-v1", "mtcnn-v1", etc.';
COMMENT ON COLUMN face_results.total_faces IS 'Total number of detected faces in the image';
COMMENT ON COLUMN face_results.processing_time_ms IS 'Face detection processing time in milliseconds';

COMMENT ON COLUMN face_items.face_id IS 'Face identifier for tracking the same person across images';
COMMENT ON COLUMN face_items.bounding_box IS 'Face bounding box coordinates, format: {x, y, width, height}';
COMMENT ON COLUMN face_items.confidence IS 'Face detection confidence score, between 0.0-1.0';
COMMENT ON COLUMN face_items.age_group IS 'Predicted age group: child, teen, adult, senior';
COMMENT ON COLUMN face_items.gender IS 'Predicted gender: male, female';
COMMENT ON COLUMN face_items.ethnicity IS 'Predicted ethnicity group';
COMMENT ON COLUMN face_items.expression IS 'Detected facial expression';
COMMENT ON COLUMN face_items.face_size IS 'Approximate face size in pixels';
COMMENT ON COLUMN face_items.embedding IS 'Face embedding vector for recognition and similarity matching';
COMMENT ON COLUMN face_items.is_primary IS 'Mark as the primary/most prominent face in the image';
COMMENT ON COLUMN face_items.quality_score IS 'Overall face quality score (0.0-1.0)';
COMMENT ON COLUMN face_items.blur_score IS 'Blur detection score (0.0-1.0, lower values indicate less blur)';
COMMENT ON COLUMN face_items.pose_angles IS 'Head pose angles in degrees, format: {yaw, pitch, roll}';

COMMENT ON COLUMN face_clusters.cluster_name IS 'User-assigned name for the person in this cluster';
COMMENT ON COLUMN face_clusters.confidence_score IS 'Overall confidence score for this cluster';
COMMENT ON COLUMN face_clusters.member_count IS 'Number of faces currently in this cluster';
COMMENT ON COLUMN face_clusters.is_confirmed IS 'Whether this cluster has been confirmed and named by user';

COMMENT ON COLUMN face_cluster_members.similarity_score IS 'Similarity score to cluster representative (0.0-1.0)';
COMMENT ON COLUMN face_cluster_members.confidence IS 'Assignment confidence to this cluster (0.0-1.0)';
COMMENT ON COLUMN face_cluster_members.is_manual IS 'Whether this face was manually assigned to this cluster';