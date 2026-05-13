-- Duplicate detection: groups of duplicate photos sharing exact hash and/or pHash similarity.
-- Each duplicate_group represents a connected component over the duplicate edge graph,
-- where edges may come from exact (BLAKE3 hash + file_size) or perceptual (pHash) matches.

CREATE TABLE duplicate_groups (
    group_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    repository_id UUID NOT NULL REFERENCES repositories(repo_id) ON DELETE CASCADE,
    -- Component-level method: 'exact' when every edge is exact, 'phash' when every edge is phash,
    -- or 'mixed' when the component contains both. Drives default merge behavior in UI.
    method TEXT NOT NULL CHECK (method IN ('exact', 'phash', 'mixed')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'merged', 'dismissed')),
    asset_count INTEGER NOT NULL DEFAULT 0,
    total_size BIGINT NOT NULL DEFAULT 0,
    -- Best candidate asset for the user to keep (computed at detection time).
    -- May be overridden by the user at merge time.
    recommended_keeper_asset_id UUID REFERENCES assets(asset_id) ON DELETE SET NULL,
    -- Final keeper chosen at merge time. NULL while pending or dismissed.
    keeper_asset_id UUID REFERENCES assets(asset_id) ON DELETE SET NULL,
    detection_version TEXT NOT NULL DEFAULT 'duplicates-v1',
    detected_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_duplicate_groups_repo_status
    ON duplicate_groups(repository_id, status, detected_at DESC);

CREATE INDEX idx_duplicate_groups_status
    ON duplicate_groups(status);

CREATE TABLE duplicate_group_assets (
    group_id UUID NOT NULL REFERENCES duplicate_groups(group_id) ON DELETE CASCADE,
    asset_id UUID NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    role TEXT NOT NULL DEFAULT 'candidate'
        CHECK (role IN ('candidate', 'keeper', 'duplicate')),
    file_size BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (group_id, asset_id)
);

CREATE INDEX idx_duplicate_group_assets_asset ON duplicate_group_assets(asset_id);

CREATE TABLE duplicate_group_edges (
    group_id UUID NOT NULL REFERENCES duplicate_groups(group_id) ON DELETE CASCADE,
    -- Edge endpoints are stored in canonical order (asset_id_a < asset_id_b lexicographically)
    -- so the same pair cannot be inserted twice with swapped endpoints.
    asset_id_a UUID NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    asset_id_b UUID NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    method TEXT NOT NULL CHECK (method IN ('exact', 'phash')),
    distance DOUBLE PRECISION NOT NULL DEFAULT 0,
    confidence DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    PRIMARY KEY (group_id, asset_id_a, asset_id_b, method),
    CHECK (asset_id_a < asset_id_b)
);

CREATE INDEX idx_duplicate_group_edges_assets
    ON duplicate_group_edges(asset_id_a, asset_id_b);
