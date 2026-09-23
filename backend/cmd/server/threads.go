package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
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
	ID, URL, Author, DisplayName, AvatarURL, Text, ImageURL string
	PublishedAt                                             time.Time
	Likes, Replies, Reposts                                 int64
	IsReply, IsRepost                                       bool
}

type threadClassification struct {
	Relevant bool
	Category string
	Source   string
}

type threadCandidate struct {
	target         threadsTarget
	post           threadPost
	classification threadClassification
	score          float64
}

type threadComment struct {
	ID          string `json:"id"`
	Author      string `json:"author"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	Likes       int64  `json:"likes"`
}

type threadsAuthorModeration struct {
	Username        string `json:"username"`
	Blocked         bool   `json:"blocked"`
	PostCount       int64  `json:"post_count"`
	ProfileTargetID int64  `json:"profile_target_id,omitempty"`
	ProfileEnabled  bool   `json:"profile_enabled"`
}

func normalizeThreadsAuthor(value string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(value), "@"))
}

func (s *server) isThreadsAuthorBlocked(ctx context.Context, author string) (bool, error) {
	author = normalizeThreadsAuthor(author)
	if author == "" {
		return false, nil
	}
	var exists int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM threads_author_blocks WHERE username=?`, author).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

const (
	threadsAutoFollowLimit     = 200
	threadsRelatedFetchLimit   = 16
	threadsCandidatesPerTarget = 24
	threadsDirectPostsLimit    = 8
	threadsPostsPerCrawl       = 60
	threadsAuthorLimit         = 1
	threadsCategoryLimit       = 2
	threadsCommentLimit        = 10
	threadsCommentCacheTTL     = 30 * time.Minute
)

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
	targets = interleaveThreadsTargets(targets)
	candidates := make([]threadCandidate, 0, len(targets)*threadsCandidatesPerTarget)
	results := make(map[int64]int, len(targets))
	seen := make(map[string]bool)
	for _, target := range targets {
		log.Printf("threads fetching %s %q", target.Kind, target.Query)
		posts, crawlErr := s.fetchThreadsTarget(jobCtx, target)
		completed++
		if crawlErr != nil {
			failed++
			log.Printf("threads %s %q: %v", target.Kind, target.Query, crawlErr)
		} else {
			for _, post := range posts {
				key := normalizeURL(post.URL)
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				candidate, ok, qualifyErr := s.qualifyThreadPost(jobCtx, target, post)
				if qualifyErr != nil {
					crawlErr = qualifyErr
					failed++
					log.Printf("threads %s %q qualification: %v", target.Kind, target.Query, qualifyErr)
					break
				}
				if ok {
					candidates = append(candidates, candidate)
				}
			}
			log.Printf("threads %s %q candidates=%d", target.Kind, target.Query, len(posts))
		}
		s.recordThreadsTargetHealth(jobCtx, target.ID, 0, crawlErr)
		s.updateJob(jobCtx, job, "Đang crawl Threads", target.Query, completed, failed)
		if jobCtx.Err() != nil {
			return
		}
		time.Sleep(1200 * time.Millisecond)
	}
	selected := selectDiverseThreadCandidates(candidates, threadsPostsPerCrawl)
	for _, candidate := range selected {
		wasNew, saveErr := s.saveThreadPost(jobCtx, candidate.target, candidate.post, candidate.classification)
		if saveErr != nil {
			failed++
			log.Printf("threads save @%s: %v", candidate.post.Author, saveErr)
			continue
		}
		if wasNew {
			inserted++
			results[candidate.target.ID]++
		}
	}
	for _, target := range targets {
		if results[target.ID] > 0 {
			s.recordThreadsTargetHealth(jobCtx, target.ID, results[target.ID], nil)
		}
	}
	log.Printf("threads crawl selection: candidates=%d selected=%d inserted=%d", len(candidates), len(selected), inserted)
	log.Printf("threads crawl finished: targets=%d new=%d failed=%d", completed, inserted, failed)
}

// interleaveThreadsTargets prevents a large profile watch list from starving
// keyword discovery, while retaining a deterministic ordering within each kind.
func interleaveThreadsTargets(targets []threadsTarget) []threadsTarget {
	profiles, keywords := []threadsTarget{}, []threadsTarget{}
	for _, target := range targets {
		if target.Kind == "keyword" {
			keywords = append(keywords, target)
		} else {
			profiles = append(profiles, target)
		}
	}
	sort.SliceStable(profiles, func(i, j int) bool { return threadsTargetBefore(profiles[i], profiles[j]) })
	sort.SliceStable(keywords, func(i, j int) bool { return threadsTargetBefore(keywords[i], keywords[j]) })
	ordered := make([]threadsTarget, 0, len(targets))
	for len(profiles) > 0 || len(keywords) > 0 {
		if len(keywords) > 0 {
			ordered = append(ordered, keywords[0])
			keywords = keywords[1:]
		}
		if len(profiles) > 0 {
			ordered = append(ordered, profiles[0])
			profiles = profiles[1:]
		}
	}
	return ordered
}

func threadsTargetBefore(left, right threadsTarget) bool {
	priority := func(target threadsTarget) int {
		switch {
		case target.LastSuccessAt == "":
			return 0 // New targets get their first useful signal quickly.
		case target.LastError != "":
			return 1 // Surface unhealthy targets early for visible health data.
		default:
			return 2
		}
	}
	leftPriority, rightPriority := priority(left), priority(right)
	if leftPriority != rightPriority {
		return leftPriority < rightPriority
	}
	if left.LastFetchAt != right.LastFetchAt {
		return left.LastFetchAt < right.LastFetchAt
	}
	return left.ID < right.ID
}

func (s *server) fetchThreadsTarget(ctx context.Context, target threadsTarget) ([]threadPost, error) {
	page := "https://www.threads.com/@" + strings.TrimPrefix(strings.TrimSpace(target.Query), "@")
	if target.Kind == "keyword" {
		page = "https://www.threads.com/search/?q=" + url.QueryEscape(target.Query) + "&serp_type=recent"
	}
	body, err := threadsGET(ctx, page)
	if err != nil {
		return nil, err
	}
	posts := threadsPostsFromSSR(body, strings.TrimPrefix(strings.TrimSpace(target.Query), "@"))
	if target.Kind == "profile" {
		author := normalizeThreadsAuthor(target.Query)
		posts = filterThreadsPosts(posts, func(post threadPost) bool { return normalizeThreadsAuthor(post.Author) == author })
	}
	posts = uniqueThreadsPosts(posts)
	initialLimit := threadsCandidatesPerTarget
	if target.Kind == "keyword" {
		// Search SSR responses are commonly clustered by author. Keep one post
		// per author first, then fill the remaining direct slots by page order.
		// The rest of the bounded pool is reserved for permalink discovery.
		posts = diverseThreadsSeedPosts(posts, threadsDirectPostsLimit)
		initialLimit = threadsDirectPostsLimit
	}
	if len(posts) > initialLimit {
		posts = posts[:initialLimit]
	}
	seen := map[string]bool{}
	for _, post := range posts {
		if post.URL != "" {
			seen[normalizeURL(post.URL)] = true
		}
	}

	// A Threads SSR page can include post permalinks from recommendations and
	// reply context that are not emitted as complete post objects. Follow a
	// small, deduplicated subset so a target is not limited to its first feed
	// slice, while keeping each scheduled crawl bounded.
	links := threadsPostURL.FindAllString(string(body), -1)
	if target.Kind != "keyword" {
		return posts, nil
	}
	relatedFetched := 0
	for _, link := range links {
		link = normalizeURL(link)
		if link == "" || seen[link] || relatedFetched >= threadsRelatedFetchLimit || len(posts) >= threadsCandidatesPerTarget {
			continue
		}
		seen[link] = true
		post, err := threadsPostFromPage(ctx, link)
		if err != nil {
			continue
		}
		relatedFetched++
		posts = append(posts, post)
	}
	return posts, nil
}

func uniqueThreadsPosts(posts []threadPost) []threadPost {
	seen := make(map[string]bool, len(posts))
	unique := make([]threadPost, 0, len(posts))
	for _, post := range posts {
		key := normalizeURL(post.URL)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, post)
	}
	return unique
}

// diverseThreadsSeedPosts preserves the page's recency ordering while making
// room for distinct authors before taking additional posts from anyone.
func diverseThreadsSeedPosts(posts []threadPost, limit int) []threadPost {
	if limit <= 0 || len(posts) == 0 {
		return nil
	}
	if limit >= len(posts) {
		return posts
	}
	selected := make([]threadPost, 0, limit)
	used := make([]bool, len(posts))
	authors := map[string]bool{}
	for i, post := range posts {
		author := normalizeThreadsAuthor(post.Author)
		if author == "" || authors[author] {
			continue
		}
		authors[author], used[i] = true, true
		selected = append(selected, post)
		if len(selected) == limit {
			return selected
		}
	}
	for i, post := range posts {
		if used[i] {
			continue
		}
		selected = append(selected, post)
		if len(selected) == limit {
			break
		}
	}
	return selected
}

func filterThreadsPosts(posts []threadPost, keep func(threadPost) bool) []threadPost {
	filtered := posts[:0]
	for _, post := range posts {
		if keep(post) {
			filtered = append(filtered, post)
		}
	}
	return filtered
}

func (s *server) qualifyThreadPost(ctx context.Context, target threadsTarget, post threadPost) (threadCandidate, bool, error) {
	if post.Text == "" || post.IsReply || post.IsRepost {
		return threadCandidate{}, false, nil
	}
	blocked, err := s.isThreadsAuthorBlocked(ctx, post.Author)
	if err != nil || blocked {
		return threadCandidate{}, false, err
	}
	classification := s.classifyThreadPost(ctx, target, post)
	if !classification.Relevant {
		return threadCandidate{}, false, nil
	}
	return threadCandidate{target: target, post: post, classification: classification, score: threadPostScore(target, post, classification)}, true, nil
}

func threadPostScore(target threadsTarget, post threadPost, classification threadClassification) float64 {
	// Quality signals intentionally stay explainable and work without an AI
	// provider. AI classification is a small confidence boost, not a gate.
	textLength := len([]rune(strings.TrimSpace(post.Text)))
	if textLength > 400 {
		textLength = 400
	}
	score := math.Min(float64(textLength)/20, 20)
	engagement := post.Likes + post.Replies*2 + post.Reposts*3
	if engagement < 0 {
		engagement = 0
	}
	score += math.Min(math.Log1p(float64(engagement))*5, 25)
	if classification.Source == "ai" {
		score += 5
	}
	if target.Kind == "keyword" {
		score += 3 // Preserve room for discovering new authors and topics.
	}
	age := time.Since(post.PublishedAt)
	switch {
	case age <= 6*time.Hour:
		score += 20
	case age <= 24*time.Hour:
		score += 12
	case age <= 7*24*time.Hour:
		score += 5
	}
	return score
}

// selectDiverseThreadCandidates ranks by quality, then enforces hard author
// and category limits. A second pass fills unused capacity only when the
// diverse pool cannot reach the crawl limit.
func selectDiverseThreadCandidates(candidates []threadCandidate, limit int) []threadCandidate {
	if limit <= 0 || len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].post.PublishedAt.After(candidates[j].post.PublishedAt)
		}
		return candidates[i].score > candidates[j].score
	})
	if limit > len(candidates) {
		limit = len(candidates)
	}
	selected := make([]threadCandidate, 0, limit)
	authorCount, categoryCount := map[string]int{}, map[string]int{}
	selectedByPrimaryPass := make([]bool, len(candidates))
	for i, candidate := range candidates {
		if len(selected) == limit {
			return selected
		}
		author := normalizeThreadsAuthor(candidate.post.Author)
		category := candidate.classification.Category
		if authorCount[author] >= threadsAuthorLimit || categoryCount[category] >= threadsCategoryLimit {
			continue
		}
		selectedByPrimaryPass[i] = true
		selected = append(selected, candidate)
		authorCount[author]++
		categoryCount[category]++
	}
	for i, candidate := range candidates {
		if len(selected) == limit {
			break
		}
		if !selectedByPrimaryPass[i] {
			selected = append(selected, candidate)
		}
	}
	return selected
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
			code := threadsPostCode(node)
			text := threadsPostText(node)
			if code != "" && validThreadPostText(text) && !seen[code] {
				seen[code] = true
				author, displayName, avatarURL := threadsPostAuthor(node, fallbackAuthor)
				published := threadsPostPublishedAt(node)
				image := threadsPostImage(node)
				isReply, _ := node["is_reply"].(bool)
				isRepost, _ := node["is_repost"].(bool)
				postID := threadsString(node["pk"])
				if postID == "" {
					postID = threadsString(node["id"])
				}
				posts = append(posts, threadPost{ID: postID, URL: "https://www.threads.com/@" + author + "/post/" + code, Author: author, DisplayName: displayName, AvatarURL: avatarURL, Text: strings.TrimSpace(text), ImageURL: image, PublishedAt: published, Likes: threadsInt(node["like_count"]), Replies: threadsInt(node["comment_count"]), Reposts: threadsInt(node["repost_count"]), IsReply: isReply, IsRepost: isRepost})
			}
			for _, item := range node {
				walk(item)
			}
		}
	}
	doc.Find("script").Each(func(_ int, script *goquery.Selection) {
		decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(script.Text())))
		for {
			var value any
			if err := decoder.Decode(&value); err != nil {
				break
			}
			walk(value)
		}
	})
	return posts
}

// threadsCommentsFromSSR keeps only top-level replies to the requested post.
// A post page may also contain recommendations and nested conversations, so
// accepting every object marked is_reply would leak unrelated discussion.
func threadsCommentsFromSSR(body []byte, rootPostID string) []threadComment {
	if strings.TrimSpace(rootPostID) == "" {
		return nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	comments := []threadComment{}
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case []any:
			for _, item := range node {
				walk(item)
			}
		case map[string]any:
			isReply, _ := node["is_reply"].(bool)
			id, text := threadsNodeID(node), threadsPostText(node)
			if isReply && id != "" && validThreadPostText(text) && !seen[id] && threadsReplyParentID(node) == rootPostID {
				seen[id] = true
				author, displayName, avatarURL := threadsPostAuthor(node, "")
				published := threadsPostPublishedAt(node)
				comments = append(comments, threadComment{ID: id, Author: author, DisplayName: displayName, AvatarURL: avatarURL, Body: strings.TrimSpace(text), PublishedAt: published.Format(time.RFC3339), Likes: threadsInt(node["like_count"])})
			}
			for _, item := range node {
				walk(item)
			}
		}
	}
	doc.Find("script").Each(func(_ int, script *goquery.Selection) {
		decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(script.Text())))
		for {
			var value any
			if err := decoder.Decode(&value); err != nil {
				break
			}
			walk(value)
		}
	})
	return comments
}

func threadsNodeID(node map[string]any) string {
	for _, key := range []string{"pk", "id"} {
		if id := threadsIdentifier(node[key]); id != "" {
			return id
		}
	}
	return ""
}

func threadsReplyParentID(node map[string]any) string {
	for _, key := range []string{"reply_to_id", "reply_to_post_id", "parent_post_id", "thread_id"} {
		if id := threadsIdentifier(node[key]); id != "" {
			return id
		}
		if parent, ok := node[key].(map[string]any); ok {
			if id := threadsNodeID(parent); id != "" {
				return id
			}
		}
	}
	return ""
}

func threadsIdentifier(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	}
	return ""
}

func threadsString(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func threadsPostCode(node map[string]any) string {
	if code := threadsString(node["code"]); code != "" {
		return code
	}
	for _, key := range []string{"permalink", "url"} {
		if link := threadsString(node[key]); link != "" {
			if match := threadsPostURL.FindString(link); match != "" {
				parts := strings.Split(strings.TrimSuffix(match, "/"), "/")
				return parts[len(parts)-1]
			}
		}
	}
	return ""
}

func threadsPostText(node map[string]any) string {
	if caption, ok := node["caption"].(map[string]any); ok {
		if text := threadsString(caption["text"]); text != "" {
			return text
		}
	}
	return threadsString(node["text"])
}

func threadsPostAuthor(node map[string]any, fallback string) (author, displayName, avatarURL string) {
	author = fallback
	user, _ := node["user"].(map[string]any)
	if user == nil {
		user, _ = node["owner"].(map[string]any)
	}
	if user == nil {
		user = node
	}
	if username := threadsString(user["username"]); username != "" {
		author = username
	}
	return author, threadsString(user["full_name"]), threadsString(user["profile_pic_url"])
}

func threadsPostPublishedAt(node map[string]any) time.Time {
	if seconds := threadsInt(node["taken_at"]); seconds > 0 {
		return time.Unix(seconds, 0).UTC()
	}
	if timestamp := threadsString(node["timestamp"]); timestamp != "" {
		if parsed, err := time.Parse(time.RFC3339, timestamp); err == nil {
			return parsed.UTC()
		}
	}
	return time.Now().UTC()
}

func threadsPostImage(node map[string]any) string {
	if versions, ok := node["image_versions2"].(map[string]any); ok {
		if candidates, ok := versions["candidates"].([]any); ok && len(candidates) > 0 {
			if first, ok := candidates[0].(map[string]any); ok {
				return threadsString(first["url"])
			}
		}
	}
	return threadsString(node["media_url"])
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
	if blocked, err := s.isThreadsAuthorBlocked(ctx, p.Author); err != nil || blocked {
		return false, err
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
	var existingThreadPostID, existingURL string
	err := s.db.QueryRowContext(ctx, `SELECT id,thread_post_id,url FROM articles WHERE url=?`, p.URL).Scan(&articleID, &existingThreadPostID, &existingURL)
	if errors.Is(err, sql.ErrNoRows) {
		fingerprint := articleFingerprint(p.Text, 20)
		if fingerprint != "" {
			err = s.db.QueryRowContext(ctx, `SELECT id,thread_post_id,url FROM articles WHERE content_fingerprint=? OR title_fingerprint=? LIMIT 1`, fingerprint, fingerprint).Scan(&articleID, &existingThreadPostID, &existingURL)
		}
	}
	isNew := errors.Is(err, sql.ErrNoRows)
	if err != nil && !isNew {
		return false, err
	}
	existingThread := existingThreadPostID != "" || allowedThreadsPostURL(existingURL)
	if isNew {
		r, e := s.db.ExecContext(ctx, `INSERT INTO articles(source_id,category_id,title,description,full_content,summary,url,image_url,content_images,published_at,title_fingerprint,content_fingerprint,thread_post_id,thread_author,thread_display_name,thread_avatar_url,thread_likes,thread_replies,thread_reposts,thread_classification,thread_classification_source) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, sourceID, categoryID, title, p.Text, p.Text, "", p.URL, p.ImageURL, "[]", p.PublishedAt.UTC().Format(time.RFC3339), articleFingerprint(title, 20), articleFingerprint(p.Text, 20), p.ID, p.Author, p.DisplayName, p.AvatarURL, p.Likes, p.Replies, p.Reposts, classification.Category, classification.Source)
		if e != nil {
			return false, e
		}
		articleID, _ = r.LastInsertId()
		_ = s.replaceArticleCategories(ctx, articleID, []categoryDefinition{{slug: classification.Category, name: classification.Category}})
	} else {
		if existingThread {
			// A Threads crawl is the authoritative source for these fields. In
			// particular, rewriting description/full_content repairs old rows that
			// were previously overwritten with the page's hydration payload.
			_, err = s.db.ExecContext(ctx, `UPDATE articles SET category_id=?,title=?,description=?,full_content=?,title_fingerprint=?,content_fingerprint=?,image_url=CASE WHEN ?<>'' THEN ? ELSE image_url END,thread_post_id=CASE WHEN thread_post_id<>'' THEN thread_post_id ELSE ? END,thread_author=?,thread_display_name=?,thread_avatar_url=?,thread_likes=?,thread_replies=?,thread_reposts=?,thread_classification=?,thread_classification_source=? WHERE id=?`, categoryID, title, p.Text, p.Text, articleFingerprint(title, 20), articleFingerprint(p.Text, 20), p.ImageURL, p.ImageURL, p.ID, p.Author, p.DisplayName, p.AvatarURL, p.Likes, p.Replies, p.Reposts, classification.Category, classification.Source, articleID)
		} else {
			_, err = s.db.ExecContext(ctx, `UPDATE articles SET image_url=CASE WHEN ?<>'' THEN ? ELSE image_url END WHERE id=?`, p.ImageURL, p.ImageURL, articleID)
		}
		if err != nil {
			return false, err
		}
		if existingThread {
			if err = s.replaceArticleCategories(ctx, articleID, []categoryDefinition{{slug: classification.Category, name: classification.Category}}); err != nil {
				return false, err
			}
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
	author = normalizeThreadsAuthor(author)
	if author == "" {
		return
	}
	if blocked, err := s.isThreadsAuthorBlocked(ctx, author); err != nil || blocked {
		return
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -7).Format(time.RFC3339)
	var posts, keywords, strongestEngagement int
	err := s.db.QueryRowContext(ctx, `SELECT count(DISTINCT a.id),count(DISTINCT t.query),COALESCE(MAX(a.thread_likes+a.thread_replies+a.thread_reposts),0) FROM articles a JOIN article_threads_targets att ON att.article_id=a.id JOIN threads_targets t ON t.id=att.target_id WHERE lower(a.thread_author)=? AND t.kind='keyword' AND a.published_at>=?`, author, cutoff).Scan(&posts, &keywords, &strongestEngagement)
	if err != nil || posts < 3 || keywords < 2 {
		return
	}
	// A transparent promotion score keeps discovery broad without following
	// every account that happens to match a keyword once. Three posts across
	// two searches qualify only when at least one post has modest engagement.
	promotionScore := threadsPromotionScore(posts, keywords, strongestEngagement)
	if promotionScore < 60 {
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
		log.Printf("threads auto-follow added: @%s score=%d qualified_posts=%d keywords=%d", author, promotionScore, posts, keywords)
	}
}

func threadsPromotionScore(posts, keywords, strongestEngagement int) int {
	if strongestEngagement < 0 {
		strongestEngagement = 0
	}
	return posts*8 + keywords*14 + min(strongestEngagement, 16)
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
	readJoin := "0"
	args := []any{}
	if u, logged := s.optionalThreadsReader(r); logged {
		readJoin = "EXISTS(SELECT 1 FROM article_reading_history arh WHERE arh.article_id=a.id AND arh.user_id=? AND arh.status='read')"
		args = append(args, u.ID)
		if q.Get("unread") == "1" {
			where = append(where, "NOT EXISTS(SELECT 1 FROM article_reading_history arh WHERE arh.article_id=a.id AND arh.user_id=? AND arh.status='read')")
			args = append(args, u.ID)
		}
	} else if q.Get("unread") == "1" {
		jsonErr(w, http.StatusUnauthorized, "a Telegram or admin session is required for unread posts")
		return
	}
	if q.Get("visibility") == "hidden" && s.isAdminRequest(r) {
		where = append(where, "a.is_hidden=1")
	} else {
		where = append(where, "a.is_hidden=0")
	}
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
	rows, err := s.db.QueryContext(r.Context(), `SELECT a.id,a.title,a.description,a.summary,a.url,a.image_url,s.name,s.id,s.country_code,s.country_name,c.slug,a.published_at,a.thread_author,a.thread_display_name,a.thread_avatar_url,a.thread_likes,a.thread_replies,a.thread_reposts,a.thread_classification,a.thread_classification_source,a.is_hidden,`+readJoin+` FROM articles a JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id WHERE `+strings.Join(where, " AND ")+` ORDER BY `+order+` LIMIT ? OFFSET ?`, args...)
	if err != nil {
		jsonErr(w, 500, "could not load Threads posts")
		return
	}
	defer rows.Close()
	out := []article{}
	for rows.Next() {
		var a article
		var hidden, read int
		if err = rows.Scan(&a.ID, &a.Title, &a.Description, &a.Summary, &a.URL, &a.ImageURL, &a.Source, &a.SourceID, &a.CountryCode, &a.CountryName, &a.Category, &a.PublishedAt, &a.ThreadAuthor, &a.ThreadDisplayName, &a.ThreadAvatarURL, &a.ThreadLikes, &a.ThreadReplies, &a.ThreadReposts, &a.ThreadClassification, &a.ThreadClassificationSource, &hidden, &read); err != nil {
			jsonErr(w, 500, "could not read Threads posts")
			return
		}
		a.Categories = []articleCategory{{Slug: a.Category, Name: a.Category}}
		a.ThreadPostID = "threads"
		a.IsHidden = hidden == 1
		a.IsRead = read == 1
		s.wrapThreadImage(&a)
		out = append(out, a)
	}
	jsonOut(w, 200, out)
}

// threadsAuthors supplies the autocomplete list for the public Threads view.
// It only exposes authors that already have a retained, enabled Threads post.
func (s *server) threadsAuthors(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT DISTINCT a.thread_author FROM articles a JOIN sources s ON s.id=a.source_id WHERE a.thread_post_id<>'' AND a.thread_author<>'' AND s.enabled=1 AND a.is_hidden=0 ORDER BY lower(a.thread_author) LIMIT 500`)
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

func (s *server) threadComments(w http.ResponseWriter, r *http.Request) {
	articleID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || articleID < 1 {
		jsonErr(w, http.StatusNotFound, "Threads post not found")
		return
	}
	visibility := "a.is_hidden=0"
	if s.isAdminRequest(r) {
		visibility = "1=1"
	}
	var postURL, postID string
	err = s.db.QueryRowContext(r.Context(), `SELECT a.url,a.thread_post_id FROM articles a JOIN sources s ON s.id=a.source_id WHERE a.id=? AND a.thread_post_id<>'' AND s.enabled=1 AND `+visibility, articleID).Scan(&postURL, &postID)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "Threads post not found")
		return
	}
	if err != nil || !allowedThreadsPostURL(postURL) {
		jsonErr(w, http.StatusBadGateway, "Threads comments are unavailable")
		return
	}
	comments, fetchedAt, err := s.cachedThreadComments(r.Context(), articleID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load Threads comments")
		return
	}
	if !fetchedAt.IsZero() && time.Since(fetchedAt) < threadsCommentCacheTTL {
		jsonOut(w, http.StatusOK, comments)
		return
	}
	body, err := threadsGET(r.Context(), postURL)
	if err != nil {
		jsonErr(w, http.StatusBadGateway, "Threads comments are unavailable")
		return
	}
	comments = rankThreadComments(threadsCommentsFromSSR(body, postID))
	filtered := comments[:0]
	for _, comment := range comments {
		if comment.Author == "" {
			continue
		}
		blocked, blockErr := s.isThreadsAuthorBlocked(r.Context(), comment.Author)
		if blockErr != nil {
			jsonErr(w, http.StatusInternalServerError, "could not filter Threads comments")
			return
		}
		if !blocked {
			filtered = append(filtered, comment)
		}
	}
	if err = s.replaceThreadComments(r.Context(), articleID, filtered); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not save Threads comments")
		return
	}
	jsonOut(w, http.StatusOK, filtered)
}

func (s *server) cachedThreadComments(ctx context.Context, articleID int64) ([]threadComment, time.Time, error) {
	var fetchedRaw string
	err := s.db.QueryRowContext(ctx, `SELECT fetched_at FROM threads_comment_fetches WHERE article_id=?`, articleID).Scan(&fetchedRaw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, time.Time{}, err
	}
	fetchedAt, _ := time.Parse(time.RFC3339, fetchedRaw)
	rows, err := s.db.QueryContext(ctx, `SELECT thread_comment_id,author,display_name,avatar_url,body,published_at,likes FROM threads_post_comments WHERE article_id=? ORDER BY likes DESC,published_at DESC`, articleID)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()
	comments := []threadComment{}
	for rows.Next() {
		var comment threadComment
		if err = rows.Scan(&comment.ID, &comment.Author, &comment.DisplayName, &comment.AvatarURL, &comment.Body, &comment.PublishedAt, &comment.Likes); err != nil {
			return nil, time.Time{}, err
		}
		comments = append(comments, comment)
	}
	return comments, fetchedAt, rows.Err()
}

func rankThreadComments(comments []threadComment) []threadComment {
	sort.SliceStable(comments, func(i, j int) bool {
		if comments[i].Likes == comments[j].Likes {
			return comments[i].PublishedAt > comments[j].PublishedAt
		}
		return comments[i].Likes > comments[j].Likes
	})
	if len(comments) > threadsCommentLimit {
		return comments[:threadsCommentLimit]
	}
	return comments
}

func (s *server) replaceThreadComments(ctx context.Context, articleID int64, comments []threadComment) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `DELETE FROM threads_post_comments WHERE article_id=?`, articleID); err != nil {
		return err
	}
	for _, comment := range comments {
		if _, err = tx.ExecContext(ctx, `INSERT INTO threads_post_comments(article_id,thread_comment_id,author,display_name,avatar_url,body,published_at,likes) VALUES(?,?,?,?,?,?,?,?)`, articleID, comment.ID, normalizeThreadsAuthor(comment.Author), comment.DisplayName, comment.AvatarURL, comment.Body, comment.PublishedAt, comment.Likes); err != nil {
			return err
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err = tx.ExecContext(ctx, `INSERT INTO threads_comment_fetches(article_id,fetched_at) VALUES(?,?) ON CONFLICT(article_id) DO UPDATE SET fetched_at=excluded.fetched_at`, articleID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *server) wrapThreadImage(article *article) {
	if article.ThreadPostID != "" && article.ID > 0 {
		article.URL = fmt.Sprintf("/go/threads/%d", article.ID)
	}
	if article.ThreadPostID != "" && article.ImageURL != "" {
		article.ImageURL = fmt.Sprintf("/api/threads/posts/%d/image", article.ID)
	}
	if article.ThreadPostID != "" && article.ThreadAvatarURL != "" {
		article.ThreadAvatarURL = fmt.Sprintf("/api/threads/posts/%d/avatar", article.ID)
	}
}

// redirectThread only resolves an internal article ID. It intentionally never
// accepts an arbitrary destination URL, preventing this endpoint becoming an
// open redirect.
func (s *server) redirectThread(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	visibility := "a.is_hidden=0"
	if s.isAdminRequest(r) {
		visibility = "1=1"
	}
	var original string
	err = s.db.QueryRowContext(r.Context(), `SELECT a.url FROM articles a JOIN sources s ON s.id=a.source_id WHERE a.id=? AND a.thread_post_id<>'' AND s.enabled=1 AND `+visibility, id).Scan(&original)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil || !allowedThreadsPostURL(original) {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, original, http.StatusFound)
}

func allowedThreadsPostURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "threads.com" || host == "www.threads.com"
}

func (s *server) threadImage(w http.ResponseWriter, r *http.Request) {
	s.threadAsset(w, r, "image_url")
}

func (s *server) threadAvatar(w http.ResponseWriter, r *http.Request) {
	s.threadAsset(w, r, "thread_avatar_url")
}

func (s *server) threadAsset(w http.ResponseWriter, r *http.Request, column string) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonErr(w, http.StatusNotFound, "Threads media not found")
		return
	}
	var imageURL string
	visibility := "a.is_hidden=0"
	if s.isAdminRequest(r) {
		visibility = "1=1"
	}
	err = s.db.QueryRowContext(r.Context(), `SELECT a.`+column+` FROM articles a JOIN sources s ON s.id=a.source_id WHERE a.id=? AND a.thread_post_id<>'' AND a.`+column+`<>'' AND s.enabled=1 AND `+visibility, id).Scan(&imageURL)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "Threads media not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load Threads media")
		return
	}
	if !allowedThreadsImageURL(imageURL) {
		jsonErr(w, http.StatusBadGateway, "Threads media source is unavailable")
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
		jsonErr(w, http.StatusBadGateway, "Threads media source is unavailable")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TelegramNewsImageProxy/1.0)")
	res, err := client.Do(req)
	if err != nil || res.StatusCode < 200 || res.StatusCode >= 300 {
		if res != nil {
			res.Body.Close()
		}
		jsonErr(w, http.StatusBadGateway, "Threads media source is unavailable")
		return
	}
	defer res.Body.Close()
	contentType := strings.ToLower(strings.TrimSpace(res.Header.Get("Content-Type")))
	if !strings.HasPrefix(contentType, "image/") || (res.ContentLength > maxThreadImageBytes) {
		jsonErr(w, http.StatusBadGateway, "Threads media source is unavailable")
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

func (s *server) adminThreadsAuthorModeration(w http.ResponseWriter, r *http.Request) {
	author := normalizeThreadsAuthor(chi.URLParam(r, "username"))
	if author == "" {
		jsonErr(w, http.StatusBadRequest, "invalid Threads username")
		return
	}
	state := threadsAuthorModeration{Username: author}
	var blocked, enabled int
	_ = s.db.QueryRowContext(r.Context(), `SELECT 1 FROM threads_author_blocks WHERE username=?`, author).Scan(&blocked)
	state.Blocked = blocked == 1
	if err := s.db.QueryRowContext(r.Context(), `SELECT count(*) FROM articles WHERE thread_post_id<>'' AND lower(thread_author)=?`, author).Scan(&state.PostCount); err != nil {
		jsonErr(w, 500, "could not count Threads posts")
		return
	}
	err := s.db.QueryRowContext(r.Context(), `SELECT id,enabled FROM threads_targets WHERE kind='profile' AND lower(query)=? LIMIT 1`, author).Scan(&state.ProfileTargetID, &enabled)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, 500, "could not load Threads profile target")
		return
	}
	state.ProfileEnabled = enabled == 1
	jsonOut(w, http.StatusOK, state)
}

func (s *server) adminBlockThreadsAuthor(w http.ResponseWriter, r *http.Request) {
	author := normalizeThreadsAuthor(chi.URLParam(r, "username"))
	if author == "" {
		jsonErr(w, http.StatusBadRequest, "invalid Threads username")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonErr(w, 500, "could not block Threads username")
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `INSERT OR IGNORE INTO threads_author_blocks(username) VALUES(?)`, author); err != nil {
		jsonErr(w, 500, "could not block Threads username")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE threads_targets SET enabled=0 WHERE kind='profile' AND lower(query)=?`, author); err != nil {
		jsonErr(w, 500, "could not disable Threads profile target")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE articles SET is_hidden=1,hidden_at=? WHERE thread_post_id<>'' AND lower(thread_author)=?`, time.Now().UTC().Format(time.RFC3339), author); err != nil {
		jsonErr(w, 500, "could not hide Threads author posts")
		return
	}
	if err = tx.Commit(); err != nil {
		jsonErr(w, 500, "could not block Threads username")
		return
	}
	jsonOut(w, http.StatusOK, map[string]bool{"blocked": true})
}

func (s *server) adminUnblockThreadsAuthor(w http.ResponseWriter, r *http.Request) {
	author := normalizeThreadsAuthor(chi.URLParam(r, "username"))
	if author == "" {
		jsonErr(w, http.StatusBadRequest, "invalid Threads username")
		return
	}
	if _, err := s.db.ExecContext(r.Context(), `DELETE FROM threads_author_blocks WHERE username=?`, author); err != nil {
		jsonErr(w, 500, "could not unblock Threads username")
		return
	}
	jsonOut(w, http.StatusOK, map[string]bool{"blocked": false})
}

func (s *server) adminDeleteThreadsAuthorPosts(w http.ResponseWriter, r *http.Request) {
	author := normalizeThreadsAuthor(chi.URLParam(r, "username"))
	if author == "" {
		jsonErr(w, http.StatusBadRequest, "invalid Threads username")
		return
	}
	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not remove Threads author")
		return
	}
	defer tx.Rollback()
	// Deleting an author's retained posts also means they must not be fetched
	// again on the next crawl. Keep the target recoverable but disable it and
	// blacklist the author in the same transaction as the deletion.
	if _, err = tx.ExecContext(r.Context(), `INSERT OR IGNORE INTO threads_author_blocks(username) VALUES(?)`, author); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not block Threads username")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE threads_targets SET enabled=0 WHERE kind='profile' AND lower(query)=?`, author); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not disable Threads profile target")
		return
	}
	result, err := tx.ExecContext(r.Context(), `DELETE FROM articles WHERE thread_post_id<>'' AND lower(thread_author)=?`, author)
	if err != nil {
		jsonErr(w, 500, "could not delete Threads posts")
		return
	}
	if err = tx.Commit(); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not remove Threads author")
		return
	}
	deleted, _ := result.RowsAffected()
	jsonOut(w, http.StatusOK, map[string]int64{"deleted": deleted})
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
