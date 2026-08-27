CREATE TABLE IF NOT EXISTS article_categories (
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  PRIMARY KEY (article_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_article_categories_category ON article_categories(category_id, article_id);

-- Existing articles had one primary category. Preserve that relation while
-- future RSS imports can attach every category exposed by an item.
INSERT OR IGNORE INTO article_categories(article_id, category_id)
SELECT id, category_id FROM articles;
