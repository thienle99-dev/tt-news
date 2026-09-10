package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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

func TestThreadsPostsFromSSRIgnoresHydrationPayload(t *testing.T) {
	body := []byte(`<html><script type="application/json">{"data":{"code":"server-payload","pk":"1","caption":{"text":"{\"require\":[[\"maybeDisableAnimations\",null,null,[]]]} {\"require\":[[\"bootstrapWebSession\",null,null,[1789027160]]]}"},"user":{"username":"threads"}}}</script></html>`)
	if posts := threadsPostsFromSSR(body, "fallback"); len(posts) != 0 {
		t.Fatalf("posts = %#v, want hydration payload to be rejected", posts)
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
