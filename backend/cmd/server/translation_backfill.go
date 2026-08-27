package main

import (
	"context"
	"log"

	translationservice "telegram-news/internal/translation"
)

func runTranslateAllVietnamese(cfg config) {
	if cfg.AIURL == "" || cfg.AIKey == "" {
		log.Fatal("set AI_URL and AI_KEY before running translate-all-vi")
	}
	db, err := openDB(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		log.Fatal(err)
	}
	rows, err := db.Query(`SELECT id,title,summary FROM articles WHERE NOT EXISTS (SELECT 1 FROM article_translations tr WHERE tr.article_id=articles.id AND tr.language_code='vi') ORDER BY id`)
	if err != nil {
		log.Fatal(err)
	}
	type pending struct {
		id             int64
		title, summary string
	}
	items := []pending{}
	for rows.Next() {
		var item pending
		if err = rows.Scan(&item.id, &item.title, &item.summary); err != nil {
			log.Printf("translation row: %v", err)
			continue
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}
	rows.Close()
	client := translationservice.Client{URL: cfg.AIURL, APIKey: cfg.AIKey, Model: cfg.AIModel}
	translated, failed := 0, 0
	for _, item := range items {
		out, translateErr := client.Vietnamese(context.Background(), translationservice.Fields{Title: item.title, Summary: item.summary})
		if translateErr != nil {
			log.Printf("translate article %d: %v", item.id, translateErr)
			failed++
			continue
		}
		if _, err = db.Exec(`INSERT INTO article_translations(article_id,language_code,title,description,summary) VALUES(?,'vi',?,?,?) ON CONFLICT(article_id,language_code) DO UPDATE SET title=excluded.title,description=excluded.description,summary=excluded.summary`, item.id, out.Title, "", out.Summary); err != nil {
			log.Printf("save translation %d: %v", item.id, err)
			failed++
			continue
		}
		translated++
	}
	log.Printf("Vietnamese translation backfill finished: %d translated, %d failed", translated, failed)
}
