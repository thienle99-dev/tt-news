CREATE TABLE IF NOT EXISTS article_ai_feedback (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  issue_type TEXT NOT NULL CHECK(issue_type IN ('incorrect', 'missing')),
  reason TEXT NOT NULL,
  summary_snapshot TEXT NOT NULL DEFAULT '',
  original_content_snapshot TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_article_ai_feedback_article_created
  ON article_ai_feedback(article_id, created_at DESC);
