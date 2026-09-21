package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const dailyDigestLimit = 10

type dailyDigestPreferences struct {
	Enabled     bool    `json:"enabled"`
	CategoryIDs []int64 `json:"category_ids"`
	SourceIDs   []int64 `json:"source_ids"`
}

func (s *server) dailyDigestPreferences(w http.ResponseWriter, r *http.Request) {
	pref, err := s.loadDailyDigestPreferences(r.Context(), currentUser(r).ID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load daily digest preferences")
		return
	}
	jsonOut(w, http.StatusOK, pref)
}

func (s *server) updateDailyDigestPreferences(w http.ResponseWriter, r *http.Request) {
	var input dailyDigestPreferences
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&input); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid daily digest preferences")
		return
	}
	input.CategoryIDs = uniquePositiveIDs(input.CategoryIDs)
	input.SourceIDs = uniquePositiveIDs(input.SourceIDs)
	if input.Enabled && len(input.CategoryIDs) == 0 && len(input.SourceIDs) == 0 {
		jsonErr(w, http.StatusBadRequest, "select at least one topic or source before enabling the daily digest")
		return
	}
	if err := s.validateDigestIDs(r.Context(), "categories", input.CategoryIDs); err != nil {
		jsonErr(w, http.StatusBadRequest, "one or more selected topics no longer exist")
		return
	}
	if err := s.validateDigestIDs(r.Context(), "sources", input.SourceIDs); err != nil {
		jsonErr(w, http.StatusBadRequest, "one or more selected sources no longer exist")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not save daily digest preferences")
		return
	}
	defer tx.Rollback()
	userID := currentUser(r).ID
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO user_daily_digests(user_id,enabled,updated_at) VALUES(?,?,?) ON CONFLICT(user_id) DO UPDATE SET enabled=excluded.enabled,updated_at=excluded.updated_at`, userID, input.Enabled, time.Now().UTC().Format(time.RFC3339)); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not save daily digest preferences")
		return
	}
	for _, table := range []string{"user_daily_digest_categories", "user_daily_digest_sources"} {
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM `+table+` WHERE user_id=?`, userID); err != nil {
			jsonErr(w, http.StatusInternalServerError, "could not save daily digest preferences")
			return
		}
	}
	for _, item := range input.CategoryIDs {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO user_daily_digest_categories(user_id,category_id) VALUES(?,?)`, userID, item); err != nil {
			jsonErr(w, http.StatusInternalServerError, "could not save daily digest preferences")
			return
		}
	}
	for _, item := range input.SourceIDs {
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO user_daily_digest_sources(user_id,source_id) VALUES(?,?)`, userID, item); err != nil {
			jsonErr(w, http.StatusInternalServerError, "could not save daily digest preferences")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not save daily digest preferences")
		return
	}
	jsonOut(w, http.StatusOK, input)
}

func (s *server) loadDailyDigestPreferences(ctx context.Context, userID int64) (dailyDigestPreferences, error) {
	pref := dailyDigestPreferences{CategoryIDs: []int64{}, SourceIDs: []int64{}}
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT enabled FROM user_daily_digests WHERE user_id=?`, userID).Scan(&enabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return pref, err
	}
	pref.Enabled = enabled == 1
	for _, item := range []struct {
		query string
		into  *[]int64
	}{
		{`SELECT category_id FROM user_daily_digest_categories WHERE user_id=? ORDER BY category_id`, &pref.CategoryIDs},
		{`SELECT source_id FROM user_daily_digest_sources WHERE user_id=? ORDER BY source_id`, &pref.SourceIDs},
	} {
		rows, err := s.db.QueryContext(ctx, item.query, userID)
		if err != nil {
			return pref, err
		}
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return pref, err
			}
			*item.into = append(*item.into, id)
		}
		if err = rows.Close(); err != nil {
			return pref, err
		}
	}
	return pref, nil
}

func (s *server) validateDigestIDs(ctx context.Context, table string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	if table != "categories" && table != "sources" {
		return errors.New("invalid digest table")
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for index, id := range ids {
		args[index] = id
	}
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE id IN (`+placeholders+`)`, args...).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return errors.New("invalid digest selection")
	}
	return nil
}

func (s *server) runDailyDigestWorker(ctx context.Context) {
	location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		log.Printf("daily digest disabled: %v", err)
		return
	}
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		s.runDailyDigestIfDue(ctx, time.Now().In(location), location)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *server) runDailyDigestIfDue(ctx context.Context, now time.Time, location *time.Location) {
	if now.Hour() != 8 {
		return
	}
	day := now.Format("2006-01-02")
	rows, err := s.db.QueryContext(ctx, `SELECT u.id,u.telegram_id FROM users u JOIN user_daily_digests d ON d.user_id=u.id WHERE d.enabled=1 AND NOT EXISTS (SELECT 1 FROM daily_digest_deliveries x WHERE x.user_id=u.id AND x.day=? AND x.status='sent')`, day)
	if err != nil {
		log.Printf("daily digest recipients: %v", err)
		return
	}
	recipients := make([]struct{ userID, chatID int64 }, 0)
	for rows.Next() {
		var recipient struct{ userID, chatID int64 }
		if err = rows.Scan(&recipient.userID, &recipient.chatID); err != nil {
			rows.Close()
			log.Printf("daily digest recipient: %v", err)
			return
		}
		recipients = append(recipients, recipient)
	}
	if err = rows.Close(); err != nil {
		log.Printf("daily digest recipients: %v", err)
		return
	}
	client := &http.Client{Timeout: 20 * time.Second}
	for _, recipient := range recipients {
		articles, loadErr := s.dailyDigestArticles(ctx, recipient.userID, now.Add(-24*time.Hour).UTC())
		if loadErr != nil {
			s.recordDailyDigestDelivery(ctx, recipient.userID, day, "failed", 0, loadErr.Error())
			continue
		}
		if len(articles) == 0 {
			continue
		}
		if sendErr := s.sendDailyDigest(ctx, client, recipient.chatID, articles); sendErr != nil {
			s.recordDailyDigestDelivery(ctx, recipient.userID, day, "failed", 0, sendErr.Error())
			log.Printf("daily digest user=%d: %v", recipient.userID, sendErr)
			continue
		}
		s.recordDailyDigestDelivery(ctx, recipient.userID, day, "sent", len(articles), "")
	}
}

type dailyDigestArticle struct {
	ID                         int64
	Title, Source, PublishedAt string
}

func (s *server) dailyDigestArticles(ctx context.Context, userID int64, since time.Time) ([]dailyDigestArticle, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.title,s.name,a.published_at FROM articles a JOIN sources s ON s.id=a.source_id
		WHERE a.published_at>=? AND s.enabled=1 AND a.is_hidden=0 AND (
		EXISTS(SELECT 1 FROM user_daily_digest_sources ds WHERE ds.user_id=? AND ds.source_id=a.source_id)
		OR EXISTS(SELECT 1 FROM user_daily_digest_categories dc WHERE dc.user_id=? AND (dc.category_id=a.category_id OR EXISTS(SELECT 1 FROM article_categories ac WHERE ac.article_id=a.id AND ac.category_id=dc.category_id)))
		) ORDER BY a.published_at DESC,a.id DESC LIMIT ?`, since.Format(time.RFC3339), userID, userID, dailyDigestLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	articles := []dailyDigestArticle{}
	for rows.Next() {
		var item dailyDigestArticle
		if err = rows.Scan(&item.ID, &item.Title, &item.Source, &item.PublishedAt); err != nil {
			return nil, err
		}
		articles = append(articles, item)
	}
	return articles, rows.Err()
}

func (s *server) sendDailyDigest(ctx context.Context, client *http.Client, chatID int64, articles []dailyDigestArticle) error {
	var message strings.Builder
	message.WriteString("Bản tin dành cho bạn · 24 giờ qua\n\n")
	for index, article := range articles {
		title := strings.TrimSpace(article.Title)
		if len([]rune(title)) > 170 {
			title = string([]rune(title)[:167]) + "..."
		}
		fmt.Fprintf(&message, "%d. %s\n%s\n\n", index+1, title, article.Source)
	}
	payload := map[string]any{"chat_id": chatID, "text": strings.TrimSpace(message.String()), "reply_markup": map[string]any{"inline_keyboard": [][]any{{map[string]any{"text": "Mở News", "web_app": map[string]string{"url": s.cfg.MiniAppURL}}}}}}
	return s.sendTelegramMessage(ctx, client, payload)
}

func (s *server) recordDailyDigestDelivery(ctx context.Context, userID int64, day, status string, articleCount int, detail string) {
	if len(detail) > 1000 {
		detail = detail[:1000]
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO daily_digest_deliveries(user_id,day,status,article_count,error,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(user_id,day) DO UPDATE SET status=excluded.status,article_count=excluded.article_count,error=excluded.error,updated_at=excluded.updated_at`, userID, day, status, articleCount, detail, time.Now().UTC().Format(time.RFC3339))
}
