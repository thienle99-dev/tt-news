CREATE TABLE IF NOT EXISTS threads_targets (
  id INTEGER PRIMARY KEY,
  kind TEXT NOT NULL CHECK(kind IN ('profile','keyword')),
  query TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 1,
  last_fetch_at TEXT NOT NULL DEFAULT '',
  last_success_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  last_inserted INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(kind, query)
);

CREATE TABLE IF NOT EXISTS article_threads_targets (
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  target_id INTEGER NOT NULL REFERENCES threads_targets(id) ON DELETE CASCADE,
  PRIMARY KEY(article_id, target_id)
);
CREATE INDEX IF NOT EXISTS idx_article_threads_targets_target ON article_threads_targets(target_id, article_id);
