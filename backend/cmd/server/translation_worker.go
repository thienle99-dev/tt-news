package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	translationservice "telegram-news/internal/translation"
)

func (s *server) runTranslationWorker(ctx context.Context) {
	if !strings.EqualFold(s.cfg.AITranslateLanguage, "Vietnamese") && !strings.EqualFold(s.cfg.AITranslateLanguage, "vi") {
		log.Printf("RSS translation worker disabled: AI_TRANSLATE_LANGUAGE=%q is not supported; only Vietnamese is currently supported", s.cfg.AITranslateLanguage)
		return
	}
	if !s.aiConfigured() {
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
	var title, description, summary string
	err := s.db.QueryRowContext(ctx, `SELECT tj.article_id,a.title,a.description,a.summary FROM translation_jobs tj JOIN articles a ON a.id=tj.article_id WHERE tj.language_code='vi' ORDER BY tj.created_at DESC,tj.article_id DESC LIMIT 1`).Scan(&articleID, &title, &description, &summary)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	log.Printf("RSS translation worker: translating article id=%d", articleID)
	translated, err := s.aiClient().Vietnamese(ctx, translationservice.Fields{Title: title, Description: description, Summary: summary})
	if err != nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE translation_jobs SET created_at=CURRENT_TIMESTAMP WHERE article_id=? AND language_code='vi'`, articleID)
		return true, err
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO article_translations(article_id,language_code,title,description,summary) VALUES(?,'vi',?,?,?) ON CONFLICT(article_id,language_code) DO UPDATE SET title=excluded.title,description=excluded.description,summary=excluded.summary`, articleID, translated.Title, translated.Description, translated.Summary); err != nil {
		return true, err
	}
	if _, err = s.db.ExecContext(ctx, `DELETE FROM translation_jobs WHERE article_id=? AND language_code='vi'`, articleID); err != nil {
		return true, err
	}
	return true, nil
}

func (s *server) processTranslationQueue(ctx context.Context) {
	for ctx.Err() == nil {
		processed, err := s.processTranslationJob(ctx)
		if err != nil {
			log.Printf("manual translation queue: %v", err)
			return
		}
		if !processed {
			return
		}
	}
}
