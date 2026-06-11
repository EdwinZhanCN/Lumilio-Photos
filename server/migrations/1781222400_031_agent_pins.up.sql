-- Agent pinned widgets: refs promoted from session memory to durable storage.
-- A pin owns a frozen snapshot of asset ids plus the plan that produced it;
-- live-mode pins replay the plan on hydration when it is self-contained
-- (producer ops only). Layout fields back the react-grid-layout board.
CREATE TABLE agent_pins (
    pin_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    title TEXT NOT NULL DEFAULT '',
    widget TEXT NOT NULL DEFAULT 'asset_grid',
    mode TEXT NOT NULL DEFAULT 'frozen' CHECK (mode IN ('frozen', 'live')),
    plan JSONB NOT NULL DEFAULT '{}'::jsonb,
    summary TEXT NOT NULL DEFAULT '',
    asset_ids UUID[] NOT NULL DEFAULT '{}',
    truncated BOOLEAN NOT NULL DEFAULT FALSE,
    layout_x INTEGER NOT NULL DEFAULT 0,
    layout_y INTEGER NOT NULL DEFAULT 0,
    layout_w INTEGER NOT NULL DEFAULT 4,
    layout_h INTEGER NOT NULL DEFAULT 4,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_agent_pins_user ON agent_pins(user_id, created_at DESC);
