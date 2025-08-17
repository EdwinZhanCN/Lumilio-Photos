-- Tags table
CREATE TABLE tags (
    tag_id SERIAL PRIMARY KEY,
    tag_name VARCHAR(50) UNIQUE NOT NULL,
    category VARCHAR(50),
    is_ai_generated BOOLEAN DEFAULT true
);

-- Asset tags junction table
CREATE TABLE asset_tags (
    asset_id UUID NOT NULL REFERENCES assets(asset_id),
    tag_id INTEGER NOT NULL REFERENCES tags(tag_id),
    confidence NUMERIC(4,3) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    source VARCHAR(20) NOT NULL DEFAULT 'system' CHECK (source IN ('system', 'user', 'ai')),
    PRIMARY KEY (asset_id, tag_id)
);

-- Albums table
CREATE TABLE albums (
    album_id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(user_id),
    album_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    description TEXT,
    cover_asset_id UUID REFERENCES assets(asset_id)
);

-- Album assets junction table
CREATE TABLE album_assets (
    album_id INTEGER NOT NULL REFERENCES albums(album_id),
    asset_id UUID NOT NULL REFERENCES assets(asset_id),
    position INTEGER DEFAULT 0,
    added_time TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (album_id, asset_id)
);

-- Indexes
CREATE INDEX idx_albums_user_id ON albums(user_id);
