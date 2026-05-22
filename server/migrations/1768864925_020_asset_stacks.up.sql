-- Asset Stacks for extensible photo grouping

CREATE TYPE stack_relation AS ENUM (
    'raw_original',
    'jpeg_original',
    'edited_version',
    'alternative',
    'live_photo_still',
    'live_photo_video'
);

CREATE TABLE asset_stacks (
    stack_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stack_kind TEXT NOT NULL DEFAULT 'manual',
    group_key TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (stack_id)
);

CREATE TABLE asset_stack_members (
    asset_id UUID NOT NULL REFERENCES assets(asset_id) ON DELETE CASCADE,
    stack_id UUID NOT NULL REFERENCES asset_stacks(stack_id) ON DELETE CASCADE,
    relation stack_relation NOT NULL DEFAULT 'alternative',
    position INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (asset_id)
);

CREATE INDEX idx_asset_stack_members_stack ON asset_stack_members(stack_id);
