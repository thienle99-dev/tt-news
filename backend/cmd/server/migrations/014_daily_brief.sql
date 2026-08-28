CREATE TABLE IF NOT EXISTS featured_brief_takeaways (
  brief_id INTEGER NOT NULL REFERENCES featured_briefs(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  text TEXT NOT NULL,
  PRIMARY KEY (brief_id, position)
);

CREATE TABLE IF NOT EXISTS featured_brief_takeaway_translations (
  brief_id INTEGER NOT NULL REFERENCES featured_briefs(id) ON DELETE CASCADE,
  language_code TEXT NOT NULL,
  position INTEGER NOT NULL,
  text TEXT NOT NULL,
  PRIMARY KEY (brief_id, language_code, position)
);
