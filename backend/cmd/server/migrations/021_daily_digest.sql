CREATE TABLE IF NOT EXISTS user_daily_digests (
  user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  enabled INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_daily_digest_categories (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, category_id)
);

CREATE TABLE IF NOT EXISTS user_daily_digest_sources (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source_id INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, source_id)
);

CREATE TABLE IF NOT EXISTS daily_digest_deliveries (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  day TEXT NOT NULL,
  status TEXT NOT NULL CHECK(status IN ('sent','failed')),
  article_count INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, day)
);

CREATE INDEX IF NOT EXISTS idx_daily_digest_deliveries_day_status ON daily_digest_deliveries(day, status);
