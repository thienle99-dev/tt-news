package main

import (
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAdminSourceIDsDeduplicatesAndIgnoresInvalidIDs(t *testing.T) {
	request := httptest.NewRequest("POST", "/api/admin/rss/fetch", strings.NewReader(`{"source_ids":[3,0,3,-1,7]}`))
	ids, err := adminSourceIDs(request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []int64{3, 7}) {
		t.Fatalf("source IDs = %#v", ids)
	}
}

func TestEnqueueSourceTranslationsOnlyQueuesSelectedSources(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO categories(id,slug,name) VALUES(100,'test','Test');
		INSERT INTO sources(id,name,feed_url,category_id,enabled) VALUES(101,'Alpha','https://alpha.test/feed',100,1),(102,'Beta','https://beta.test/feed',100,1);
		INSERT INTO articles(id,source_id,category_id,title,url,published_at) VALUES
			(201,101,100,'Alpha one','https://alpha.test/1','2026-08-27T00:00:00Z'),
			(202,101,100,'Alpha two','https://alpha.test/2','2026-08-27T00:00:00Z'),
			(203,102,100,'Beta one','https://beta.test/1','2026-08-27T00:00:00Z');
		INSERT INTO article_translations(article_id,language_code,title) VALUES(202,'vi','Đã dịch');`); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db}
	queued, err := s.enqueueSourceTranslations(t.Context(), []int64{101})
	if err != nil {
		t.Fatal(err)
	}
	if queued != 1 {
		t.Fatalf("queued = %d", queued)
	}
	var articleID int64
	if err = db.QueryRow("SELECT article_id FROM translation_jobs").Scan(&articleID); err != nil {
		t.Fatal(err)
	}
	if articleID != 201 {
		t.Fatalf("queued article = %d", articleID)
	}
}
