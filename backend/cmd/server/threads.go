package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-chi/chi/v5"
)

const maxThreadImageBytes int64 = 15 << 20

type threadsTarget struct {
	ID              int64  `json:"id"`
	Kind            string `json:"kind"`
	Query           string `json:"query"`
	Enabled         bool   `json:"enabled"`
	LastFetchAt     string `json:"last_fetch_at"`
	LastSuccessAt   string `json:"last_success_at"`
	LastError       string `json:"last_error"`
	LastInserted    int    `json:"last_inserted"`
	Origin          string `json:"origin"`
	AutoFollow      bool   `json:"auto_follow"`
	LastQualifiedAt string `json:"last_qualified_at"`
}

type threadPost struct {
	ID, URL, Author, Text, ImageURL string
	PublishedAt                     time.Time
	Likes, Replies, Reposts         int64
	IsReply, IsRepost               bool
}

type threadClassification struct {
	Relevant bool
	Category string
	Source   string
}

const threadsAutoFollowLimit = 200

var threadsPostURL = regexp.MustCompile(`https?://www\.threads\.(?:com|net)/@[^/"'\s]+/post/[A-Za-z0-9_-]+`)

func (s *server) runThreads(ctx context.Context) {
	s.fetchThreads(ctx, nil)
	t := time.NewTicker(s.cfg.ThreadsInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.fetchThreads(ctx, nil)
		}
	}
}

func (s *server) fetchThreads(ctx context.Context, only []int64) {
	query := `SELECT id,kind,query,enabled,last_fetch_at,last_success_at,last_error,last_inserted,origin,auto_follow,last_qualified_at FROM threads_targets WHERE enabled=1`
	args := []any{}
	if len(only) > 0 {
		query += " AND id IN (" + placeholders(len(only)) + ")"
		for _, id := range only {
			args = append(args, id)
		}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		log.Printf("threads targets: %v", err)
		return
	}
	defer rows.Close()
	targets := []threadsTarget{}
	for rows.Next() {
		var item threadsTarget
		var enabled int
		var autoFollow int
		if err = rows.Scan(&item.ID, &item.Kind, &item.Query, &enabled, &item.LastFetchAt, &item.LastSuccessAt, &item.LastError, &item.LastInserted, &item.Origin, &autoFollow, &item.LastQualifiedAt); err == nil {
			item.Enabled = enabled == 1
			item.AutoFollow = autoFollow == 1
			targets = append(targets, item)
		}
	}
	if len(targets) == 0 {
		log.Print("threads crawl skipped: no enabled targets")
		return
	}
	log.Printf("threads crawl started: targets=%d", len(targets))
	job, jobCtx, err := s.startJob(ctx, "threads_crawl", "Crawl Threads", "scheduled", len(targets))
	if err != nil {
		log.Printf("threads crawl skipped: %v", err)
		return
	}
	completed, failed, inserted := int64(0), int64(0), 0
	defer func() {
		s.finishJob(context.Background(), job, "completed", fmt.Sprintf("%d target · %d post mới · %d lỗi", completed, inserted, failed), completed, failed)
	}()
	for _, target := range targets {
		log.Printf("threads fetching %s %q", target.Kind, target.Query)
		count, crawlErr := s.fetchThreadsTarget(jobCtx, target)
		completed++
		inserted += count
		s.recordThreadsTargetHealth(jobCtx, target.ID, count, crawlErr)
		if crawlErr != nil {
			failed++
			log.Printf("threads %s %q: %v", target.Kind, target.Query, crawlErr)
		} else {
			log.Printf("threads %s %q done: new=%d", target.Kind, target.Query, count)
		}
		s.updateJob(jobCtx, job, "Đang crawl Threads", target.Query, completed, failed)
		if jobCtx.Err() != nil {
			return
		}
		time.Sleep(1200 * time.Millisecond)
	}
	log.Printf("threads crawl finished: targets=%d new=%d failed=%d", completed, inserted, failed)
}

func (s *server) fetchThreadsTarget(ctx context.Context, target threadsTarget) (int, error) {
	page := "https://www.threads.com/@" + strings.TrimPrefix(strings.TrimSpace(target.Query), "@")
	if target.Kind == "keyword" {
		page = "https://www.threads.com/search/?q=" + url.QueryEscape(target.Query) + "&serp_type=recent"
	}
	body, err := threadsGET(ctx, page)
	if err != nil {
		return 0, err
	}
	posts := threadsPostsFromSSR(body, strings.TrimPrefix(strings.TrimSpace(target.Query), "@"))
	if len(posts) > 0 {
		inserted := 0
		for _, post := range posts {
			if post.IsReply || post.IsRepost {
				log.Printf("threads post skipped: reply/repost author=@%s", post.Author)
				continue
			}
			classification := s.classifyThreadPost(ctx, target, post)
			if !classification.Relevant {
				log.Printf("threads post skipped: irrelevant author=@%s target=%q", post.Author, target.Query)
				continue
			}
			wasNew, saveErr := s.saveThreadPost(ctx, target, post, classification)
			if saveErr != nil {
				return inserted, saveErr
			}
			if wasNew {
				inserted++
			}
		}
		return inserted, nil
	}
	links := threadsPostURL.FindAllString(string(body), -1)
	seen := map[string]bool{}
	inserted := 0
	for _, link := range links {
		link = normalizeURL(link)
		if link == "" || seen[link] {
			continue
		}
		seen[link] = true
		post, err := threadsPostFromPage(ctx, link)
		if err != nil {
			continue
		}
		if post.Text == "" {
			continue
		}
		classification := s.classifyThreadPost(ctx, target, post)
		if !classification.Relevant {
			continue
		}
		wasNew, err := s.saveThreadPost(ctx, target, post, classification)
		if err != nil {
			return inserted, err
		}
		if wasNew {
			inserted++
		}
	}
	return inserted, nil
}

func threadsGET(ctx context.Context, raw string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	// Threads serves profile post data inside SSR JSON to crawler user agents.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Threads returned %s", res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, 4<<20))
}

// threadsPostsFromSSR walks the JSON script payload produced for crawler user
// agents. Posts are media objects with a non-empty caption and short code;
// walking maps makes this resilient to Relay wrapper changes.
func threadsPostsFromSSR(body []byte, fallbackAuthor string) []threadPost {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	posts := []threadPost{}
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case []any:
			for _, item := range node {
				walk(item)
			}
		case map[string]any:
			code, _ := node["code"].(string)
			caption, _ := node["caption"].(map[string]any)
			text, _ := caption["text"].(string)
			if code != "" && validThreadPostText(text) && !seen[code] {
				seen[code] = true
				author := fallbackAuthor
				if user, ok := node["user"].(map[string]any); ok {
					if username, ok := user["username"].(string); ok && username != "" {
						author = username
					}
				}
				published := time.Now().UTC()
				if seconds := threadsInt(node["taken_at"]); seconds > 0 {
					published = time.Unix(seconds, 0).UTC()
				}
				image := ""
				if versions, ok := node["image_versions2"].(map[string]any); ok {
					if candidates, ok := versions["candidates"].([]any); ok && len(candidates) > 0 {
						if first, ok := candidates[0].(map[string]any); ok {
							image, _ = first["url"].(string)
						}
					}
				}
				isReply, _ := node["is_reply"].(bool)
				isRepost, _ := node["is_repost"].(bool)
				posts = append(posts, threadPost{ID: fmt.Sprint(node["pk"]), URL: "https://www.threads.com/@" + author + "/post/" + code, Author: author, Text: strings.TrimSpace(text), ImageURL: image, PublishedAt: published, Likes: threadsInt(node["like_count"]), Replies: threadsInt(node["comment_count"]), Reposts: threadsInt(node["repost_count"]), IsReply: isReply, IsRepost: isRepost})
			}
			for _, item := range node {
				walk(item)
			}
		}
	}
	doc.Find("script[type='application/json']").Each(func(_ int, script *goquery.Selection) {
		var value any
		if json.Unmarshal([]byte(script.Text()), &value) == nil {
			walk(value)
		}
	})
	return posts
}

func threadsInt(value any) int64 {
	switch number := value.(type) {
	case float64:
		return int64(number)
	case int64:
		return number
	case json.Number:
		n, _ := number.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(number, 10, 64)
		return n
	}
	return 0
}

func threadsPostFromPage(ctx context.Context, link string) (threadPost, error) {
	body, err := threadsGET(ctx, link)
	if err != nil {
		return threadPost{}, err
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return threadPost{}, err
	}
	post := threadPost{URL: link, PublishedAt: time.Now().UTC()}
	parts := strings.Split(strings.TrimSuffix(link, "/"), "/")
	post.ID = parts[len(parts)-1]
	if len(parts) > 3 {
		post.Author = strings.TrimPrefix(parts[len(parts)-3], "@")
	}
	doc.Find("meta").Each(func(_ int, m *goquery.Selection) {
		key, _ := m.Attr("property")
		if key == "" {
			key, _ = m.Attr("name")
		}
		value, _ := m.Attr("content")
		switch key {
		case "og:description", "twitter:description":
			if post.Text == "" {
				post.Text = strings.TrimSpace(value)
			}
		case "og:image", "twitter:image":
			if post.ImageURL == "" {
				post.ImageURL = value
			}
		case "article:published_time":
			if parsed, e := time.Parse(time.RFC3339, value); e == nil {
				post.PublishedAt = parsed
			}
		}
	})
	post.Text = strings.TrimSpace(strings.TrimPrefix(post.Text, "Threads · "))
	if !validThreadPostText(post.Text) {
		post.Text = ""
	}
	return post, nil
}

// validThreadPostText rejects the JavaScript hydration payload that Threads
// may return to logged-out crawlers. Those strings are page bootstrapping data,
// not the caption of a public post.
func validThreadPostText(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	for _, marker := range []string{
		`{"require":`, `"bootstrapwebsession"`, `"cometssr`,
		`"qpltagserverjs"`, `"qpltimingsserverjs"`,
		`"replacenativetimer"`, `"maybedisableanimations"`,
	} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func (s *server) saveThreadPost(ctx context.Context, target threadsTarget, p threadPost, classification threadClassification) (bool, error) {
	if !validThreadPostText(p.Text) {
		return false, nil
	}
	var sourceID, categoryID int64
	if err := s.db.QueryRowContext(ctx, `SELECT s.id FROM sources s WHERE s.feed_url='https://www.threads.com'`).Scan(&sourceID); err != nil {
		return false, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM categories WHERE slug=?`, classification.Category).Scan(&categoryID); err != nil {
		return false, err
	}
	title := p.Text
	chars := []rune(title)
	if len(chars) > 140 {
		title = string(chars[:140]) + "…"
	}
	if title == "" {
		return false, nil
	}
	var articleID int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM articles WHERE url=?`, p.URL).Scan(&articleID)
	if errors.Is(err, sql.ErrNoRows) {
		fingerprint := articleFingerprint(p.Text, 20)
		if fingerprint != "" {
			err = s.db.QueryRowContext(ctx, `SELECT id FROM articles WHERE content_fingerprint=? OR title_fingerprint=? LIMIT 1`, fingerprint, fingerprint).Scan(&articleID)
		}
	}
	isNew := errors.Is(err, sql.ErrNoRows)
	if err != nil && !isNew {
		return false, err
	}
	if isNew {
		r, e := s.db.ExecContext(ctx, `INSERT INTO articles(source_id,category_id,title,description,full_content,summary,url,image_url,content_images,published_at,title_fingerprint,content_fingerprint,thread_post_id,thread_author,thread_likes,thread_replies,thread_reposts,thread_classification,thread_classification_source) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, sourceID, categoryID, title, p.Text, p.Text, "", p.URL, p.ImageURL, "[]", p.PublishedAt.UTC().Format(time.RFC3339), articleFingerprint(title, 20), articleFingerprint(p.Text, 20), p.ID, p.Author, p.Likes, p.Replies, p.Reposts, classification.Category, classification.Source)
		if e != nil {
			return false, e
		}
		articleID, _ = r.LastInsertId()
		_ = s.replaceArticleCategories(ctx, articleID, []categoryDefinition{{slug: classification.Category, name: classification.Category}})
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE articles SET image_url=CASE WHEN ?<>'' THEN ? ELSE image_url END,thread_author=CASE WHEN thread_post_id<>'' THEN ? ELSE thread_author END,thread_likes=CASE WHEN thread_post_id<>'' THEN ? ELSE thread_likes END,thread_replies=CASE WHEN thread_post_id<>'' THEN ? ELSE thread_replies END,thread_reposts=CASE WHEN thread_post_id<>'' THEN ? ELSE thread_reposts END WHERE id=?`, p.ImageURL, p.ImageURL, p.Author, p.Likes, p.Replies, p.Reposts, articleID)
		if err != nil {
			return false, err
		}
	}
	_, err = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO article_threads_targets(article_id,target_id) VALUES(?,?)`, articleID, target.ID)
	if err != nil {
		return isNew, err
	}
	if target.Kind == "keyword" {
		s.promoteThreadsAccount(ctx, p.Author)
	}
	if isNew {
		go s.notifyArticleWatches(context.Background(), articleID)
		if s.cfg.AITranslateEnabled && s.cfg.RSSTranslateVietnamese {
			_, _ = s.db.ExecContext(ctx, `INSERT OR IGNORE INTO translation_jobs(article_id,language_code) VALUES(?,'vi')`, articleID)
		}
	}
	return isNew, nil
}

func (s *server) classifyThreadPost(ctx context.Context, target threadsTarget, post threadPost) threadClassification {
	fallback := threadsRuleClassification(target, post.Text)
	if !fallback.Relevant || !s.aiConfigured() {
		return fallback
	}
	decision, err := s.aiClient().ClassifyThread(ctx, post.Text)
	if err != nil || !threadsCategoryAllowed(decision.Category) {
		if err != nil {
			log.Printf("threads AI fallback author=@%s: %v", post.Author, err)
		}
		return fallback
	}
	return threadClassification{Relevant: decision.Relevant, Category: decision.Category, Source: "ai"}
}

func threadsRuleClassification(target threadsTarget, text string) threadClassification {
	value := strings.ToLower(strings.TrimSpace(text))
	if len([]rune(value)) < 24 || strings.Contains(value, "giveaway") || strings.Contains(value, "mã giảm giá") {
		return threadClassification{}
	}
	category := "society"
	for slug, words := range map[string][]string{
		"ai":         {" ai ", "trí tuệ nhân tạo", "chatgpt", "gemini", "llm"},
		"technology": {"công nghệ", "startup", "phần mềm", "điện thoại"},
		"business":   {"chứng khoán", "cổ phiếu", "bất động sản", "doanh nghiệp", "kinh tế"},
		"science":    {"khoa học", "nghiên cứu"},
		"health":     {"y tế", "sức khỏe", "bệnh viện"},
		"world":      {"việt nam", "hà nội", "tphcm", "tp hcm", "tin nóng"},
	} {
		for _, word := range words {
			if strings.Contains(" "+value+" ", word) {
				category = slug
				break
			}
		}
	}
	if target.Kind == "keyword" {
		value += " " + strings.ToLower(target.Query)
	}
	return threadClassification{Relevant: true, Category: category, Source: "rule"}
}

func threadsCategoryAllowed(slug string) bool {
	for _, value := range []string{"technology", "programming", "world", "business", "society", "culture", "sports", "education", "health", "science", "ai", "security", "llm", "cybersecurity"} {
		if slug == value {
			return true
		}
	}
	return false
}

func (s *server) promoteThreadsAccount(ctx context.Context, author string) {
	author = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(author), "@"))
	if author == "" {
		return
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)
	var posts, keywords int
	err := s.db.QueryRowContext(ctx, `SELECT count(DISTINCT a.id),count(DISTINCT t.query) FROM articles a JOIN article_threads_targets att ON att.article_id=a.id JOIN threads_targets t ON t.id=att.target_id WHERE a.thread_author=? AND t.kind='keyword' AND a.published_at>=?`, author, cutoff).Scan(&posts, &keywords)
	if err != nil || posts < 3 || keywords < 2 {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = s.db.ExecContext(ctx, `UPDATE threads_targets SET last_qualified_at=? WHERE kind='profile' AND lower(query)=? AND auto_follow=1`, now, author)
	var exists int
	if s.db.QueryRowContext(ctx, `SELECT count(*) FROM threads_targets WHERE kind='profile' AND lower(query)=?`, author).Scan(&exists) != nil || exists > 0 {
		return
	}
	var active int
	_ = s.db.QueryRowContext(ctx, `SELECT count(*) FROM threads_targets WHERE kind='profile' AND auto_follow=1 AND enabled=1`).Scan(&active)
	if active >= threadsAutoFollowLimit {
		stale := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)
		_, _ = s.db.ExecContext(ctx, `UPDATE threads_targets SET enabled=0,last_error='auto-follow paused: inactive for 30 days' WHERE id IN (SELECT id FROM threads_targets WHERE kind='profile' AND auto_follow=1 AND enabled=1 AND (last_qualified_at='' OR last_qualified_at<?) ORDER BY last_qualified_at LIMIT 1)`, stale)
		_ = s.db.QueryRowContext(ctx, `SELECT count(*) FROM threads_targets WHERE kind='profile' AND auto_follow=1 AND enabled=1`).Scan(&active)
	}
	if active >= threadsAutoFollowLimit {
		log.Printf("threads auto-follow limit reached: @%s not added", author)
		return
	}
	if _, err = s.db.ExecContext(ctx, `INSERT INTO threads_targets(kind,query,enabled,origin,auto_follow,last_qualified_at) VALUES('profile',?,1,'discovery',1,?)`, author, now); err == nil {
		log.Printf("threads auto-follow added: @%s qualified_posts=%d keywords=%d", author, posts, keywords)
	}
}

func (s *server) recordThreadsTargetHealth(ctx context.Context, id int64, inserted int, fetchErr error) {
	now := time.Now().UTC().Format(time.RFC3339)
	success, errorText := now, ""
	if fetchErr != nil {
		success = ""
		errorText = fetchErr.Error()
		if len(errorText) > 1000 {
			errorText = errorText[:1000]
		}
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE threads_targets SET last_fetch_at=?,last_success_at=CASE WHEN ?<>'' THEN ? ELSE last_success_at END,last_error=?,last_inserted=? WHERE id=?`, now, success, success, errorText, inserted, id)
}

func (s *server) listThreads(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := intEnvFrom(q.Get("limit"), 30)
	if limit < 1 || limit > 100 {
		limit = 30
	}
	offset := intEnvFrom(q.Get("offset"), 0)
	where := []string{"a.thread_post_id<>''", "s.enabled=1"}
	args := []any{}
	if id, err := strconv.ParseInt(q.Get("target"), 10, 64); err == nil && id > 0 {
		where = append(where, "EXISTS(SELECT 1 FROM article_threads_targets att WHERE att.article_id=a.id AND att.target_id=?)")
		args = append(args, id)
	}
	if term := strings.TrimSpace(q.Get("q")); term != "" {
		where = append(where, "(a.title LIKE ? OR a.description LIKE ? OR a.thread_author LIKE ?)")
		like := "%" + term + "%"
		args = append(args, like, like, like)
	}
	if username := strings.TrimPrefix(strings.TrimSpace(q.Get("username")), "@"); username != "" {
		where = append(where, "lower(a.thread_author) LIKE lower(?)")
		args = append(args, "%"+username+"%")
	}
	order := "a.published_at DESC,a.id DESC"
	if q.Get("sort") == "engagement" {
		order = "(a.thread_likes+a.thread_replies+a.thread_reposts) DESC,a.published_at DESC"
	}
	args = append(args, limit, offset)
	rows, err := s.db.QueryContext(r.Context(), `SELECT a.id,a.title,a.description,a.summary,a.url,a.image_url,s.name,s.id,s.country_code,s.country_name,c.slug,a.published_at,a.thread_author,a.thread_likes,a.thread_replies,a.thread_reposts,a.thread_classification,a.thread_classification_source FROM articles a JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id WHERE `+strings.Join(where, " AND ")+` ORDER BY `+order+` LIMIT ? OFFSET ?`, args...)
	if err != nil {
		jsonErr(w, 500, "could not load Threads posts")
		return
	}
	defer rows.Close()
	out := []article{}
	for rows.Next() {
		var a article
		if err = rows.Scan(&a.ID, &a.Title, &a.Description, &a.Summary, &a.URL, &a.ImageURL, &a.Source, &a.SourceID, &a.CountryCode, &a.CountryName, &a.Category, &a.PublishedAt, &a.ThreadAuthor, &a.ThreadLikes, &a.ThreadReplies, &a.ThreadReposts, &a.ThreadClassification, &a.ThreadClassificationSource); err != nil {
			jsonErr(w, 500, "could not read Threads posts")
			return
		}
		a.Categories = []articleCategory{{Slug: a.Category, Name: a.Category}}
		a.ThreadPostID = "threads"
		s.wrapThreadImage(&a)
		out = append(out, a)
	}
	jsonOut(w, 200, out)
}

// threadsAuthors supplies the autocomplete list for the public Threads view.
// It only exposes authors that already have a retained, enabled Threads post.
func (s *server) threadsAuthors(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT DISTINCT a.thread_author FROM articles a JOIN sources s ON s.id=a.source_id WHERE a.thread_post_id<>'' AND a.thread_author<>'' AND s.enabled=1 ORDER BY lower(a.thread_author) LIMIT 500`)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load Threads authors")
		return
	}
	defer rows.Close()
	authors := []string{}
	for rows.Next() {
		var author string
		if err := rows.Scan(&author); err != nil {
			jsonErr(w, http.StatusInternalServerError, "could not read Threads authors")
			return
		}
		authors = append(authors, author)
	}
	if err := rows.Err(); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not read Threads authors")
		return
	}
	jsonOut(w, http.StatusOK, authors)
}

func (s *server) wrapThreadImage(article *article) {
	if article.ThreadPostID != "" && article.ImageURL != "" {
		article.ImageURL = fmt.Sprintf("/api/threads/posts/%d/image", article.ID)
	}
}

func (s *server) threadImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonErr(w, http.StatusNotFound, "image not found")
		return
	}
	var imageURL string
	err = s.db.QueryRowContext(r.Context(), `SELECT a.image_url FROM articles a JOIN sources s ON s.id=a.source_id WHERE a.id=? AND a.thread_post_id<>'' AND a.image_url<>'' AND s.enabled=1`, id).Scan(&imageURL)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "image not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load image")
		return
	}
	if !allowedThreadsImageURL(imageURL) {
		jsonErr(w, http.StatusBadGateway, "image source is unavailable")
		return
	}
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if len(via) >= 3 || !allowedThreadsImageURL(next.URL.String()) {
			return errors.New("unsupported image redirect")
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, imageURL, nil)
	if err != nil {
		jsonErr(w, http.StatusBadGateway, "image source is unavailable")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TelegramNewsImageProxy/1.0)")
	res, err := client.Do(req)
	if err != nil || res.StatusCode < 200 || res.StatusCode >= 300 {
		if res != nil {
			res.Body.Close()
		}
		jsonErr(w, http.StatusBadGateway, "image source is unavailable")
		return
	}
	defer res.Body.Close()
	contentType := strings.ToLower(strings.TrimSpace(res.Header.Get("Content-Type")))
	if !strings.HasPrefix(contentType, "image/") || (res.ContentLength > maxThreadImageBytes) {
		jsonErr(w, http.StatusBadGateway, "image source is unavailable")
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, io.LimitReader(res.Body, maxThreadImageBytes+1))
}

func allowedThreadsImageURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "fbcdn.net" || strings.HasSuffix(host, ".fbcdn.net")
}

func (s *server) adminThreadsTargets(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,kind,query,enabled,last_fetch_at,last_success_at,last_error,last_inserted,origin,auto_follow,last_qualified_at FROM threads_targets ORDER BY kind,auto_follow,query`)
	if err != nil {
		jsonErr(w, 500, "could not load Threads targets")
		return
	}
	defer rows.Close()
	out := []threadsTarget{}
	for rows.Next() {
		var v threadsTarget
		var enabled int
		var autoFollow int
		if rows.Scan(&v.ID, &v.Kind, &v.Query, &enabled, &v.LastFetchAt, &v.LastSuccessAt, &v.LastError, &v.LastInserted, &v.Origin, &autoFollow, &v.LastQualifiedAt) == nil {
			v.Enabled = enabled == 1
			v.AutoFollow = autoFollow == 1
			out = append(out, v)
		}
	}
	jsonOut(w, 200, out)
}

func (s *server) threadsTargets(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,kind,query,enabled,last_fetch_at,last_success_at,last_error,last_inserted,origin,auto_follow,last_qualified_at FROM threads_targets WHERE enabled=1 ORDER BY kind,auto_follow,query`)
	if err != nil {
		jsonErr(w, 500, "could not load Threads targets")
		return
	}
	defer rows.Close()
	out := []threadsTarget{}
	for rows.Next() {
		var v threadsTarget
		var enabled int
		var autoFollow int
		if rows.Scan(&v.ID, &v.Kind, &v.Query, &enabled, &v.LastFetchAt, &v.LastSuccessAt, &v.LastError, &v.LastInserted, &v.Origin, &autoFollow, &v.LastQualifiedAt) == nil {
			v.Enabled = enabled == 1
			v.AutoFollow = autoFollow == 1
			out = append(out, v)
		}
	}
	jsonOut(w, 200, out)
}
func (s *server) adminCreateThreadsTarget(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind, Query string
		Enabled     *bool `json:"enabled"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&in) != nil || (in.Kind != "profile" && in.Kind != "keyword") || strings.TrimSpace(in.Query) == "" {
		jsonErr(w, 400, "kind (profile|keyword) and query are required")
		return
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	result, err := s.db.ExecContext(r.Context(), `INSERT INTO threads_targets(kind,query,enabled) VALUES(?,?,?)`, in.Kind, strings.TrimSpace(in.Query), enabled)
	if err != nil {
		jsonErr(w, 409, "Threads target already exists")
		return
	}
	id, _ := result.LastInsertId()
	jsonOut(w, 201, map[string]any{"id": id, "enabled": enabled})
}
func (s *server) adminUpdateThreadsTarget(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Query   *string `json:"query"`
		Enabled *bool   `json:"enabled"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || (in.Query == nil && in.Enabled == nil) {
		jsonErr(w, 400, "query or enabled is required")
		return
	}
	sets := []string{}
	args := []any{}
	if in.Query != nil {
		if strings.TrimSpace(*in.Query) == "" {
			jsonErr(w, 400, "query cannot be empty")
			return
		}
		sets = append(sets, "query=?")
		args = append(args, strings.TrimSpace(*in.Query))
	}
	if in.Enabled != nil {
		sets = append(sets, "enabled=?")
		args = append(args, *in.Enabled)
	}
	args = append(args, chi.URLParam(r, "id"))
	result, err := s.db.ExecContext(r.Context(), `UPDATE threads_targets SET `+strings.Join(sets, ",")+` WHERE id=?`, args...)
	if err != nil {
		jsonErr(w, 409, "could not update Threads target")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		jsonErr(w, 404, "Threads target not found")
		return
	}
	jsonOut(w, 200, map[string]bool{"updated": true})
}
func (s *server) adminDeleteThreadsTarget(w http.ResponseWriter, r *http.Request) {
	result, err := s.db.ExecContext(r.Context(), `DELETE FROM threads_targets WHERE id=?`, chi.URLParam(r, "id"))
	if err != nil {
		jsonErr(w, 500, "could not delete Threads target")
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		jsonErr(w, 404, "Threads target not found")
		return
	}
	jsonOut(w, 200, map[string]bool{"deleted": true})
}

// adminDeleteThreadsTargetPosts removes the posts collected for one target but
// keeps the target itself ready for its next crawl. A post associated with
// another target is retained and only detached from this target.
func (s *server) adminDeleteThreadsTargetPosts(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || targetID < 1 {
		jsonErr(w, http.StatusBadRequest, "invalid Threads target")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not start Threads post deletion")
		return
	}
	defer tx.Rollback()
	var exists int
	if err = tx.QueryRowContext(r.Context(), `SELECT 1 FROM threads_targets WHERE id=?`, targetID).Scan(&exists); err == sql.ErrNoRows {
		jsonErr(w, http.StatusNotFound, "Threads target not found")
		return
	} else if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load Threads target")
		return
	}
	result, err := tx.ExecContext(r.Context(), `DELETE FROM articles WHERE thread_post_id<>'' AND id IN (SELECT article_id FROM article_threads_targets WHERE target_id=?) AND NOT EXISTS (SELECT 1 FROM article_threads_targets other WHERE other.article_id=articles.id AND other.target_id<>?)`, targetID, targetID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not delete Threads posts")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE FROM article_threads_targets WHERE target_id=?`, targetID); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not clear Threads target posts")
		return
	}
	if err = tx.Commit(); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not finish Threads post deletion")
		return
	}
	deleted, _ := result.RowsAffected()
	jsonOut(w, http.StatusOK, map[string]int64{"deleted": deleted})
}
func (s *server) adminFetchThreads(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetIDs []int64 `json:"target_ids"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
	go s.fetchThreads(context.Background(), body.TargetIDs)
	jsonOut(w, 202, map[string]string{"status": "Threads crawl started"})
}

// adminDiscoverThreads runs only keyword targets. New authors found through
// those searches are evaluated by promoteThreadsAccount and can become profile
// targets without needlessly recrawling the existing watch list.
func (s *server) adminDiscoverThreads(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id FROM threads_targets WHERE kind='keyword' AND enabled=1 ORDER BY id`)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load Threads discovery targets")
		return
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			jsonErr(w, http.StatusInternalServerError, "could not read Threads discovery targets")
			return
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not read Threads discovery targets")
		return
	}
	if len(ids) == 0 {
		jsonErr(w, http.StatusConflict, "no enabled keyword targets for Threads discovery")
		return
	}
	go s.fetchThreads(context.Background(), ids)
	jsonOut(w, http.StatusAccepted, map[string]any{"status": "Threads username discovery started", "target_count": len(ids)})
}
