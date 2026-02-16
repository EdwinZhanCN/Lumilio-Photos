CREATE TABLE IF NOT EXISTS plugins (
  plugin_id TEXT PRIMARY KEY,
  display_name TEXT NOT NULL,
  description TEXT,
  panel TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS plugin_releases (
  plugin_id TEXT NOT NULL,
  version TEXT NOT NULL,
  channel TEXT NOT NULL DEFAULT 'stable',
  manifest_json TEXT NOT NULL,
  is_active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (plugin_id, version),
  FOREIGN KEY (plugin_id) REFERENCES plugins(plugin_id)
);

CREATE TABLE IF NOT EXISTS plugin_revocations (
  plugin_id TEXT NOT NULL,
  version TEXT NOT NULL,
  reason TEXT,
  active INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (plugin_id, version)
);

CREATE INDEX IF NOT EXISTS idx_plugins_panel_status
  ON plugins(panel, status);

CREATE INDEX IF NOT EXISTS idx_releases_lookup
  ON plugin_releases(plugin_id, channel, is_active, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_revocations_active
  ON plugin_revocations(active, plugin_id, version);
