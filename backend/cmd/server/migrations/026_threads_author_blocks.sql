CREATE TABLE IF NOT EXISTS threads_author_blocks (
  username TEXT PRIMARY KEY,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_threads_author_blocks_username ON threads_author_blocks(username);
