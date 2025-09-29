-- Repository table migration (up) for Lumilio Photos
-- Creates the repositories table to manage photo repositories/collections
-- Requires extensions created by 000001_extensions.up.sql (pgcrypto for gen_random_uuid).

-- Repository Definition
CREATE TABLE repositories (
    repo_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    config JSONB,
    status TEXT DEFAULT 'active',
    last_sync TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_repositories_status ON repositories(status);
CREATE INDEX idx_repositories_path ON repositories(path);
