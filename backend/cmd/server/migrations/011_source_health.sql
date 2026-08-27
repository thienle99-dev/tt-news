CREATE TABLE IF NOT EXISTS source_health (
  source_id INTEGER PRIMARY KEY REFERENCES sources(id) ON DELETE CASCADE,
  last_fetch_at TEXT NOT NULL DEFAULT '',
  last_success_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  last_inserted INTEGER NOT NULL DEFAULT 0
);
