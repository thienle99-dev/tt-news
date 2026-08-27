package main

import (
	"context"
	"log"
	"strings"
	"time"

	translationservice "telegram-news/internal/translation"
)

const cleanupReviewVersion = "weather-entertainment-v1"

func (s *server) runContentCleanupWorker(ctx context.Context) {
	if s.cfg.AIURL == "" || s.cfg.AIKey == "" {
		log.Print("content cleanup worker disabled: set AI_URL and AI_KEY to enable it")
		return
	}
	s.cleanupJunkArticles(ctx)
	ticker := time.NewTicker(s.cfg.ContentCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done(): return
		case <-ticker.C: s.cleanupJunkArticles(ctx)
		}
	}
}

func (s *server) cleanupJunkArticles(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.title,a.summary FROM articles a WHERE (a.content_reviewed_at='' OR a.content_review_version<>?) AND a.summary<>'' AND NOT EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.article_id=a.id) AND NOT EXISTS(SELECT 1 FROM featured_topic_articles fta WHERE fta.article_id=a.id) ORDER BY a.published_at ASC,a.id ASC LIMIT ?`, cleanupReviewVersion, s.cfg.ContentCleanupLimit)
	if err != nil { log.Printf("content cleanup candidates: %v", err); return }
	defer rows.Close()
	type candidate struct { id int64; title, summary string }
	candidates := []candidate{}
	for rows.Next() { var item candidate; if err = rows.Scan(&item.id, &item.title, &item.summary); err != nil { log.Printf("content cleanup read: %v", err); return }; candidates = append(candidates, item) }
	if err = rows.Err(); err != nil { log.Printf("content cleanup rows: %v", err); return }
	client := translationservice.Client{URL: s.cfg.AIURL, APIKey: s.cfg.AIKey, Model: s.cfg.AIModel}
	removed, removedWithoutAI, reviewed := 0, 0, 0
	for _, item := range candidates {
		if isObviousJunk(item.title) {
			deleted, deleteErr := s.deleteJunkArticle(ctx, item.id)
			if deleteErr != nil { log.Printf("content cleanup delete %d: %v", item.id, deleteErr); continue }
			if deleted { removed++; removedWithoutAI++ }
			continue
		}
		junk, reviewErr := client.IsJunk(ctx, item.title, item.summary)
		if reviewErr != nil { log.Printf("content cleanup review %d: %v", item.id, reviewErr); continue }
		reviewed++
		if junk {
			deleted, deleteErr := s.deleteJunkArticle(ctx, item.id)
			if deleteErr != nil { log.Printf("content cleanup delete %d: %v", item.id, deleteErr); continue }
			if deleted { removed++ }
			continue
		}
		if _, err = s.db.ExecContext(ctx, "UPDATE articles SET content_reviewed_at=?,content_review_version=? WHERE id=?", time.Now().UTC().Format(time.RFC3339), cleanupReviewVersion, item.id); err != nil { log.Printf("content cleanup mark %d: %v", item.id, err) }
	}
	if reviewed > 0 || removedWithoutAI > 0 { log.Printf("content cleanup finished: reviewed=%d removed=%d removed-without-ai=%d", reviewed, removed, removedWithoutAI) }
}

func (s *server) deleteJunkArticle(ctx context.Context, id int64) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM articles WHERE id=? AND NOT EXISTS(SELECT 1 FROM saved_articles WHERE article_id=articles.id) AND NOT EXISTS(SELECT 1 FROM featured_topic_articles WHERE article_id=articles.id)`, id)
	if err != nil { return false, err }
	count, _ := result.RowsAffected()
	return count > 0, nil
}

func isObviousJunk(title string) bool {
	value := strings.ToLower(strings.TrimSpace(title))
	for _, phrase := range []string{
		"weather forecast", "weather outlook", "weather update", "weekly weather", "horoscope",
		"podcast:", "podcast |", "listen:", "watch:", "video:", "photo gallery", "in pictures",
		"celebrity", "red carpet", "fashion week", "recipe", "restaurant review", "travel guide",
	} {
		if strings.Contains(value, phrase) { return true }
	}
	return false
}
