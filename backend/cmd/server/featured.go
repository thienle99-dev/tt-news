package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	translationservice "telegram-news/internal/translation"
)

const featuredCandidateLimit = 80
const featuredPerSourceLimit = 4

func (s *server) runFeaturedWorker(ctx context.Context) {
	if s.cfg.AIURL == "" || s.cfg.AIKey == "" {
		log.Print("featured briefing worker disabled: set AI_URL and AI_KEY to enable it")
		return
	}
	s.generateFeaturedBrief(ctx)
	ticker := time.NewTicker(s.cfg.FeaturedBriefInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done(): return
		case <-ticker.C: s.generateFeaturedBrief(ctx)
		}
	}
}

func (s *server) generateFeaturedBrief(ctx context.Context) {
	if s.cfg.AIURL == "" || s.cfg.AIKey == "" { return }
	s.featuredMu.Lock()
	defer s.featuredMu.Unlock()
	now := time.Now().UTC()
	slot := now.Truncate(s.cfg.FeaturedBriefInterval)
	var exists int
	if err := s.db.QueryRowContext(ctx, "SELECT 1 FROM featured_briefs WHERE slot_start=?", slot.Format(time.RFC3339)).Scan(&exists); err == nil { return } else if !errors.Is(err, sql.ErrNoRows) { log.Printf("featured briefing lookup: %v", err); return }
	windowStart := now.Add(-s.cfg.FeaturedBriefWindow)
	candidates, err := s.featuredCandidates(ctx, windowStart)
	if err != nil { log.Printf("featured briefing candidates: %v", err); return }
	if len(candidates) < 5 { log.Printf("featured briefing skipped: only %d eligible articles", len(candidates)); return }
	client := translationservice.Client{URL: s.cfg.AIURL, APIKey: s.cfg.AIKey, Model: s.cfg.AIModel}
	brief, err := client.Featured(ctx, candidates)
	if err != nil { log.Printf("featured briefing generation: %v", err); return }
	if err = validateFeaturedBrief(brief, candidates); err != nil { log.Printf("featured briefing validation: %v", err); return }
	vietnamese, err := client.FeaturedVietnamese(ctx, brief)
	if err != nil { log.Printf("featured briefing Vietnamese translation: %v", err); return }
	if err = validateFeaturedTranslation(vietnamese, brief); err != nil { log.Printf("featured briefing Vietnamese validation: %v", err); return }
	if err = s.storeFeaturedBrief(ctx, slot, windowStart, now, brief, vietnamese); err != nil { log.Printf("featured briefing save: %v", err); return }
	log.Printf("featured briefing generated: topics=%d candidates=%d", len(brief.Topics), len(candidates))
}

func (s *server) featuredCandidates(ctx context.Context, since time.Time) ([]translationservice.FeaturedCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.title,a.summary,s.name,c.slug,a.published_at FROM articles a JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id WHERE s.enabled=1 AND c.slug<>'business' AND a.summary<>'' AND a.published_at>=? ORDER BY a.published_at DESC,a.id DESC`, since.Format(time.RFC3339))
	if err != nil { return nil, err }
	defer rows.Close()
	bySource := map[string]int{}
	out := make([]translationservice.FeaturedCandidate, 0, featuredCandidateLimit)
	for rows.Next() {
		var item translationservice.FeaturedCandidate
		if err = rows.Scan(&item.ID, &item.Title, &item.Summary, &item.Source, &item.Category, &item.PublishedAt); err != nil { return nil, err }
		if bySource[item.Source] >= featuredPerSourceLimit { continue }
		bySource[item.Source]++
		out = append(out, item)
		if len(out) == featuredCandidateLimit { break }
	}
	return out, rows.Err()
}

func validateFeaturedBrief(brief translationservice.FeaturedBrief, candidates []translationservice.FeaturedCandidate) error {
	if brief.Title == "" || brief.Intro == "" || len(brief.Topics) < 5 || len(brief.Topics) > 8 { return errors.New("brief must contain a title, intro, and 5 to 8 topics") }
	allowed, used := map[int64]bool{}, map[int64]bool{}
	for _, item := range candidates { allowed[item.ID] = true }
	for _, topic := range brief.Topics {
		if strings.TrimSpace(topic.Title) == "" || strings.TrimSpace(topic.Summary) == "" || len(topic.ArticleIDs) < 1 || len(topic.ArticleIDs) > 3 { return errors.New("invalid featured topic") }
		for _, id := range topic.ArticleIDs { if !allowed[id] || used[id] { return fmt.Errorf("invalid or repeated article %d", id) }; used[id] = true }
	}
	return nil
}

func validateFeaturedTranslation(translated, original translationservice.FeaturedBrief) error {
	if translated.Title == "" || translated.Intro == "" || len(translated.Topics) != len(original.Topics) { return errors.New("translated briefing shape does not match") }
	for index, topic := range translated.Topics {
		if topic.Title == "" || topic.Summary == "" || len(topic.ArticleIDs) != len(original.Topics[index].ArticleIDs) { return errors.New("translated topic shape does not match") }
		for idIndex, id := range topic.ArticleIDs { if id != original.Topics[index].ArticleIDs[idIndex] { return errors.New("translated article IDs changed") } }
	}
	return nil
}

func (s *server) storeFeaturedBrief(ctx context.Context, slot, start, end time.Time, brief, vietnamese translationservice.FeaturedBrief) error {
	tx, err := s.db.BeginTx(ctx, nil); if err != nil { return err }; defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO featured_briefs(slot_start,generated_at,window_start,window_end,title,intro) VALUES(?,?,?,?,?,?)`, slot.Format(time.RFC3339), end.Format(time.RFC3339), start.Format(time.RFC3339), end.Format(time.RFC3339), brief.Title, brief.Intro)
	if err != nil { return err }; briefID, err := result.LastInsertId(); if err != nil { return err }
	if _, err = tx.ExecContext(ctx, `INSERT INTO featured_brief_translations(brief_id,language_code,title,intro) VALUES(?,'vi',?,?)`, briefID, vietnamese.Title, vietnamese.Intro); err != nil { return err }
	for position, topic := range brief.Topics {
		result, err = tx.ExecContext(ctx, `INSERT INTO featured_topics(brief_id,position,title,summary) VALUES(?,?,?,?)`, briefID, position, topic.Title, topic.Summary); if err != nil { return err }; topicID, err := result.LastInsertId(); if err != nil { return err }
		vi := vietnamese.Topics[position]
		if _, err = tx.ExecContext(ctx, `INSERT INTO featured_topic_translations(topic_id,language_code,title,summary) VALUES(?,'vi',?,?)`, topicID, vi.Title, vi.Summary); err != nil { return err }
		for articlePosition, articleID := range topic.ArticleIDs { if _, err = tx.ExecContext(ctx, `INSERT INTO featured_topic_articles(topic_id,article_id,position) VALUES(?,?,?)`, topicID, articleID, articlePosition); err != nil { return err } }
	}
	return tx.Commit()
}

func (s *server) featured(w http.ResponseWriter, r *http.Request) {
	language := r.URL.Query().Get("lang"); if language != "vi" { language = "" }
	var brief featuredBrief
	var title, intro string
	if language == "vi" {
		err := s.db.QueryRowContext(r.Context(), `SELECT b.id,b.generated_at,b.window_start,b.window_end,COALESCE(t.title,b.title),COALESCE(t.intro,b.intro) FROM featured_briefs b LEFT JOIN featured_brief_translations t ON t.brief_id=b.id AND t.language_code='vi' ORDER BY b.slot_start DESC LIMIT 1`).Scan(&brief.ID, &brief.GeneratedAt, &brief.WindowStart, &brief.WindowEnd, &title, &intro)
		if errors.Is(err, sql.ErrNoRows) { jsonErr(w, 404, "featured briefing not found"); return }; if err != nil { jsonErr(w, 500, "could not load featured briefing"); return }
	} else {
		err := s.db.QueryRowContext(r.Context(), `SELECT id,generated_at,window_start,window_end,title,intro FROM featured_briefs ORDER BY slot_start DESC LIMIT 1`).Scan(&brief.ID, &brief.GeneratedAt, &brief.WindowStart, &brief.WindowEnd, &title, &intro)
		if errors.Is(err, sql.ErrNoRows) { jsonErr(w, 404, "featured briefing not found"); return }; if err != nil { jsonErr(w, 500, "could not load featured briefing"); return }
	}
	brief.Title, brief.Intro = title, intro
	rows, err := s.db.QueryContext(r.Context(), `SELECT ft.id,ft.position,COALESCE(tt.title,ft.title),COALESCE(tt.summary,ft.summary) FROM featured_topics ft LEFT JOIN featured_topic_translations tt ON tt.topic_id=ft.id AND tt.language_code=? WHERE ft.brief_id=? ORDER BY ft.position`, language, brief.ID)
	if err != nil { jsonErr(w, 500, "could not load featured topics"); return }; defer rows.Close()
	for rows.Next() { var topic featuredTopic; if err = rows.Scan(&topic.ID, &topic.Position, &topic.Title, &topic.Summary); err != nil { jsonErr(w, 500, "could not read featured topics"); return }; topic.Articles, err = s.featuredTopicArticles(r.Context(), topic.ID, language, r); if err != nil { jsonErr(w, 500, "could not load featured articles"); return }; brief.Topics = append(brief.Topics, topic) }
	jsonOut(w, 200, brief)
}

func (s *server) featuredTopicArticles(ctx context.Context, topicID int64, language string, r *http.Request) ([]article, error) {
	translationJoin, title, summary := "", "a.title", "a.summary"; args := []any{}
	if language == "vi" { translationJoin = " LEFT JOIN article_translations tr ON tr.article_id=a.id AND tr.language_code='vi'"; title, summary = "COALESCE(NULLIF(tr.title,''),a.title)", "COALESCE(NULLIF(tr.summary,''),a.summary)" }
	saved := "0"; if user, ok := s.optionalUser(r); ok { saved = "EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.article_id=a.id AND sa.user_id=?)"; args = append(args, user.ID) }
	args = append(args, topicID)
	query := fmt.Sprintf(`SELECT a.id,%s,a.description,%s,a.url,a.image_url,s.name,s.id,s.country_code,s.country_name,c.slug,a.published_at,%s FROM featured_topic_articles fta JOIN articles a ON a.id=fta.article_id JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id%s WHERE fta.topic_id=? AND s.enabled=1 AND c.slug<>'business' ORDER BY fta.position`, title, summary, saved, translationJoin)
	rows, err := s.db.QueryContext(ctx, query, args...); if err != nil { return nil, err }; defer rows.Close(); out := []article{}
	for rows.Next() { var item article; var isSaved int; if err = rows.Scan(&item.ID,&item.Title,&item.Description,&item.Summary,&item.URL,&item.ImageURL,&item.Source,&item.SourceID,&item.CountryCode,&item.CountryName,&item.Category,&item.PublishedAt,&isSaved); err != nil { return nil, err }; item.IsSaved = isSaved == 1; out = append(out, item) }
	return out, rows.Err()
}
