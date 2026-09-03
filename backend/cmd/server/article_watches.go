package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
)

type articleWatch struct {
	ID           int64  `json:"id"`
	Kind         string `json:"kind"`
	ArticleID    *int64 `json:"article_id,omitempty"`
	CategoryID   *int64 `json:"category_id,omitempty"`
	Title        string `json:"title"`
	CategorySlug string `json:"category_slug,omitempty"`
	CategoryName string `json:"category_name,omitempty"`
	Keyword      string `json:"keyword,omitempty"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
}

func (s *server) listWatches(w http.ResponseWriter, r *http.Request) {
	items, err := s.watchesForUser(r.Context(), currentUser(r).ID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load follows")
		return
	}
	jsonOut(w, http.StatusOK, items)
}

func (s *server) watchesForUser(ctx context.Context, userID int64) ([]articleWatch, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT aw.id,aw.kind,aw.article_id,aw.category_id,COALESCE(a.title,''),COALESCE(c.slug,''),COALESCE(c.name,''),aw.keyword,aw.enabled,aw.created_at
		FROM article_watches aw LEFT JOIN articles a ON a.id=aw.article_id LEFT JOIN categories c ON c.id=aw.category_id
		WHERE aw.user_id=? ORDER BY aw.created_at DESC,aw.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []articleWatch{}
	for rows.Next() {
		var item articleWatch
		var articleID, categoryID sql.NullInt64
		var enabled int
		if err = rows.Scan(&item.ID, &item.Kind, &articleID, &categoryID, &item.Title, &item.CategorySlug, &item.CategoryName, &item.Keyword, &enabled, &item.CreatedAt); err != nil {
			return nil, err
		}
		if articleID.Valid {
			value := articleID.Int64
			item.ArticleID = &value
		}
		if categoryID.Valid {
			value := categoryID.Int64
			item.CategoryID = &value
		}
		item.Enabled = enabled == 1
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *server) followArticle(w http.ResponseWriter, r *http.Request) {
	articleID, err := positivePathID(r, "id")
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid article id")
		return
	}
	var title string
	if err = s.db.QueryRowContext(r.Context(), `SELECT title FROM articles WHERE id=?`, articleID).Scan(&title); errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "article not found")
		return
	} else if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not follow article")
		return
	}
	userID := currentUser(r).ID
	_, err = s.db.ExecContext(r.Context(), `INSERT INTO article_watches(user_id,kind,article_id,keyword,enabled,updated_at) VALUES(?,?,?,?,1,?) ON CONFLICT(user_id,article_id) WHERE article_id IS NOT NULL DO UPDATE SET enabled=1,keyword=excluded.keyword,updated_at=excluded.updated_at`, userID, "article", articleID, title, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not follow article")
		return
	}
	s.writeArticleWatch(w, r, userID, "article_id", articleID)
}

func (s *server) followTopic(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimSpace(chi.URLParam(r, "slug"))
	var categoryID int64
	if err := s.db.QueryRowContext(r.Context(), `SELECT id FROM categories WHERE slug=?`, slug).Scan(&categoryID); errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "topic not found")
		return
	} else if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not follow topic")
		return
	}
	userID := currentUser(r).ID
	_, err := s.db.ExecContext(r.Context(), `INSERT INTO article_watches(user_id,kind,category_id,enabled,updated_at) VALUES(?,?,?,1,?) ON CONFLICT(user_id,category_id) WHERE category_id IS NOT NULL DO UPDATE SET enabled=1,updated_at=excluded.updated_at`, userID, "topic", categoryID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not follow topic")
		return
	}
	s.writeArticleWatch(w, r, userID, "category_id", categoryID)
}

func (s *server) writeArticleWatch(w http.ResponseWriter, r *http.Request, userID int64, column string, value int64) {
	items, err := s.watchesForUser(r.Context(), userID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load follow")
		return
	}
	for _, item := range items {
		if (column == "article_id" && item.ArticleID != nil && *item.ArticleID == value) || (column == "category_id" && item.CategoryID != nil && *item.CategoryID == value) {
			jsonOut(w, http.StatusOK, item)
			return
		}
	}
	jsonErr(w, http.StatusInternalServerError, "could not load follow")
}

func (s *server) updateWatch(w http.ResponseWriter, r *http.Request) {
	id, err := positivePathID(r, "id")
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid follow id")
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if err = json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input); err != nil || input.Enabled == nil {
		jsonErr(w, http.StatusBadRequest, "enabled is required")
		return
	}
	result, err := s.db.ExecContext(r.Context(), `UPDATE article_watches SET enabled=?,updated_at=? WHERE id=? AND user_id=?`, *input.Enabled, time.Now().UTC().Format(time.RFC3339), id, currentUser(r).ID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not update follow")
		return
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		jsonErr(w, http.StatusNotFound, "follow not found")
		return
	}
	items, err := s.watchesForUser(r.Context(), currentUser(r).ID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load follow")
		return
	}
	for _, item := range items {
		if item.ID == id {
			jsonOut(w, http.StatusOK, item)
			return
		}
	}
	jsonErr(w, http.StatusInternalServerError, "could not load follow")
}

func (s *server) deleteWatch(w http.ResponseWriter, r *http.Request) {
	id, err := positivePathID(r, "id")
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid follow id")
		return
	}
	result, err := s.db.ExecContext(r.Context(), `DELETE FROM article_watches WHERE id=? AND user_id=?`, id, currentUser(r).ID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not remove follow")
		return
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		jsonErr(w, http.StatusNotFound, "follow not found")
		return
	}
	jsonOut(w, http.StatusOK, map[string]bool{"deleted": true})
}

func positivePathID(r *http.Request, name string) (int64, error) {
	value, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || value < 1 {
		return 0, errors.New("invalid id")
	}
	return value, nil
}

type watchNotification struct {
	WatchID, ChatID, ArticleID             int64
	Kind, Keyword, Title, Source, Category string
}

func (s *server) notifyArticleWatches(ctx context.Context, articleID int64) {
	if s.cfg.BotToken == "" || s.cfg.MiniAppURL == "" {
		return
	}
	rows, err := s.db.QueryContext(ctx, `SELECT aw.id,u.telegram_id,a.id,aw.kind,aw.keyword,a.title,s.name,c.name
		FROM article_watches aw JOIN users u ON u.id=aw.user_id JOIN articles a ON a.id=? JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id
		WHERE aw.enabled=1 AND NOT EXISTS(SELECT 1 FROM article_watch_alerts x WHERE x.watch_id=aw.id AND x.article_id=a.id AND x.status='sent')`, articleID)
	if err != nil {
		log.Printf("watch notifications query: %v", err)
		return
	}
	items := []watchNotification{}
	for rows.Next() {
		var item watchNotification
		if err = rows.Scan(&item.WatchID, &item.ChatID, &item.ArticleID, &item.Kind, &item.Keyword, &item.Title, &item.Source, &item.Category); err != nil {
			rows.Close()
			log.Printf("watch notifications scan: %v", err)
			return
		}
		items = append(items, item)
	}
	if err = rows.Close(); err != nil {
		log.Printf("watch notifications close: %v", err)
		return
	}
	categoryIDs, err := s.articleCategoryIDs(ctx, articleID)
	if err != nil {
		log.Printf("watch notification categories: %v", err)
		return
	}
	client := &http.Client{Timeout: 20 * time.Second}
	for _, item := range items {
		matched := item.Kind == "article" && relatedTitleMatch(item.Keyword, item.Title)
		if item.Kind == "topic" {
			var categoryID int64
			err = s.db.QueryRowContext(ctx, `SELECT category_id FROM article_watches WHERE id=?`, item.WatchID).Scan(&categoryID)
			matched = err == nil && categoryIDs[categoryID]
		}
		if !matched {
			continue
		}
		message := fmt.Sprintf("Diễn biến mới bạn đang theo dõi\n\n%s\n%s · %s", truncateWatchTitle(item.Title), item.Source, item.Category)
		payload := map[string]any{"chat_id": item.ChatID, "text": message, "disable_web_page_preview": true, "reply_markup": map[string]any{"inline_keyboard": [][]any{{map[string]any{"text": "Mở bài viết", "web_app": map[string]string{"url": strings.TrimRight(s.cfg.MiniAppURL, "/") + "/news/" + strconv.FormatInt(item.ArticleID, 10)}}}}}}
		status, detail := "sent", ""
		if err = s.sendTelegramMessage(ctx, client, payload); err != nil {
			status, detail = "failed", err.Error()
			log.Printf("watch notification user chat=%d watch=%d: %v", item.ChatID, item.WatchID, err)
		}
		if len(detail) > 1000 {
			detail = detail[:1000]
		}
		if _, err = s.db.ExecContext(ctx, `INSERT INTO article_watch_alerts(watch_id,article_id,status,error,sent_at) VALUES(?,?,?,?,?) ON CONFLICT(watch_id,article_id) DO UPDATE SET status=excluded.status,error=excluded.error,sent_at=excluded.sent_at`, item.WatchID, item.ArticleID, status, detail, time.Now().UTC().Format(time.RFC3339)); err != nil {
			log.Printf("watch notification record: %v", err)
		}
	}
}

func (s *server) articleCategoryIDs(ctx context.Context, articleID int64) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT category_id FROM article_categories WHERE article_id=? UNION SELECT category_id FROM articles WHERE id=?`, articleID, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

func relatedTitleMatch(watchedTitle, incomingTitle string) bool {
	words := meaningfulWatchWords(watchedTitle)
	if len(words) == 0 {
		return false
	}
	incoming := strings.ToLower(incomingTitle)
	matched := 0
	for word := range words {
		if strings.Contains(incoming, word) {
			matched++
		}
	}
	required := 2
	if len(words) == 1 {
		required = 1
	}
	return matched >= required
}
func meaningfulWatchWords(value string) map[string]bool {
	stop := map[string]bool{"about": true, "after": true, "again": true, "and": true, "are": true, "for": true, "from": true, "have": true, "into": true, "news": true, "the": true, "this": true, "that": true, "with": true, "với": true, "của": true, "cho": true, "trong": true, "những": true, "được": true, "một": true, "các": true, "về": true, "khi": true}
	words := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len([]rune(word)) >= 4 && !stop[word] {
			words[word] = true
		}
	}
	return words
}
func truncateWatchTitle(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > 240 {
		return string(runes[:237]) + "..."
	}
	return string(runes)
}
