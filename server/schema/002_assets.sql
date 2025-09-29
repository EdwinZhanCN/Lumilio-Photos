-- Enable vector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Universal Assets table
CREATE TABLE assets (
    asset_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id INTEGER REFERENCES users(user_id),
    type VARCHAR(20) NOT NULL CHECK (type IN ('PHOTO', 'VIDEO', 'AUDIO')),
    original_filename VARCHAR(255) NOT NULL,
    storage_path VARCHAR(50) NOT NULL,
    mime_type VARCHAR(50) NOT NULL,
    file_size BIGINT NOT NULL,
    hash VARCHAR(64),
    width INTEGER,
    height INTEGER,
    duration DOUBLE PRECISION,
    upload_time TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    taken_time TIMESTAMPTZ,
    is_deleted BOOLEAN DEFAULT false,
    deleted_at TIMESTAMPTZ,
    specific_metadata JSONB,
    rating INTEGER,
	liked BOOLEAN DEFAULT false,
    embedding VECTOR(512)
);

-- Thumbnails table
CREATE TABLE thumbnails (
    thumbnail_id SERIAL PRIMARY KEY,
    asset_id UUID NOT NULL REFERENCES assets(asset_id),
    size VARCHAR(20) NOT NULL CHECK (size IN ('small', 'medium', 'large')),
    storage_path VARCHAR(512) NOT NULL,
    mime_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Species predictions table
CREATE TABLE species_predictions (
    asset_id UUID NOT NULL REFERENCES assets(asset_id),
    label VARCHAR(255) NOT NULL,
    score REAL NOT NULL,
    PRIMARY KEY (asset_id, label)
);

-- Indexes
CREATE INDEX idx_assets_owner_id ON assets(owner_id);
CREATE INDEX idx_assets_type ON assets(type);
CREATE INDEX idx_assets_hash ON assets(hash);
CREATE INDEX idx_assets_taken_time ON assets(taken_time);
CREATE INDEX idx_thumbnails_asset_id ON thumbnails(asset_id);
CREATE INDEX idx_species_predictions_asset_id ON species_predictions(asset_id);

-- Vector index for embeddings
CREATE INDEX assets_hnsw_idx ON assets USING hnsw (embedding vector_l2_ops)
WITH (m = 16, ef_construction = 200);
