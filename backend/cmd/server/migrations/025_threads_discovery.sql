ALTER TABLE threads_targets ADD COLUMN origin TEXT NOT NULL DEFAULT 'manual';
ALTER TABLE threads_targets ADD COLUMN auto_follow INTEGER NOT NULL DEFAULT 0;
ALTER TABLE threads_targets ADD COLUMN last_qualified_at TEXT NOT NULL DEFAULT '';

ALTER TABLE articles ADD COLUMN thread_classification TEXT NOT NULL DEFAULT '';
ALTER TABLE articles ADD COLUMN thread_classification_source TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_threads_targets_auto_follow ON threads_targets(auto_follow, enabled);

INSERT OR IGNORE INTO threads_targets(kind,query,enabled,origin) VALUES
  ('keyword','Việt Nam',1,'seed'),
  ('keyword','Hà Nội',1,'seed'),
  ('keyword','TPHCM',1,'seed'),
  ('keyword','tin nóng',1,'seed'),
  ('keyword','công nghệ',1,'seed'),
  ('keyword','AI',1,'seed'),
  ('keyword','chứng khoán',1,'seed'),
  ('keyword','bất động sản',1,'seed');
