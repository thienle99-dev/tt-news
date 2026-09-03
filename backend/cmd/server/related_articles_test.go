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

func TestRelatedArticlesPrioritizeSharedTopicsAndExcludeOldOrDisabled(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err = db.Exec(`
		INSERT INTO categories(id,slug,name) VALUES(8001,'related-one','Related one'),(8002,'related-two','Related two'),(8003,'related-other','Related other');
		INSERT INTO sources(id,name,feed_url,category_id,enabled) VALUES(8001,'Target source','https://related.test/target',8001,1),(8002,'Other source','https://related.test/other',8003,1),(8003,'Disabled source','https://related.test/disabled',8001,0);
		INSERT INTO articles(id,source_id,category_id,title,url,published_at) VALUES
			(8001,8001,8001,'Target','https://related.test/target-article',?),
			(8002,8002,8001,'Two shared topics','https://related.test/two',?),
			(8003,8001,8003,'Same source only','https://related.test/source',?),
			(8004,8002,8001,'One shared topic','https://related.test/one',?),
			(8005,8002,8001,'Old topic match','https://related.test/old',?),
			(8006,8003,8001,'Disabled source match','https://related.test/disabled-article',?);
		INSERT INTO article_categories(article_id,category_id) VALUES
			(8001,8001),(8001,8002),(8002,8001),(8002,8002),(8003,8003),(8004,8001),(8005,8001),(8006,8001);`,
		now.Format(time.RFC3339), now.Add(-time.Hour).Format(time.RFC3339), now.Add(-2*time.Hour).Format(time.RFC3339), now.Add(-3*time.Hour).Format(time.RFC3339), now.AddDate(0, 0, -31).Format(time.RFC3339), now.Add(-4*time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}

	s := &server{db: db}
	request := httptest.NewRequest(http.MethodGet, "/api/articles/8001/related", nil)
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "8001")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	response := httptest.NewRecorder()
	s.relatedArticles(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("related response = %d: %s", response.Code, response.Body.String())
	}
	var items []article
	if err = json.NewDecoder(response.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("related count = %d, want 3: %#v", len(items), items)
	}
	if items[0].ID != 8002 || items[1].ID != 8004 || items[2].ID != 8003 {
		t.Fatalf("unexpected related ranking: %#v", items)
	}
}
