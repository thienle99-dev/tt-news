CREATE TABLE IF NOT EXISTS article_translations (
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  language_code TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  summary TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (article_id, language_code)
);
