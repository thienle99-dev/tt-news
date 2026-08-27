CREATE TABLE IF NOT EXISTS saved_folders (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL COLLATE NOCASE,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(user_id, name)
);

CREATE TABLE IF NOT EXISTS saved_tags (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL COLLATE NOCASE,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(user_id, name)
);

CREATE TABLE IF NOT EXISTS saved_article_folders (
  user_id INTEGER NOT NULL,
  article_id INTEGER NOT NULL,
  folder_id INTEGER NOT NULL REFERENCES saved_folders(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, article_id, folder_id),
  FOREIGN KEY (user_id, article_id) REFERENCES saved_articles(user_id, article_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_saved_article_folders_folder ON saved_article_folders(folder_id, article_id);

CREATE TABLE IF NOT EXISTS saved_article_tags (
  user_id INTEGER NOT NULL,
  article_id INTEGER NOT NULL,
  tag_id INTEGER NOT NULL REFERENCES saved_tags(id) ON DELETE CASCADE,
  PRIMARY KEY (user_id, article_id, tag_id),
  FOREIGN KEY (user_id, article_id) REFERENCES saved_articles(user_id, article_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_saved_article_tags_tag ON saved_article_tags(tag_id, article_id);
