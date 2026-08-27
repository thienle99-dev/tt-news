CREATE TABLE IF NOT EXISTS featured_briefs (
  id INTEGER PRIMARY KEY,
  slot_start TEXT NOT NULL UNIQUE,
  generated_at TEXT NOT NULL,
  window_start TEXT NOT NULL,
  window_end TEXT NOT NULL,
  title TEXT NOT NULL,
  intro TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS featured_brief_translations (
  brief_id INTEGER NOT NULL REFERENCES featured_briefs(id) ON DELETE CASCADE,
  language_code TEXT NOT NULL,
  title TEXT NOT NULL,
  intro TEXT NOT NULL,
  PRIMARY KEY (brief_id, language_code)
);

CREATE TABLE IF NOT EXISTS featured_topics (
  id INTEGER PRIMARY KEY,
  brief_id INTEGER NOT NULL REFERENCES featured_briefs(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  title TEXT NOT NULL,
  summary TEXT NOT NULL,
  UNIQUE (brief_id, position)
);

CREATE TABLE IF NOT EXISTS featured_topic_translations (
  topic_id INTEGER NOT NULL REFERENCES featured_topics(id) ON DELETE CASCADE,
  language_code TEXT NOT NULL,
  title TEXT NOT NULL,
  summary TEXT NOT NULL,
  PRIMARY KEY (topic_id, language_code)
);

CREATE TABLE IF NOT EXISTS featured_topic_articles (
  topic_id INTEGER NOT NULL REFERENCES featured_topics(id) ON DELETE CASCADE,
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  PRIMARY KEY (topic_id, article_id),
  UNIQUE (topic_id, position)
);
