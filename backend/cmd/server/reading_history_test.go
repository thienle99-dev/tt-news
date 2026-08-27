package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestReadingHistoryMarksReadsAndCanBeCleared(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO users(id,telegram_id,first_name) VALUES(1,1,'Reader');
		INSERT INTO categories(id,slug,name) VALUES(100,'test','Test');
		INSERT INTO sources(id,name,feed_url,category_id) VALUES(100,'Test source','https://example.test/feed',100);
		INSERT INTO articles(id,source_id,category_id,title,url,published_at) VALUES(100,100,100,'Test article','https://example.test/article','2026-08-27T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db}
	request := httptest.NewRequest("POST", "/api/articles/100/read", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "100")
	request = request.WithContext(context.WithValue(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext), userKey, user{ID: 1}))
	response := httptest.NewRecorder()
	s.markRead(response, request)
	if response.Code != 200 {
		t.Fatalf("mark read status = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest("GET", "/api/reading-history", nil)
	request = request.WithContext(context.WithValue(request.Context(), userKey, user{ID: 1}))
	response = httptest.NewRecorder()
	s.readingHistory(response, request)
	var items []article
	if err = json.NewDecoder(response.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].IsRead || items[0].ID != 100 {
		t.Fatalf("unexpected history: %#v", items)
	}

	request = httptest.NewRequest("DELETE", "/api/reading-history", nil)
	request = request.WithContext(context.WithValue(request.Context(), userKey, user{ID: 1}))
	response = httptest.NewRecorder()
	s.clearReadingHistory(response, request)
	var count int
	if err = db.QueryRow("SELECT count(*) FROM article_reading_history WHERE user_id=1").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("history rows after clear = %d", count)
	}
}
