package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestDailyDigestArticlesMatchesAnyFollowedTopicOrSource(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
		INSERT INTO users(id,telegram_id,first_name) VALUES(7001,7001,'Digest reader');
		INSERT INTO categories(id,slug,name) VALUES(7001,'digest-topic','Digest topic'),(7002,'other-topic','Other topic'),(7003,'secondary-topic','Secondary topic');
		INSERT INTO sources(id,name,feed_url,category_id,enabled) VALUES(7001,'Followed source','https://digest.test/followed',7002,1),(7002,'Topic source','https://digest.test/topic',7002,1),(7003,'Unrelated source','https://digest.test/other',7002,1);
		INSERT INTO articles(id,source_id,category_id,title,url,published_at) VALUES
			(7001,7001,7002,'Matched by source','https://digest.test/a','2026-08-28T01:00:00Z'),
			(7002,7002,7001,'Matched by primary topic','https://digest.test/b','2026-08-28T02:00:00Z'),
			(7003,7002,7002,'Matched by extra topic','https://digest.test/c','2026-08-28T03:00:00Z'),
			(7004,7003,7002,'Do not send','https://digest.test/d','2026-08-28T04:00:00Z');
		INSERT INTO article_categories(article_id,category_id) VALUES(7003,7001);
		INSERT INTO user_daily_digest_sources(user_id,source_id) VALUES(7001,7001);
		INSERT INTO user_daily_digest_categories(user_id,category_id) VALUES(7001,7001);
	`); err != nil {
		t.Fatal(err)
	}

	items, err := (&server{db: db}).dailyDigestArticles(context.Background(), 7001, time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("matched article count = %d, want 3 (%#v)", len(items), items)
	}
	if items[0].ID != 7003 || items[1].ID != 7002 || items[2].ID != 7001 {
		t.Fatalf("articles should be newest first and exclude unrelated: %#v", items)
	}
}
