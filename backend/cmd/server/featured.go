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

// Keep enough source material for a daily briefing of roughly 20 articles
// without making the AI request unnecessarily large.
const featuredCandidateLimit = 24
const featuredPerSourceLimit = 4
const defaultFeaturedArticleTarget = 8
const minFeaturedArticleTarget = 3

func (s *server) runFeaturedWorker(ctx context.Context) {
	if !s.aiConfigured() {
		log.Print("featured briefing worker disabled: set AI_URL and AI_KEY to enable it")
		return
	}
	s.generateFeaturedBrief(ctx)
	ticker := time.NewTicker(s.cfg.FeaturedBriefInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.generateFeaturedBrief(ctx)
		}
	}
}

func (s *server) generateFeaturedBrief(ctx context.Context) error {
	return s.generateFeaturedBriefWithTarget(ctx, defaultFeaturedArticleTarget)
}

func (s *server) generateFeaturedBriefWithTarget(ctx context.Context, articleTarget int) (err error) {
	if !s.aiConfigured() {
		return errors.New("AI is not configured")
	}
	articleTarget = min(max(articleTarget, minFeaturedArticleTarget), featuredCandidateLimit)
	job, jobCtx, jobErr := s.startJob(ctx, "daily_brief", "Tạo bản tin hôm nay", "scheduled", articleTarget)
	if jobErr != nil {
		return jobErr
	}
	ctx = jobCtx
	jobStatus, jobDetail := "completed", "Bản tin đã được tạo"
	defer func() {
		if err != nil {
			jobStatus, jobDetail = "failed", err.Error()
		}
		s.finishJob(context.Background(), job, jobStatus, jobDetail, 0, 0)
	}()
	s.featuredMu.Lock()
	defer s.featuredMu.Unlock()
	now := time.Now().UTC()
	slot := now.Truncate(s.cfg.FeaturedBriefInterval)
	var exists int
	if err := s.db.QueryRowContext(ctx, "SELECT 1 FROM featured_briefs WHERE slot_start=?", slot.Format(time.RFC3339)).Scan(&exists); err == nil {
		jobStatus, jobDetail = "skipped", "Bản tin cho khung thời gian hiện tại đã tồn tại"
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("featured briefing lookup: %w", err)
	}
	windowStart := now.Add(-s.cfg.FeaturedBriefWindow)
	candidates, err := s.featuredCandidates(ctx, windowStart)
	if err != nil {
		return fmt.Errorf("featured briefing candidates: %w", err)
	}
	if len(candidates) < 3 {
		jobStatus, jobDetail = "skipped", fmt.Sprintf("Chỉ có %d bài phù hợp", len(candidates))
		return nil
	}
	client := s.aiClient()
	brief, err := client.Featured(ctx, candidates[:min(articleTarget, len(candidates))], articleTarget)
	if err != nil {
		return fmt.Errorf("featured briefing generation: %w", err)
	}
	if err = validateFeaturedBrief(brief, candidates, articleTarget); err != nil {
		articleIDs := 0
		for _, topic := range brief.Topics {
			articleIDs += len(topic.ArticleIDs)
		}
		return fmt.Errorf("featured briefing validation (candidates=%d target=%d topics=%d article_ids=%d): %w", len(candidates), min(articleTarget, len(candidates)), len(brief.Topics), articleIDs, err)
	}
	vietnamese, err := client.FeaturedVietnamese(ctx, brief)
	if err != nil {
		return fmt.Errorf("featured briefing Vietnamese translation: %w", err)
	}
	if err = validateFeaturedTranslation(vietnamese, brief); err != nil {
		return fmt.Errorf("featured briefing Vietnamese validation: %w", err)
	}
	if err = s.storeFeaturedBrief(ctx, slot, windowStart, now, brief, vietnamese); err != nil {
		return fmt.Errorf("featured briefing save: %w", err)
	}
	log.Printf("featured briefing generated: topics=%d articles=%d candidates=%d", len(brief.Topics), min(articleTarget, len(candidates)), len(candidates))
	return nil
}

func (s *server) featuredCandidates(ctx context.Context, since time.Time) ([]translationservice.FeaturedCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.title,COALESCE(NULLIF(a.summary,''),a.description),s.name,c.slug,a.published_at FROM articles a JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id WHERE s.enabled=1 AND (a.summary<>'' OR a.description<>'') AND a.published_at>=? ORDER BY a.published_at DESC,a.id DESC`, since.Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	bySource := map[string]int{}
	out := make([]translationservice.FeaturedCandidate, 0, featuredCandidateLimit)
	for rows.Next() {
		var item translationservice.FeaturedCandidate
		if err = rows.Scan(&item.ID, &item.Title, &item.Summary, &item.Source, &item.Category, &item.PublishedAt); err != nil {
			return nil, err
		}
		if bySource[item.Source] >= featuredPerSourceLimit {
			continue
		}
		bySource[item.Source]++
		out = append(out, item)
		if len(out) == featuredCandidateLimit {
			break
		}
	}
	return out, rows.Err()
}

func validateFeaturedBrief(brief translationservice.FeaturedBrief, candidates []translationservice.FeaturedCandidate, articleTarget int) error {
	targetArticles := min(articleTarget, len(candidates))
	minimumTopics := max(3, (targetArticles+2)/3)
	maximumTopics := min(12, targetArticles)
	if brief.Title == "" || brief.Intro == "" || len(brief.Takeaways) != 3 || len(brief.Topics) < minimumTopics || len(brief.Topics) > maximumTopics {
		return fmt.Errorf("brief must contain a title, intro, 3 takeaways, and %d to %d topics (received title=%t intro=%t takeaways=%d topics=%d)", minimumTopics, maximumTopics, brief.Title != "", brief.Intro != "", len(brief.Takeaways), len(brief.Topics))
	}
	for _, takeaway := range brief.Takeaways {
		if strings.TrimSpace(takeaway) == "" {
			return errors.New("brief contains an empty takeaway")
		}
	}
	allowed, used := map[int64]bool{}, map[int64]bool{}
	for _, item := range candidates {
		allowed[item.ID] = true
	}
	for _, topic := range brief.Topics {
		if strings.TrimSpace(topic.Title) == "" || strings.TrimSpace(topic.Summary) == "" || strings.TrimSpace(topic.WhyItMatters) == "" || len(topic.ArticleIDs) < 1 || len(topic.ArticleIDs) > 3 {
			return errors.New("invalid featured topic")
		}
		for _, id := range topic.ArticleIDs {
			if !allowed[id] || used[id] {
				return fmt.Errorf("invalid or repeated article %d", id)
			}
			used[id] = true
		}
	}
	if len(used) != targetArticles {
		return fmt.Errorf("brief must use exactly %d distinct articles", targetArticles)
	}
	return nil
}

func validateFeaturedTranslation(translated, original translationservice.FeaturedBrief) error {
	if translated.Title == "" || translated.Intro == "" || len(translated.Takeaways) != len(original.Takeaways) || len(translated.Topics) != len(original.Topics) {
		return errors.New("translated briefing shape does not match")
	}
	for _, takeaway := range translated.Takeaways {
		if strings.TrimSpace(takeaway) == "" {
			return errors.New("translated briefing contains an empty takeaway")
		}
	}
	for index, topic := range translated.Topics {
		if topic.Title == "" || topic.Summary == "" || topic.WhyItMatters == "" || len(topic.ArticleIDs) != len(original.Topics[index].ArticleIDs) {
			return errors.New("translated topic shape does not match")
		}
		for idIndex, id := range topic.ArticleIDs {
			if id != original.Topics[index].ArticleIDs[idIndex] {
				return errors.New("translated article IDs changed")
			}
		}
	}
	return nil
}

func (s *server) storeFeaturedBrief(ctx context.Context, slot, start, end time.Time, brief, vietnamese translationservice.FeaturedBrief) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO featured_briefs(slot_start,generated_at,window_start,window_end,title,intro) VALUES(?,?,?,?,?,?)`, slot.Format(time.RFC3339), end.Format(time.RFC3339), start.Format(time.RFC3339), end.Format(time.RFC3339), brief.Title, brief.Intro)
	if err != nil {
		return err
	}
	briefID, err := result.LastInsertId()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO featured_brief_translations(brief_id,language_code,title,intro) VALUES(?,'vi',?,?)`, briefID, vietnamese.Title, vietnamese.Intro); err != nil {
		return err
	}
	for position, takeaway := range brief.Takeaways {
		if _, err = tx.ExecContext(ctx, `INSERT INTO featured_brief_takeaways(brief_id,position,text) VALUES(?,?,?)`, briefID, position, takeaway); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO featured_brief_takeaway_translations(brief_id,language_code,position,text) VALUES(?,'vi',?,?)`, briefID, position, vietnamese.Takeaways[position]); err != nil {
			return err
		}
	}
	for position, topic := range brief.Topics {
		result, err = tx.ExecContext(ctx, `INSERT INTO featured_topics(brief_id,position,title,summary,why_it_matters) VALUES(?,?,?,?,?)`, briefID, position, topic.Title, topic.Summary, topic.WhyItMatters)
		if err != nil {
			return err
		}
		topicID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		vi := vietnamese.Topics[position]
		if _, err = tx.ExecContext(ctx, `INSERT INTO featured_topic_translations(topic_id,language_code,title,summary,why_it_matters) VALUES(?,'vi',?,?,?)`, topicID, vi.Title, vi.Summary, vi.WhyItMatters); err != nil {
			return err
		}
		for articlePosition, articleID := range topic.ArticleIDs {
			if _, err = tx.ExecContext(ctx, `INSERT INTO featured_topic_articles(topic_id,article_id,position) VALUES(?,?,?)`, topicID, articleID, articlePosition); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *server) featured(w http.ResponseWriter, r *http.Request) {
	language := r.URL.Query().Get("lang")
	if language != "vi" {
		language = ""
	}
	var brief featuredBrief
	var title, intro string
	if language == "vi" {
		err := s.db.QueryRowContext(r.Context(), `SELECT b.id,b.generated_at,b.window_start,b.window_end,COALESCE(t.title,b.title),COALESCE(t.intro,b.intro) FROM featured_briefs b LEFT JOIN featured_brief_translations t ON t.brief_id=b.id AND t.language_code='vi' ORDER BY b.slot_start DESC LIMIT 1`).Scan(&brief.ID, &brief.GeneratedAt, &brief.WindowStart, &brief.WindowEnd, &title, &intro)
		if errors.Is(err, sql.ErrNoRows) {
			jsonOut(w, 200, nil)
			return
		}
		if err != nil {
			jsonErr(w, 500, "could not load featured briefing")
			return
		}
	} else {
		err := s.db.QueryRowContext(r.Context(), `SELECT id,generated_at,window_start,window_end,title,intro FROM featured_briefs ORDER BY slot_start DESC LIMIT 1`).Scan(&brief.ID, &brief.GeneratedAt, &brief.WindowStart, &brief.WindowEnd, &title, &intro)
		if errors.Is(err, sql.ErrNoRows) {
			jsonOut(w, 200, nil)
			return
		}
		if err != nil {
			jsonErr(w, 500, "could not load featured briefing")
			return
		}
	}
	brief.Title, brief.Intro = title, intro
	takeaways, err := s.featuredTakeaways(r.Context(), brief.ID, language)
	if err != nil {
		jsonErr(w, 500, "could not load featured takeaways")
		return
	}
	brief.Takeaways = takeaways
	rows, err := s.db.QueryContext(r.Context(), `SELECT ft.id,ft.position,COALESCE(tt.title,ft.title),COALESCE(tt.summary,ft.summary),COALESCE(NULLIF(tt.why_it_matters,''),ft.why_it_matters) FROM featured_topics ft LEFT JOIN featured_topic_translations tt ON tt.topic_id=ft.id AND tt.language_code=? WHERE ft.brief_id=? ORDER BY ft.position`, language, brief.ID)
	if err != nil {
		jsonErr(w, 500, "could not load featured topics")
		return
	}
	defer rows.Close()
	topics := []featuredTopic{}
	for rows.Next() {
		var topic featuredTopic
		if err = rows.Scan(&topic.ID, &topic.Position, &topic.Title, &topic.Summary, &topic.WhyItMatters); err != nil {
			jsonErr(w, 500, "could not read featured topics")
			return
		}
		topics = append(topics, topic)
	}
	if err = rows.Err(); err != nil {
		jsonErr(w, 500, "could not read featured topics")
		return
	}
	// SQLite is intentionally configured with one connection. Release the
	// topic rows before loading their articles, otherwise the nested queries
	// wait for the connection held by rows.
	if err = rows.Close(); err != nil {
		jsonErr(w, 500, "could not close featured topics")
		return
	}
	for index := range topics {
		topics[index].Articles, err = s.featuredTopicArticles(r.Context(), topics[index].ID, language, r)
		if err != nil {
			jsonErr(w, 500, "could not load featured articles")
			return
		}
	}
	brief.Topics = topics
	jsonOut(w, 200, brief)
}

func (s *server) featuredTakeaways(ctx context.Context, briefID int64, language string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(NULLIF(t.text,''),b.text) FROM featured_brief_takeaways b LEFT JOIN featured_brief_takeaway_translations t ON t.brief_id=b.brief_id AND t.position=b.position AND t.language_code=? WHERE b.brief_id=? ORDER BY b.position`, language, briefID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []string{}
	for rows.Next() {
		var item string
		if err = rows.Scan(&item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *server) featuredTopicArticles(ctx context.Context, topicID int64, language string, r *http.Request) ([]article, error) {
	translationJoin, title, summary := "", "a.title", "a.summary"
	args := []any{}
	if language == "vi" {
		translationJoin = " LEFT JOIN article_translations tr ON tr.article_id=a.id AND tr.language_code='vi'"
		title, summary = "COALESCE(NULLIF(tr.title,''),a.title)", "COALESCE(NULLIF(tr.summary,''),a.summary)"
	}
	saved := "0"
	read := "0"
	if user, ok := s.optionalUser(r); ok {
		saved = "EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.article_id=a.id AND sa.user_id=?)"
		args = append(args, user.ID)
		read = "EXISTS(SELECT 1 FROM article_reading_history arh WHERE arh.article_id=a.id AND arh.user_id=? AND arh.status='read')"
		args = append(args, user.ID)
	}
	args = append(args, topicID)
	query := fmt.Sprintf(`SELECT a.id,%s,a.description,%s,a.url,a.image_url,s.name,s.id,s.country_code,s.country_name,c.slug,a.published_at,%s,%s FROM featured_topic_articles fta JOIN articles a ON a.id=fta.article_id JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id%s WHERE fta.topic_id=? AND s.enabled=1 ORDER BY fta.position`, title, summary, saved, read, translationJoin)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []article{}
	for rows.Next() {
		var item article
		var isSaved, isRead int
		if err = rows.Scan(&item.ID, &item.Title, &item.Description, &item.Summary, &item.URL, &item.ImageURL, &item.Source, &item.SourceID, &item.CountryCode, &item.CountryName, &item.Category, &item.PublishedAt, &isSaved, &isRead); err != nil {
			return nil, err
		}
		item.IsSaved = isSaved == 1
		item.IsRead = isRead == 1
		out = append(out, item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = s.attachArticleCategoriesList(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}
