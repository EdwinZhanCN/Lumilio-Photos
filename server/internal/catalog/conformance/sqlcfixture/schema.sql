CREATE TABLE spike_records (
    id TEXT PRIMARY KEY,
    optional_parent_id TEXT,
    created_at INTEGER NOT NULL,
    payload TEXT NOT NULL CHECK (json_valid(payload)),
    embedding BLOB NOT NULL
) STRICT;
