package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorthReadRanksFreshCompleteAndHealthySources(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err = db.Exec(`
		INSERT INTO categories(id,slug,name) VALUES(9101,'worth-read','Worth read');
		INSERT INTO sources(id,name,feed_url,category_id,enabled) VALUES
			(9101,'Healthy','https://worth-read.test/healthy',9101,1),
			(9102,'Unknown','https://worth-read.test/unknown',9101,1);
		INSERT INTO source_health(source_id,last_fetch_at,last_success_at,last_error) VALUES(9101,?,?, '');
		INSERT INTO articles(id,source_id,category_id,title,description,full_content,url,published_at) VALUES
			(9101,9102,9101,'Fresh but incomplete','short','', 'https://worth-read.test/incomplete',?),
			(9102,9102,9101,'Fresh and complete','short',?, 'https://worth-read.test/complete',?),
			(9103,9101,9101,'Fresh healthy complete','short',?, 'https://worth-read.test/best',?);`,
		now, now, now,
		strings.Repeat("x", 600), now,
		strings.Repeat("x", 600), now); err != nil {
		t.Fatal(err)
	}

	s := &server{db: db}
	request := httptest.NewRequest(http.MethodGet, "/api/articles?sort=worth_read", nil)
	response := httptest.NewRecorder()
	s.listArticles(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
	var items []article
	if err = json.NewDecoder(response.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].ID != 9103 || items[1].ID != 9102 || items[2].ID != 9101 {
		t.Fatalf("unexpected worth-read order: %#v", items)
	}
}
