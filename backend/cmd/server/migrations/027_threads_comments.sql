CREATE TABLE IF NOT EXISTS threads_post_comments (
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  thread_comment_id TEXT NOT NULL,
  author TEXT NOT NULL,
  display_name TEXT NOT NULL DEFAULT '',
  avatar_url TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL,
  published_at TEXT NOT NULL,
  likes INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(article_id, thread_comment_id)
);

CREATE INDEX IF NOT EXISTS idx_threads_post_comments_article_rank
  ON threads_post_comments(article_id, likes DESC, published_at DESC);

CREATE TABLE IF NOT EXISTS threads_comment_fetches (
  article_id INTEGER PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
  fetched_at TEXT NOT NULL
);
