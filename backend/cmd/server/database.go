package main

import (
	"database/sql"
	"strings"

	"telegram-news/internal/crawlers/rss"
	"telegram-news/internal/crawlers/scmp"
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
	var err error
	for _, name := range []string{"migrations/001_init.sql", "migrations/002_translations.sql"} {
		var schema []byte
		schema, err = embedded.ReadFile(name)
		if err != nil {
			return err
		}
		if _, err = db.Exec(string(schema)); err != nil {
			return err
		}
	}

	if _, err = db.Exec("ALTER TABLE articles ADD COLUMN summary TEXT NOT NULL DEFAULT ''"); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		return err
	}
	for _, statement := range []string{
		"ALTER TABLE sources ADD COLUMN country_code TEXT NOT NULL DEFAULT 'GLOBAL'",
		"ALTER TABLE sources ADD COLUMN country_name TEXT NOT NULL DEFAULT 'Toàn cầu'",
	} {
		if _, err = db.Exec(statement); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}

	categories := []struct{ slug, name string }{
		{"technology", "Công nghệ"},
		{"world", "Thế giới"},
		{"business", "Kinh doanh"},
		{"society", "Xã hội"},
	}
	for _, category := range categories {
		if _, err = db.Exec("INSERT INTO categories(slug,name) VALUES(?,?) ON CONFLICT(slug) DO UPDATE SET name=excluded.name", category.slug, category.name); err != nil {
			return err
		}
	}

	sources := []rss.Source{
		{Name: "Hacker News", URL: "https://hnrss.org/frontpage", Category: "technology", CountryCode: "US", CountryName: "Hoa Kỳ"},
		{Name: "BBC World", URL: "https://feeds.bbci.co.uk/news/world/rss.xml", Category: "world", CountryCode: "GB", CountryName: "Vương quốc Anh"},
		{Name: "BBC Business", URL: "https://feeds.bbci.co.uk/news/business/rss.xml", Category: "business", CountryCode: "GB", CountryName: "Vương quốc Anh"},
	}
	sources = append(sources, scmp.Feeds...)
	for _, source := range sources {
		if source.CountryCode == "" {
			source.CountryCode, source.CountryName = "GLOBAL", "Toàn cầu"
		}
		if _, err = db.Exec(`INSERT INTO sources(name,feed_url,category_id,country_code,country_name) VALUES(?,?,(SELECT id FROM categories WHERE slug=?),?,?) ON CONFLICT(feed_url) DO UPDATE SET country_code=excluded.country_code,country_name=excluded.country_name`, source.Name, source.URL, source.Category, source.CountryCode, source.CountryName); err != nil {
			return err
		}
	}
	return nil
}
