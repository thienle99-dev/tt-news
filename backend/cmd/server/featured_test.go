package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	translationservice "telegram-news/internal/translation"
)

func TestFeaturedReleasesTopicRowsBeforeLoadingArticles(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
		INSERT INTO categories(id,slug,name) VALUES(900,'featured-test','Featured test');
		INSERT INTO sources(id,name,feed_url,category_id) VALUES(900,'Featured test source','https://example.test/featured',900);
		INSERT INTO articles(id,source_id,category_id,title,summary,url,published_at) VALUES(900,900,900,'Featured article','Article summary','https://example.test/article','2026-08-28T00:00:00Z');
		INSERT INTO featured_briefs(id,slot_start,generated_at,window_start,window_end,title,intro) VALUES(900,'2026-08-28T00:00:00Z','2026-08-28T00:00:00Z','2026-08-27T00:00:00Z','2026-08-28T00:00:00Z','Featured','Intro');
		INSERT INTO featured_brief_takeaways(brief_id,position,text) VALUES(900,0,'Takeaway');
		INSERT INTO featured_topics(id,brief_id,position,title,summary,why_it_matters) VALUES(900,900,0,'Topic','Summary','Impact');
		INSERT INTO featured_topic_articles(topic_id,article_id,position) VALUES(900,900,0);
	`); err != nil {
		t.Fatal(err)
	}

	s := &server{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request := httptest.NewRequest("GET", "/api/featured?lang=vi", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	s.featured(response, request)
	if response.Code != 200 {
		t.Fatalf("featured status = %d: %s", response.Code, response.Body.String())
	}
	var brief featuredBrief
	if err = json.NewDecoder(response.Body).Decode(&brief); err != nil {
		t.Fatal(err)
	}
	if len(brief.Topics) != 1 || len(brief.Topics[0].Articles) != 1 {
		t.Fatalf("unexpected featured brief: %#v", brief)
	}
}

func TestValidateFeaturedBrief(t *testing.T) {
	candidates := make([]translationservice.FeaturedCandidate, 3)
	topics := make([]translationservice.FeaturedTopic, 3)
	for index := range candidates {
		id := int64(index + 1)
		candidates[index] = translationservice.FeaturedCandidate{ID: id}
		topics[index] = translationservice.FeaturedTopic{Title: "Topic", Summary: "Summary", WhyItMatters: "Impact", ArticleIDs: []int64{id}}
	}
	brief := translationservice.FeaturedBrief{Title: "Featured", Intro: "Intro", Takeaways: []string{"One", "Two", "Three"}, Topics: topics}
	if err := validateFeaturedBrief(brief, candidates, defaultFeaturedArticleTarget); err != nil {
		t.Fatalf("valid brief rejected: %v", err)
	}
	brief.Topics[1].ArticleIDs[0] = 1
	if err := validateFeaturedBrief(brief, candidates, defaultFeaturedArticleTarget); err == nil {
		t.Fatal("duplicate article accepted")
	}
	brief.Topics[1].ArticleIDs[0] = 2
	brief.Takeaways = brief.Takeaways[:2]
	if err := validateFeaturedBrief(brief, candidates, defaultFeaturedArticleTarget); err == nil {
		t.Fatal("brief with fewer than three takeaways accepted")
	}
}

func TestValidateFeaturedBriefRequiresDailyArticleTarget(t *testing.T) {
	candidates := make([]translationservice.FeaturedCandidate, defaultFeaturedArticleTarget)
	for index := range candidates {
		candidates[index] = translationservice.FeaturedCandidate{ID: int64(index + 1)}
	}
	topics := make([]translationservice.FeaturedTopic, 7)
	articleID := int64(1)
	for index := range topics {
		count := 3
		if index == len(topics)-1 {
			count = 2
		}
		for range count {
			topics[index].ArticleIDs = append(topics[index].ArticleIDs, articleID)
			articleID++
		}
		topics[index].Title = "Topic"
		topics[index].Summary = "Summary"
		topics[index].WhyItMatters = "Impact"
	}
	brief := translationservice.FeaturedBrief{Title: "Daily briefing", Intro: "Intro", Takeaways: []string{"One", "Two", "Three"}, Topics: topics}
	if err := validateFeaturedBrief(brief, candidates, defaultFeaturedArticleTarget); err != nil {
		t.Fatalf("20-article brief rejected: %v", err)
	}
	brief.Topics[len(brief.Topics)-1].ArticleIDs = brief.Topics[len(brief.Topics)-1].ArticleIDs[:1]
	if err := validateFeaturedBrief(brief, candidates, defaultFeaturedArticleTarget); err == nil {
		t.Fatal("brief with fewer than 20 selected articles accepted")
	}
}

func TestValidateFeaturedTranslation(t *testing.T) {
	original := translationservice.FeaturedBrief{Title: "Featured", Intro: "Intro", Takeaways: []string{"One", "Two", "Three"}, Topics: []translationservice.FeaturedTopic{{Title: "Topic", Summary: "Summary", WhyItMatters: "Impact", ArticleIDs: []int64{1, 2}}, {Title: "Topic 2", Summary: "Summary", WhyItMatters: "Impact", ArticleIDs: []int64{3}}}}
	translated := translationservice.FeaturedBrief{Title: "Nổi bật", Intro: original.Intro, Takeaways: []string{"Một", "Hai", "Ba"}, Topics: []translationservice.FeaturedTopic{{Title: "Chủ đề", Summary: "Tóm tắt", WhyItMatters: "Tác động", ArticleIDs: []int64{1, 2}}, {Title: "Chủ đề 2", Summary: "Tóm tắt", WhyItMatters: "Tác động", ArticleIDs: []int64{3}}}}
	if err := validateFeaturedTranslation(translated, original); err != nil {
		t.Fatalf("valid translation rejected: %v", err)
	}
	translated.Topics[0].ArticleIDs[0] = 4
	if err := validateFeaturedTranslation(translated, original); err == nil {
		t.Fatal("changed article ID accepted")
	}
}

func TestIsObviousJunk(t *testing.T) {
	for _, title := range []string{"Weekly weather forecast for London", "Podcast: Global music", "In pictures: the red carpet", "Travel guide to Hanoi"} {
		if !isObviousJunk(title) {
			t.Fatalf("expected junk: %q", title)
		}
	}
	if isObviousJunk("Weather emergency prompts evacuation") {
		t.Fatal("ordinary reporting must not be pre-filtered")
	}
}
