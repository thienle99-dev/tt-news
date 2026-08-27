package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// requireAdmin deliberately uses a separate secret from Telegram user auth.
// Operations that change sources or trigger workers must never be available to
// a regular Mini App user.
func (s *server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-Admin-Token")
		if s.cfg.AdminToken == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(s.cfg.AdminToken)) != 1 {
			jsonErr(w, http.StatusUnauthorized, "admin token is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) recordSourceHealth(ctx context.Context, sourceID int64, inserted int, fetchErr error) {
	now := time.Now().UTC().Format(time.RFC3339)
	lastSuccess, lastError := "", ""
	if fetchErr == nil {
		lastSuccess = now
	} else {
		lastError = fetchErr.Error()
		if len(lastError) > 1000 {
			lastError = lastError[:1000]
		}
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO source_health(source_id,last_fetch_at,last_success_at,last_error,last_inserted) VALUES(?,?,?,?,?)
		ON CONFLICT(source_id) DO UPDATE SET last_fetch_at=excluded.last_fetch_at,last_success_at=CASE WHEN excluded.last_success_at<>'' THEN excluded.last_success_at ELSE source_health.last_success_at END,last_error=excluded.last_error,last_inserted=excluded.last_inserted`, sourceID, now, lastSuccess, lastError, inserted)
}

func (s *server) adminStatus(w http.ResponseWriter, r *http.Request) {
	aiSettings, aiSource := s.currentAISettings()
	type sourceStatus struct {
		ID int64 `json:"id"`
		Name string `json:"name"`
		Enabled bool `json:"enabled"`
		LastFetchAt string `json:"last_fetch_at"`
		LastSuccessAt string `json:"last_success_at"`
		LastError string `json:"last_error"`
		LastInserted int `json:"last_inserted"`
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT s.id,s.name,s.enabled,COALESCE(h.last_fetch_at,''),COALESCE(h.last_success_at,''),COALESCE(h.last_error,''),COALESCE(h.last_inserted,0) FROM sources s LEFT JOIN source_health h ON h.source_id=s.id ORDER BY s.name`)
	if err != nil { jsonErr(w, 500, "could not load source status"); return }
	defer rows.Close()
	sources := []sourceStatus{}
	for rows.Next() {
		var item sourceStatus
		var enabled int
		if err = rows.Scan(&item.ID, &item.Name, &enabled, &item.LastFetchAt, &item.LastSuccessAt, &item.LastError, &item.LastInserted); err != nil { jsonErr(w, 500, "could not read source status"); return }
		item.Enabled = enabled == 1
		sources = append(sources, item)
	}
	counts := map[string]int64{}
	for key, query := range map[string]string{
		"translation_queue": "SELECT count(*) FROM translation_jobs",
		"translations_generated": "SELECT count(*) FROM article_translations",
		"featured_briefs": "SELECT count(*) FROM featured_briefs",
		"ai_feedback": "SELECT count(*) FROM article_ai_feedback",
	} {
		var count int64
		_ = s.db.QueryRowContext(r.Context(), query).Scan(&count)
		counts[key] = count
	}
	jsonOut(w, 200, map[string]any{
		"sources": sources,
		"translation_queue": counts["translation_queue"],
		"ai": map[string]any{"model": aiSettings.Model, "configured": s.aiConfigured(), "config_source": aiSource, "translations_generated": counts["translations_generated"], "featured_briefs": counts["featured_briefs"], "feedback": counts["ai_feedback"], "cost_tracking": "provider token/cost usage is not exposed by the configured Chat Completions client"},
	})
}

func (s *server) adminFetchRSS(w http.ResponseWriter, r *http.Request) {
	go s.fetchSources(context.Background(), "", time.Time{})
	jsonOut(w, http.StatusAccepted, map[string]string{"status": "rss fetch started"})
}

func (s *server) adminRegenerateFeatured(w http.ResponseWriter, r *http.Request) {
	slot := time.Now().UTC().Truncate(s.cfg.FeaturedBriefInterval).Format(time.RFC3339)
	if _, err := s.db.ExecContext(r.Context(), "DELETE FROM featured_briefs WHERE slot_start=?", slot); err != nil { jsonErr(w, 500, "could not reset featured brief"); return }
	go s.generateFeaturedBrief(context.Background())
	jsonOut(w, http.StatusAccepted, map[string]string{"status": "featured brief generation started"})
}

func (s *server) adminUpdateSource(w http.ResponseWriter, r *http.Request) {
	var body struct { Enabled *bool `json:"enabled"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil { jsonErr(w, 400, "enabled is required"); return }
	result, err := s.db.ExecContext(r.Context(), "UPDATE sources SET enabled=? WHERE id=? AND category_id<>(SELECT id FROM categories WHERE slug='business')", *body.Enabled, chi.URLParam(r, "id"))
	if err != nil { jsonErr(w, 500, "could not update source"); return }
	count, _ := result.RowsAffected()
	if count == 0 { jsonErr(w, 404, "source not found"); return }
	jsonOut(w, 200, map[string]bool{"enabled": *body.Enabled})
}
