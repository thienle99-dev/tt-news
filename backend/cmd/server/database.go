package main

import (
	"database/sql"
	"strings"
)

func health(dbPath string) bool {
	db, err := openDB(dbPath)
	if err != nil {
		return false
	}
	defer db.Close()
	return db.Ping() == nil
}

func openDB(file string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	for _, query := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err = db.Exec(query); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	schema, err := embedded.ReadFile("migrations/001_init.sql")
	if err != nil {
		return err
	}
	if _, err = db.Exec(string(schema)); err != nil {
		return err
	}

	if _, err = db.Exec("ALTER TABLE articles ADD COLUMN summary TEXT NOT NULL DEFAULT ''"); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}

	categories := []struct{ slug, name string }{
		{"technology", "Technology"},
		{"world", "World"},
		{"business", "Business"},
	}
	for _, category := range categories {
		if _, err = db.Exec("INSERT OR IGNORE INTO categories(slug,name) VALUES(?,?)", category.slug, category.name); err != nil {
			return err
		}
	}

	sources := []struct{ name, url, slug string }{
		{"Hacker News", "https://hnrss.org/frontpage", "technology"},
		{"BBC World", "https://feeds.bbci.co.uk/news/world/rss.xml", "world"},
		{"BBC Business", "https://feeds.bbci.co.uk/news/business/rss.xml", "business"},
	}
	for _, source := range sources {
		if _, err = db.Exec("INSERT OR IGNORE INTO sources(name,feed_url,category_id) VALUES(?,?,(SELECT id FROM categories WHERE slug=?))", source.name, source.url, source.slug); err != nil {
			return err
		}
	}
	return nil
}
