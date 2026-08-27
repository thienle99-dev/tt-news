CREATE TABLE IF NOT EXISTS translation_jobs (
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  language_code TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (article_id, language_code)
);
