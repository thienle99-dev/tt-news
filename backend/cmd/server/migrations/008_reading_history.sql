CREATE TABLE IF NOT EXISTS article_reading_history (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'reading' CHECK(status IN ('reading', 'read')),
  first_opened_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_opened_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  read_at TEXT,
  PRIMARY KEY (user_id, article_id)
);

CREATE INDEX IF NOT EXISTS idx_article_reading_history_user_recent
  ON article_reading_history(user_id, last_opened_at DESC);
CREATE INDEX IF NOT EXISTS idx_article_reading_history_user_status_recent
  ON article_reading_history(user_id, status, last_opened_at DESC);
