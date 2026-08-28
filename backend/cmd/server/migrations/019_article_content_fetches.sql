CREATE TABLE IF NOT EXISTS article_content_fetches (
  article_id INTEGER PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
  attempts INTEGER NOT NULL DEFAULT 0,
  attempted_at TEXT NOT NULL,
  last_error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_article_content_fetches_attempted ON article_content_fetches(attempted_at);
