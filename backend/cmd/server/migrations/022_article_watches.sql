CREATE TABLE IF NOT EXISTS article_watches (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind TEXT NOT NULL CHECK(kind IN ('article','topic')),
  article_id INTEGER REFERENCES articles(id) ON DELETE CASCADE,
  category_id INTEGER REFERENCES categories(id) ON DELETE CASCADE,
  keyword TEXT NOT NULL DEFAULT '',
  enabled INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK((kind='article' AND article_id IS NOT NULL AND category_id IS NULL) OR (kind='topic' AND category_id IS NOT NULL AND article_id IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_article_watches_article_unique ON article_watches(user_id, article_id) WHERE article_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_article_watches_topic_unique ON article_watches(user_id, category_id) WHERE category_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_article_watches_enabled ON article_watches(enabled, kind);

CREATE TABLE IF NOT EXISTS article_watch_alerts (
  watch_id INTEGER NOT NULL REFERENCES article_watches(id) ON DELETE CASCADE,
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  status TEXT NOT NULL CHECK(status IN ('sent','failed')),
  error TEXT NOT NULL DEFAULT '',
  sent_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(watch_id, article_id)
);
