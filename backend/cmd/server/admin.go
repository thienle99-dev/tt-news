package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
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
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		Enabled       bool   `json:"enabled"`
		LastFetchAt   string `json:"last_fetch_at"`
		LastSuccessAt string `json:"last_success_at"`
		LastError     string `json:"last_error"`
		LastInserted  int    `json:"last_inserted"`
	}
	rows, err := s.db.QueryContext(r.Context(), `SELECT s.id,s.name,s.enabled,COALESCE(h.last_fetch_at,''),COALESCE(h.last_success_at,''),COALESCE(h.last_error,''),COALESCE(h.last_inserted,0) FROM sources s LEFT JOIN source_health h ON h.source_id=s.id ORDER BY s.name`)
	if err != nil {
		jsonErr(w, 500, "could not load source status")
		return
	}
	defer rows.Close()
	sources := []sourceStatus{}
	for rows.Next() {
		var item sourceStatus
		var enabled int
		if err = rows.Scan(&item.ID, &item.Name, &enabled, &item.LastFetchAt, &item.LastSuccessAt, &item.LastError, &item.LastInserted); err != nil {
			jsonErr(w, 500, "could not read source status")
			return
		}
		item.Enabled = enabled == 1
		sources = append(sources, item)
	}
	if err = rows.Err(); err != nil {
		jsonErr(w, 500, "could not read source status")
		return
	}
	if err = rows.Close(); err != nil {
		jsonErr(w, 500, "could not close source status")
		return
	}
	counts := map[string]int64{}
	for key, query := range map[string]string{
		"translation_queue":      "SELECT count(*) FROM translation_jobs",
		"translations_generated": "SELECT count(*) FROM article_translations",
		"featured_briefs":        "SELECT count(*) FROM featured_briefs",
		"ai_feedback":            "SELECT count(*) FROM article_ai_feedback",
	} {
		var count int64
		_ = s.db.QueryRowContext(r.Context(), query).Scan(&count)
		counts[key] = count
	}
	type jobStatus struct {
		ID             int64  `json:"id"`
		Kind           string `json:"kind"`
		Title          string `json:"title"`
		Status         string `json:"status"`
		Detail         string `json:"detail"`
		TargetCount    int    `json:"target_count"`
		CompletedCount int    `json:"completed_count"`
		FailedCount    int    `json:"failed_count"`
		Trigger        string `json:"trigger"`
		Stage          string `json:"stage"`
		StartedAt      string `json:"started_at"`
		FinishedAt     string `json:"finished_at"`
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("jobs_page"))
	if page < 1 {
		page = 1
	}
	const jobsPageSize = 10
	var jobsTotal int
	if err = s.db.QueryRowContext(r.Context(), `SELECT count(*) FROM job_runs`).Scan(&jobsTotal); err != nil {
		jsonErr(w, 500, "could not count job status")
		return
	}
	jobs := []jobStatus{}
	jobRows, err := s.db.QueryContext(r.Context(), `SELECT id,kind,title,status,detail,target_count,completed_count,failed_count,trigger,stage,started_at,finished_at FROM job_runs ORDER BY id DESC LIMIT ? OFFSET ?`, jobsPageSize, (page-1)*jobsPageSize)
	if err != nil {
		jsonErr(w, 500, "could not load job status")
		return
	}
	for jobRows.Next() {
		var job jobStatus
		if err = jobRows.Scan(&job.ID, &job.Kind, &job.Title, &job.Status, &job.Detail, &job.TargetCount, &job.CompletedCount, &job.FailedCount, &job.Trigger, &job.Stage, &job.StartedAt, &job.FinishedAt); err != nil {
			jobRows.Close()
			jsonErr(w, 500, "could not read job status")
			return
		}
		jobs = append(jobs, job)
	}
	if err = jobRows.Err(); err != nil {
		jobRows.Close()
		jsonErr(w, 500, "could not read job status")
		return
	}
	jobRows.Close()
	if counts["translation_queue"] > 0 {
		jobs = append(jobs, jobStatus{Kind: "translation", Title: "Dịch bài tiếng Việt", Status: "queued", Detail: fmt.Sprintf("%d bài đang chờ xử lý", counts["translation_queue"]), TargetCount: int(counts["translation_queue"])})
	}
	usage := []map[string]any{}
	if usageRows, usageErr := s.db.QueryContext(r.Context(), `SELECT day,SUM(prompt_tokens),SUM(completion_tokens),SUM(total_tokens),SUM(requests) FROM ai_usage_daily WHERE day>=? GROUP BY day ORDER BY day`, time.Now().UTC().AddDate(0, 0, -13).Format("2006-01-02")); usageErr == nil {
		for usageRows.Next() {
			var day string
			var prompt, completion, total, requests int64
			if usageRows.Scan(&day, &prompt, &completion, &total, &requests) == nil {
				usage = append(usage, map[string]any{"day": day, "prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": total, "requests": requests})
			}
		}
		usageRows.Close()
	}
	jsonOut(w, 200, map[string]any{
		"sources":           sources,
		"translation_queue": counts["translation_queue"],
		"jobs":              jobs,
		"jobs_page":         page,
		"jobs_page_size":    jobsPageSize,
		"jobs_total":        jobsTotal,
		"ai_usage":          usage,
		"ai":                map[string]any{"model": aiSettings.Model, "configured": s.aiConfigured(), "config_source": aiSource, "translations_generated": counts["translations_generated"], "featured_briefs": counts["featured_briefs"], "feedback": counts["ai_feedback"], "cost_tracking": "provider token/cost usage is not exposed by the configured Chat Completions client"},
	})
}

func (s *server) adminFetchRSS(w http.ResponseWriter, r *http.Request) {
	sourceIDs, err := adminSourceIDs(r)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	go s.fetchSourcesSelected(context.Background(), "", sourceIDs, time.Time{})
	jsonOut(w, http.StatusAccepted, map[string]any{"status": "rss fetch started", "source_count": len(sourceIDs)})
}

func (s *server) adminCancelJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonErr(w, http.StatusBadRequest, "invalid job id")
		return
	}
	if err = s.cancelJob(r.Context(), id); err != nil {
		jsonErr(w, http.StatusConflict, err.Error())
		return
	}
	jsonOut(w, http.StatusOK, map[string]string{"status": "job cancellation requested"})
}

func adminSourceIDs(r *http.Request) ([]int64, error) {
	var body struct {
		SourceIDs []int64 `json:"source_ids"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		return nil, errors.New("invalid JSON body")
	}
	seen := map[int64]bool{}
	ids := make([]int64, 0, len(body.SourceIDs))
	for _, id := range body.SourceIDs {
		if id < 1 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
		if len(ids) > 500 {
			return nil, errors.New("too many sources selected")
		}
	}
	return ids, nil
}

func (s *server) adminEnqueueTranslations(w http.ResponseWriter, r *http.Request) {
	if !s.aiConfigured() {
		jsonErr(w, http.StatusBadRequest, "AI configuration is required")
		return
	}
	sourceIDs, err := adminSourceIDs(r)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(sourceIDs) == 0 {
		jsonErr(w, http.StatusBadRequest, "select at least one source")
		return
	}
	queued, err := s.enqueueSourceTranslations(r.Context(), sourceIDs)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not enqueue translations")
		return
	}
	go s.processTranslationQueue(context.Background())
	jsonOut(w, http.StatusAccepted, map[string]any{"status": "translations queued", "queued": queued, "source_count": len(sourceIDs)})
}

func (s *server) enqueueSourceTranslations(ctx context.Context, sourceIDs []int64) (int64, error) {
	placeholders := make([]string, 0, len(sourceIDs))
	args := make([]any, 0, len(sourceIDs))
	for _, id := range sourceIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	result, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO translation_jobs(article_id,language_code)
		SELECT a.id,'vi' FROM articles a
		JOIN sources s ON s.id=a.source_id
		LEFT JOIN article_translations tr ON tr.article_id=a.id AND tr.language_code='vi'
		WHERE s.enabled=1 AND tr.article_id IS NULL AND a.source_id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *server) adminRegenerateFeatured(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ArticleCount int `json:"article_count"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1024)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		jsonErr(w, http.StatusBadRequest, "article_count must be a number")
		return
	}
	articleTarget := defaultFeaturedArticleTarget
	if body.ArticleCount != 0 {
		if body.ArticleCount < minFeaturedArticleTarget || body.ArticleCount > featuredCandidateLimit {
			jsonErr(w, http.StatusBadRequest, "article_count must be between 3 and 24")
			return
		}
		articleTarget = body.ArticleCount
	}
	var activeJobID int64
	err := s.db.QueryRowContext(r.Context(), `SELECT id FROM job_runs WHERE kind='daily_brief' AND status='running' ORDER BY id DESC LIMIT 1`).Scan(&activeJobID)
	if err == nil {
		jsonErr(w, http.StatusConflict, fmt.Sprintf("daily briefing job %d is already running", activeJobID))
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusInternalServerError, "could not check daily briefing job")
		return
	}
	slot := time.Now().UTC().Truncate(s.cfg.FeaturedBriefInterval).Format(time.RFC3339)
	if _, err := s.db.ExecContext(r.Context(), "DELETE FROM featured_briefs WHERE slot_start=?", slot); err != nil {
		jsonErr(w, 500, "could not reset featured brief")
		return
	}
	go func() {
		if err := s.generateFeaturedBriefWithTarget(context.Background(), articleTarget); err != nil {
			log.Printf("manual featured briefing: %v", err)
		}
	}()
	jsonOut(w, http.StatusAccepted, map[string]any{"status": "featured brief generation started", "article_count": articleTarget})
}

func (s *server) adminUpdateSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		jsonErr(w, 400, "enabled is required")
		return
	}
	result, err := s.db.ExecContext(r.Context(), "UPDATE sources SET enabled=? WHERE id=? AND category_id<>(SELECT id FROM categories WHERE slug='business')", *body.Enabled, chi.URLParam(r, "id"))
	if err != nil {
		jsonErr(w, 500, "could not update source")
		return
	}
	count, _ := result.RowsAffected()
	if count == 0 {
		jsonErr(w, 404, "source not found")
		return
	}
	jsonOut(w, 200, map[string]bool{"enabled": *body.Enabled})
}
