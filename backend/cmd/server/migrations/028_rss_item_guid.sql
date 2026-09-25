ALTER TABLE articles ADD COLUMN rss_guid_hash TEXT NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_source_rss_guid_hash
  ON articles(source_id, rss_guid_hash)
  WHERE rss_guid_hash <> '';
