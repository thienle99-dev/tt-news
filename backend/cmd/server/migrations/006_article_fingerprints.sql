CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_title_fingerprint
  ON articles(title_fingerprint)
  WHERE title_fingerprint <> '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_content_fingerprint
  ON articles(content_fingerprint)
  WHERE content_fingerprint <> '';
