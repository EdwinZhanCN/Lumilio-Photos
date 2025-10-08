-- Repositories table (must exist before assets for FK)
CREATE TABLE IF NOT EXISTS repositories (
    repo_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    config JSONB,
    status TEXT DEFAULT 'active',
    last_sync TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_repositories_status ON repositories(status);
CREATE INDEX IF NOT EXISTS idx_repositories_path ON repositories(path);

-- Enable vector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Universal Assets table
CREATE TABLE assets (
    asset_id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id           INTEGER REFERENCES users(user_id),
    type               VARCHAR(20)  NOT NULL CHECK (type IN ('PHOTO', 'VIDEO', 'AUDIO')),
    original_filename  VARCHAR(255) NOT NULL,
    storage_path       VARCHAR(512),
    mime_type          VARCHAR(50)  NOT NULL,
    file_size          BIGINT       NOT NULL,
    hash               VARCHAR(64),
    width              INTEGER,
    height             INTEGER,
    duration           DOUBLE PRECISION,
    upload_time        TIMESTAMPTZ   DEFAULT CURRENT_TIMESTAMP,
    taken_time         TIMESTAMPTZ,
    is_deleted         BOOLEAN       DEFAULT FALSE,
    deleted_at         TIMESTAMPTZ,
    specific_metadata  JSONB,
    rating             INTEGER,
    liked              BOOLEAN       DEFAULT FALSE,
    repository_id      UUID REFERENCES repositories(repo_id),
    embedding          VECTOR(512),
    status             JSONB NOT NULL DEFAULT '{"state": "processing", "message": "Pending processing"}',
    UNIQUE (repository_id, storage_path)
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
CREATE INDEX idx_assets_repository_id ON assets(repository_id);

-- Composite expression index for optimized type+taken_time queries
CREATE INDEX idx_assets_type_taken_time_coalesce
  ON assets(type, COALESCE(taken_time, upload_time) DESC)
  WHERE is_deleted = false;

-- Indexes for rating and liked columns
CREATE INDEX idx_assets_rating ON assets(rating) WHERE rating IS NOT NULL;
CREATE INDEX idx_assets_liked ON assets(liked) WHERE liked = true;
CREATE INDEX idx_assets_rating_liked ON assets(rating, liked) WHERE rating IS NOT NULL OR liked = true;

CREATE INDEX idx_thumbnails_asset_id ON thumbnails(asset_id);
CREATE INDEX idx_species_predictions_asset_id ON species_predictions(asset_id);

-- Vector index for embeddings
CREATE INDEX assets_hnsw_idx ON assets USING hnsw (embedding vector_l2_ops)
WITH (m = 16, ef_construction = 200);
