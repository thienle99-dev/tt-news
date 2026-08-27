package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	translationservice "telegram-news/internal/translation"
)

func (s *server) runTranslationWorker(ctx context.Context) {
	if s.cfg.AIURL == "" || s.cfg.AIKey == "" {
		log.Print("RSS translation worker disabled: set AI_URL and AI_KEY to enable it")
		return
	}
	for ctx.Err() == nil {
		processed, err := s.processTranslationJob(ctx)
		if err != nil {
			log.Printf("RSS translation worker: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}
		if processed {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (s *server) processTranslationJob(ctx context.Context) (bool, error) {
	s.translationMu.Lock()
	defer s.translationMu.Unlock()
	var articleID int64
	var title, summary string
	err := s.db.QueryRowContext(ctx, `SELECT tj.article_id,a.title,a.summary FROM translation_jobs tj JOIN articles a ON a.id=tj.article_id WHERE tj.language_code='vi' ORDER BY tj.created_at,tj.article_id LIMIT 1`).Scan(&articleID, &title, &summary)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	translated, err := (translationservice.Client{URL: s.cfg.AIURL, APIKey: s.cfg.AIKey, Model: s.cfg.AIModel}).Vietnamese(ctx, translationservice.Fields{Title: title, Summary: summary})
	if err != nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE translation_jobs SET created_at=CURRENT_TIMESTAMP WHERE article_id=? AND language_code='vi'`, articleID)
		return true, err
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO article_translations(article_id,language_code,title,description,summary) VALUES(?,'vi',?,?,?) ON CONFLICT(article_id,language_code) DO UPDATE SET title=excluded.title,description=excluded.description,summary=excluded.summary`, articleID, translated.Title, "", translated.Summary); err != nil {
		return true, err
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM translation_jobs WHERE article_id=? AND language_code='vi'`, articleID); err != nil {
		return true, err
	}
	return true, nil
}
