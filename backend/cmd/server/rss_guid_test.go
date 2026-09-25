package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestFetchSourceDeduplicatesNewGUIDAndKeepsLegacyURLDedup(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	var categoryID int64
	if err = db.QueryRow("SELECT id FROM categories WHERE slug='technology'").Scan(&categoryID); err != nil {
		t.Fatal(err)
	}

	feeds := []string{
		`<rss version="2.0"><channel><title>GUID test</title><item><title>Original headline for stable GUID test</title><link>https://publisher.test/first-story</link><guid isPermaLink="false"> stable-item-42 </guid><description>Original body.</description></item></channel></rss>`,
		`<rss version="2.0"><channel><title>GUID test</title><item><title>Updated headline after publisher URL change</title><link>https://publisher.test/updated-story</link><guid isPermaLink="false">stable-item-42</guid><description>Updated body text.</description></item></channel></rss>`,
		`<rss version="2.0"><channel><title>GUID test</title><item><title>Legacy URL article title stays stable</title><link>https://publisher.test/legacy-story</link><description>Legacy body.</description></item></channel></rss>`,
	}
	feedIndex := 0
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feeds[feedIndex]))
		feedIndex++
	}))
	defer feedServer.Close()

	if _, err = db.Exec(`INSERT INTO sources(id,name,feed_url,category_id,enabled) VALUES(9901,'GUID test source',?,?,1)`, feedServer.URL, categoryID); err != nil {
		t.Fatal(err)
	}
	src := source{ID: 9901, Name: "GUID test source", FeedURL: feedServer.URL, CategoryID: categoryID, Category: "technology"}
	app := &server{db: db}

	first, err := app.fetchSource(context.Background(), src, zeroTime)
	if err != nil || first.Inserted != 1 {
		t.Fatalf("first fetch = (%+v, %v), want one inserted article", first, err)
	}
	second, err := app.fetchSource(context.Background(), src, zeroTime)
	if err != nil || second.Existing != 1 || second.Inserted != 0 {
		t.Fatalf("changed-URL fetch = (%+v, %v), want GUID duplicate", second, err)
	}
	var count int
	if err = db.QueryRow("SELECT count(*) FROM articles WHERE source_id=?", src.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("article count after GUID duplicate = %d, err=%v; want 1", count, err)
	}
	var storedHash string
	if err = db.QueryRow("SELECT rss_guid_hash FROM articles WHERE source_id=?", src.ID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if storedHash != rssItemGUIDHash("stable-item-42") {
		t.Fatalf("stored GUID hash = %q, want hash of stable GUID", storedHash)
	}

	if _, err = db.Exec(`INSERT INTO articles(source_id,category_id,title,description,url,published_at) VALUES(?,?,?,?,?,CURRENT_TIMESTAMP)`, src.ID, categoryID, "Legacy article", "Existing body", "https://publisher.test/legacy-story"); err != nil {
		t.Fatal(err)
	}
	legacy, err := app.fetchSource(context.Background(), src, zeroTime)
	if err != nil || legacy.Existing != 1 || legacy.Inserted != 0 {
		t.Fatalf("legacy URL fetch = (%+v, %v), want URL duplicate", legacy, err)
	}
	if err = db.QueryRow("SELECT count(*) FROM articles WHERE source_id=?", src.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("article count after legacy URL duplicate = %d, err=%v; want 2", count, err)
	}
	if feedIndex != len(feeds) {
		t.Fatalf("feed responses consumed = %d, want %d", feedIndex, len(feeds))
	}
}

var zeroTime = time.Time{}
