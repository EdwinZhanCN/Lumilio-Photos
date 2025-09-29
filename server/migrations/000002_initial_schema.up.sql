-- Initial schema (up) for Lumilio Photos
-- Includes: users, refresh_tokens, assets, thumbnails, species_predictions,
-- albums, album_assets, tags, asset_tags, and relevant indexes.
-- Requires extensions created by 000001_extensions.up.sql (pgcrypto, vector).

-- 1) Users
CREATE TABLE users (
    user_id     SERIAL PRIMARY KEY,
    username    VARCHAR(50)  NOT NULL UNIQUE,
    email       VARCHAR(100) NOT NULL UNIQUE,
    password    VARCHAR(255) NOT NULL,
    created_at  TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    is_active   BOOLEAN      DEFAULT TRUE,
    last_login  TIMESTAMPTZ
);

-- 2) Refresh Tokens
CREATE TABLE refresh_tokens (
    token_id    SERIAL PRIMARY KEY,
    user_id     INTEGER      NOT NULL REFERENCES users(user_id),
    token       VARCHAR(255) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ  NOT NULL,
    created_at  TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    is_revoked  BOOLEAN      DEFAULT FALSE
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_tokens_token ON refresh_tokens(token);

-- 3) Assets (universal)
CREATE TABLE assets (
    asset_id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id           INTEGER REFERENCES users(user_id),
    type               VARCHAR(20)  NOT NULL CHECK (type IN ('PHOTO', 'VIDEO', 'AUDIO')),
    original_filename  VARCHAR(255) NOT NULL,
    storage_path       VARCHAR(50)  NOT NULL,
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
    embedding          VECTOR(512)
);

CREATE INDEX idx_assets_owner_id ON assets(owner_id);
CREATE INDEX idx_assets_type ON assets(type);
CREATE INDEX idx_assets_hash ON assets(hash);
CREATE INDEX idx_assets_taken_time ON assets(taken_time);

-- Composite expression index for optimized type+taken_time queries
CREATE INDEX idx_assets_type_taken_time_coalesce
  ON assets(type, COALESCE(taken_time, upload_time) DESC)
  WHERE is_deleted = false;

-- Vector HNSW index for semantic search on embeddings
CREATE INDEX assets_hnsw_idx ON assets USING hnsw (embedding vector_l2_ops)
WITH (m = 16, ef_construction = 200);

-- Indexes for rating and liked columns for better query performance
CREATE INDEX idx_assets_rating ON assets(rating) WHERE rating IS NOT NULL;
CREATE INDEX idx_assets_liked ON assets(liked) WHERE liked = true;
CREATE INDEX idx_assets_rating_liked ON assets(rating, liked) WHERE rating IS NOT NULL OR liked = true;

-- 4) Thumbnails
CREATE TABLE thumbnails (
    thumbnail_id  SERIAL PRIMARY KEY,
    asset_id      UUID         NOT NULL REFERENCES assets(asset_id),
    size          VARCHAR(20)  NOT NULL CHECK (size IN ('small', 'medium', 'large')),
    storage_path  VARCHAR(512) NOT NULL,
    mime_type     VARCHAR(50)  NOT NULL,
    created_at    TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_thumbnails_asset_id ON thumbnails(asset_id);

-- 5) Species Predictions
CREATE TABLE species_predictions (
    asset_id  UUID         NOT NULL REFERENCES assets(asset_id),
    label     VARCHAR(255) NOT NULL,
    score     REAL         NOT NULL,
    PRIMARY KEY (asset_id, label)
);

CREATE INDEX idx_species_predictions_asset_id ON species_predictions(asset_id);

-- 6) Tags
CREATE TABLE tags (
    tag_id          SERIAL PRIMARY KEY,
    tag_name        VARCHAR(50) UNIQUE NOT NULL,
    category        VARCHAR(50),
    is_ai_generated BOOLEAN DEFAULT TRUE
);

-- 7) Asset Tags (junction)
CREATE TABLE asset_tags (
    asset_id   UUID    NOT NULL REFERENCES assets(asset_id),
    tag_id     INTEGER NOT NULL REFERENCES tags(tag_id),
    confidence NUMERIC(4,3) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    source     VARCHAR(20)  NOT NULL DEFAULT 'system' CHECK (source IN ('system', 'user', 'ai')),
    PRIMARY KEY (asset_id, tag_id)
);

-- 8) Albums
CREATE TABLE albums (
    album_id       SERIAL PRIMARY KEY,
    user_id        INTEGER      NOT NULL REFERENCES users(user_id),
    album_name     VARCHAR(100) NOT NULL,
    created_at     TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    description    TEXT,
    cover_asset_id UUID REFERENCES assets(asset_id)
);

CREATE INDEX idx_albums_user_id ON albums(user_id);

-- 9) Album Assets (junction)
CREATE TABLE album_assets (
    album_id   INTEGER NOT NULL REFERENCES albums(album_id),
    asset_id   UUID    NOT NULL REFERENCES assets(asset_id),
    position   INTEGER DEFAULT 0,
    added_time TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (album_id, asset_id)
);
