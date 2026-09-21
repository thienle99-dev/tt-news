package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestThreadsPostsFromSSR(t *testing.T) {
	body := []byte(`<html><script type="application/json">{"data":{"code":"DcYb2k5EuG2","pk":"3970045199010846531","taken_at":1789004347,"like_count":12,"comment_count":3,"repost_count":2,"caption":{"text":"A public post"},"user":{"username":"anzy_thien_an"},"image_versions2":{"candidates":[{"url":"https://image.test/post.jpg"}]}}}</script></html>`)
	posts := threadsPostsFromSSR(body, "fallback")
	if len(posts) != 1 {
		t.Fatalf("posts = %d, want 1", len(posts))
	}
	post := posts[0]
	if post.URL != "https://www.threads.com/@anzy_thien_an/post/DcYb2k5EuG2" || post.Text != "A public post" {
		t.Fatalf("post = %#v", post)
	}
	if post.Likes != 12 || post.Replies != 3 || post.Reposts != 2 || post.ImageURL != "https://image.test/post.jpg" {
		t.Fatalf("metadata = %#v", post)
	}
}

func TestThreadsPostsFromServerJSJSONStream(t *testing.T) {
	body := []byte(`<html><script>{"require":[["ScheduledServerJS",null,null,[]]]}{"data":{"permalink":"https://www.threads.com/@signal/post/abc123","id":"post-1","text":"Bài viết được nhúng trong Relay payload.","timestamp":"2026-09-10T08:00:00Z","like_count":112,"comment_count":8,"repost_count":3,"owner":{"username":"signal","full_name":"Signal Daily","profile_pic_url":"https://scontent.fsgn1-1.fna.fbcdn.net/avatar.jpg"},"media_url":"https://scontent.fsgn1-1.fna.fbcdn.net/image.jpg"}}</script></html>`)
	posts := threadsPostsFromSSR(body, "fallback")
	if len(posts) != 1 {
		t.Fatalf("posts = %#v, want one post from JSON stream", posts)
	}
	post := posts[0]
	if post.ID != "post-1" || post.Author != "signal" || post.DisplayName != "Signal Daily" || post.Likes != 112 || post.ImageURL == "" {
		t.Fatalf("post = %#v", post)
	}
}

func TestThreadsPostsFromSSRIgnoresHydrationPayload(t *testing.T) {
	body := []byte(`<html><script type="application/json">{"data":{"code":"server-payload","pk":"1","caption":{"text":"{\"require\":[[\"maybeDisableAnimations\",null,null,[]]]} {\"require\":[[\"bootstrapWebSession\",null,null,[1789027160]]]}"},"user":{"username":"threads"}}}</script></html>`)
	if posts := threadsPostsFromSSR(body, "fallback"); len(posts) != 0 {
		t.Fatalf("posts = %#v, want hydration payload to be rejected", posts)
	}
}

func TestThreadsCommentsFromSSRReturnsOnlyTopLevelRepliesToRequestedPost(t *testing.T) {
	body := []byte(`<html><script type="application/json">{"data":[{"pk":"root-1","code":"root-code","caption":{"text":"Original post"},"user":{"username":"owner"}},{"pk":"comment-1","is_reply":true,"reply_to_id":"root-1","caption":{"text":"Useful public reply"},"like_count":7,"taken_at":1789004347,"user":{"username":"reader","full_name":"Reader"}},{"pk":"nested-comment","is_reply":true,"reply_to_id":"comment-1","caption":{"text":"Nested reply"},"user":{"username":"other"}},{"pk":"unrelated","is_reply":true,"reply_to_id":"other-root","caption":{"text":"Recommendation reply"},"user":{"username":"other"}}]}</script></html>`)
	comments := threadsCommentsFromSSR(body, "root-1")
	if len(comments) != 1 {
		t.Fatalf("comments = %#v, want only direct reply", comments)
	}
	if comments[0].ID != "comment-1" || comments[0].Author != "reader" || comments[0].Likes != 7 {
		t.Fatalf("comment = %#v", comments[0])
	}
}

func TestValidThreadPostText(t *testing.T) {
	if validThreadPostText(`{"require":[["CometSSRMergedContentInjector","ssrInit",null,[]]]}`) {
		t.Fatal("hydration payload was accepted")
	}
	if !validThreadPostText("Hà Nội công bố kế hoạch mở rộng tuyến metro trong năm nay.") {
		t.Fatal("normal Threads post was rejected")
	}
}

func TestSelectDiverseThreadCandidatesEnforcesHardAuthorAndCategoryLimits(t *testing.T) {
	candidate := func(author, category string, score float64) threadCandidate {
		return threadCandidate{
			post:           threadPost{Author: author, PublishedAt: time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)},
			classification: threadClassification{Category: category},
			score:          score,
		}
	}
	selected := selectDiverseThreadCandidates([]threadCandidate{
		candidate("dominant", "technology", 100),
		candidate("dominant", "technology", 99),
		candidate("other-one", "technology", 98),
		candidate("other-two", "technology", 97),
		candidate("other-three", "business", 96),
	}, 3)
	if len(selected) != 3 {
		t.Fatalf("selected = %d, want 3", len(selected))
	}
	authors, categories := map[string]int{}, map[string]int{}
	for _, item := range selected {
		authors[item.post.Author]++
		categories[item.classification.Category]++
	}
	if authors["dominant"] != 1 || categories["technology"] != threadsCategoryLimit {
		t.Fatalf("hard limits were not applied; selected=%#v", selected)
	}
}

func TestSelectDiverseThreadCandidatesFallsBackWhenDiversePoolIsTooSmall(t *testing.T) {
	candidate := func(score float64) threadCandidate {
		return threadCandidate{post: threadPost{Author: "dominant"}, classification: threadClassification{Category: "technology"}, score: score}
	}
	selected := selectDiverseThreadCandidates([]threadCandidate{candidate(100), candidate(99), candidate(98)}, 3)
	if len(selected) != 3 {
		t.Fatalf("selected = %d, want fallback to fill all candidates", len(selected))
	}
}

func TestDiverseThreadsSeedPostsPrefersDistinctAuthors(t *testing.T) {
	posts := []threadPost{
		{URL: "https://www.threads.com/@alice/post/one", Author: "alice"},
		{URL: "https://www.threads.com/@alice/post/two", Author: "alice"},
		{URL: "https://www.threads.com/@bob/post/one", Author: "bob"},
		{URL: "https://www.threads.com/@carol/post/one", Author: "carol"},
	}
	selected := diverseThreadsSeedPosts(posts, 3)
	if len(selected) != 3 || selected[0].Author != "alice" || selected[1].Author != "bob" || selected[2].Author != "carol" {
		t.Fatalf("seed posts = %#v, want first post from each author", selected)
	}
}

func TestInterleaveThreadsTargetsAlternatesDiscoveryAndProfiles(t *testing.T) {
	ordered := interleaveThreadsTargets([]threadsTarget{
		{ID: 1, Kind: "profile", Query: "first-profile"},
		{ID: 2, Kind: "profile", Query: "second-profile"},
		{ID: 3, Kind: "keyword", Query: "first-keyword"},
		{ID: 4, Kind: "keyword", Query: "second-keyword"},
	})
	if len(ordered) != 4 || ordered[0].Kind != "keyword" || ordered[1].Kind != "profile" || ordered[2].Kind != "keyword" || ordered[3].Kind != "profile" {
		t.Fatalf("target order = %#v", ordered)
	}
}

func TestThreadsTargetBeforePrioritizesNewThenUnhealthyTargets(t *testing.T) {
	newTarget := threadsTarget{ID: 1}
	unhealthy := threadsTarget{ID: 2, LastSuccessAt: "2026-09-10T07:00:00Z", LastError: "Threads returned 429"}
	healthy := threadsTarget{ID: 3, LastSuccessAt: "2026-09-10T07:00:00Z", LastFetchAt: "2026-09-10T07:00:00Z"}
	if !threadsTargetBefore(newTarget, unhealthy) || !threadsTargetBefore(unhealthy, healthy) {
		t.Fatalf("targets were not ordered new, unhealthy, healthy")
	}
}

func TestProfileCandidateFilterKeepsOnlyTargetAuthor(t *testing.T) {
	posts := filterThreadsPosts([]threadPost{{Author: "Target_Account"}, {Author: "recommended_account"}}, func(post threadPost) bool {
		return normalizeThreadsAuthor(post.Author) == normalizeThreadsAuthor("@target_account")
	})
	if len(posts) != 1 || posts[0].Author != "Target_Account" {
		t.Fatalf("profile posts = %#v, want only target author", posts)
	}
}

func TestThreadsPromotionScoreRequiresQualitySignal(t *testing.T) {
	if score := threadsPromotionScore(3, 2, 0); score >= 60 {
		t.Fatalf("score without engagement = %d, want below promotion threshold", score)
	}
	if score := threadsPromotionScore(3, 2, 8); score != 60 {
		t.Fatalf("score with modest engagement = %d, want 60", score)
	}
}

func TestAdminDeleteThreadsTargetPostsKeepsSharedPosts(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
		INSERT INTO categories(id,slug,name) VALUES(9501,'threads-delete','Threads delete');
		INSERT INTO sources(id,name,feed_url,category_id,enabled) VALUES(9501,'Threads delete','https://threads.test/delete',9501,1);
		INSERT INTO articles(id,source_id,category_id,title,description,url,published_at,thread_post_id) VALUES
			(9501,9501,9501,'Only target one','post','https://threads.test/one','2026-01-01T00:00:00Z','post-one'),
			(9502,9501,9501,'Shared','post','https://threads.test/two','2026-01-01T00:00:00Z','post-two');
		INSERT INTO threads_targets(id,kind,query) VALUES(9501,'profile','threads_test_alice'),(9502,'keyword','threads test keyword');
		INSERT INTO article_threads_targets(article_id,target_id) VALUES(9501,9501),(9502,9501),(9502,9502);
	`); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db}
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/threads/targets/9501/posts", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "9501")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	response := httptest.NewRecorder()
	s.adminDeleteThreadsTargetPosts(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
	var deleted struct {
		Deleted int `json:"deleted"`
	}
	if err = json.NewDecoder(response.Body).Decode(&deleted); err != nil {
		t.Fatal(err)
	}
	if deleted.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted.Deleted)
	}
	var articles, targetLinks int
	if err = db.QueryRow(`SELECT count(*) FROM articles WHERE id IN (9501,9502)`).Scan(&articles); err != nil || articles != 1 {
		t.Fatalf("articles = %d, err = %v; want shared article only", articles, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM article_threads_targets WHERE target_id=9501`).Scan(&targetLinks); err != nil || targetLinks != 0 {
		t.Fatalf("target links = %d, err = %v; want 0", targetLinks, err)
	}
}

func TestAdminDeleteThreadsAuthorPostsBlocksAndUnfollowsAuthor(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
		INSERT INTO categories(id,slug,name) VALUES(9301,'threads-author-delete','Threads author delete');
		INSERT INTO sources(id,name,feed_url,category_id,enabled) VALUES(9301,'Threads author delete','https://threads.test/author-delete',9301,1);
		INSERT INTO articles(id,source_id,category_id,title,description,url,published_at,thread_post_id,thread_author) VALUES(9301,9301,9301,'Post','Post','https://threads.test/post','2026-01-01T00:00:00Z','post-1','Alice');
		INSERT INTO threads_targets(id,kind,query,enabled) VALUES(9301,'profile','alice',1);
	`); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db}
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/threads/authors/alice/posts", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("username", "alice")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
	response := httptest.NewRecorder()
	s.adminDeleteThreadsAuthorPosts(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
	var articles, blocks, enabled int
	if err = db.QueryRow(`SELECT count(*) FROM articles WHERE id=9301`).Scan(&articles); err != nil || articles != 0 {
		t.Fatalf("articles = %d, err = %v; want deleted", articles, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM threads_author_blocks WHERE username='alice'`).Scan(&blocks); err != nil || blocks != 1 {
		t.Fatalf("blocks = %d, err = %v; want author blocked", blocks, err)
	}
	if err = db.QueryRow(`SELECT enabled FROM threads_targets WHERE id=9301`).Scan(&enabled); err != nil || enabled != 0 {
		t.Fatalf("target enabled = %d, err = %v; want unfollowed", enabled, err)
	}
}

func TestListThreadsFiltersByUsername(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
		INSERT INTO categories(id,slug,name) VALUES(9401,'threads-test','Threads test');
		INSERT INTO sources(id,name,feed_url,category_id,enabled) VALUES(9401,'Threads test','https://threads.test/source',9401,1);
		INSERT INTO articles(id,source_id,category_id,title,description,url,published_at,thread_post_id,thread_author) VALUES
			(9401,9401,9401,'First','First post','https://threads.test/1','2026-01-01T00:00:00Z','post-1','Alice_Dev'),
			(9402,9401,9401,'Second','Second post','https://threads.test/2','2026-01-02T00:00:00Z','post-2','bob');
	`); err != nil {
		t.Fatal(err)
	}

	s := &server{db: db}
	request := httptest.NewRequest(http.MethodGet, "/api/threads?username=@alice", nil)
	response := httptest.NewRecorder()
	s.listThreads(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
	var items []article
	if err = json.NewDecoder(response.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != 9401 || items[0].ThreadAuthor != "Alice_Dev" {
		t.Fatalf("items = %#v", items)
	}
}
