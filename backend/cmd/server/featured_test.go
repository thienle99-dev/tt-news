package main

import (
	"testing"

	translationservice "telegram-news/internal/translation"
)

func TestValidateFeaturedBrief(t *testing.T) {
	candidates := make([]translationservice.FeaturedCandidate, 5)
	topics := make([]translationservice.FeaturedTopic, 5)
	for index := range candidates {
		id := int64(index + 1)
		candidates[index] = translationservice.FeaturedCandidate{ID: id}
		topics[index] = translationservice.FeaturedTopic{Title: "Topic", Summary: "Summary", ArticleIDs: []int64{id}}
	}
	brief := translationservice.FeaturedBrief{Title: "Featured", Intro: "Intro", Topics: topics}
	if err := validateFeaturedBrief(brief, candidates); err != nil {
		t.Fatalf("valid brief rejected: %v", err)
	}
	brief.Topics[1].ArticleIDs[0] = 1
	if err := validateFeaturedBrief(brief, candidates); err == nil {
		t.Fatal("duplicate article accepted")
	}
}

func TestValidateFeaturedTranslation(t *testing.T) {
	original := translationservice.FeaturedBrief{Title: "Featured", Intro: "Intro", Topics: []translationservice.FeaturedTopic{{Title: "Topic", Summary: "Summary", ArticleIDs: []int64{1, 2}}, {Title: "Topic 2", Summary: "Summary", ArticleIDs: []int64{3}}}}
	translated := translationservice.FeaturedBrief{Title: "Nổi bật", Intro: original.Intro, Topics: []translationservice.FeaturedTopic{{Title: "Chủ đề", Summary: "Tóm tắt", ArticleIDs: []int64{1, 2}}, {Title: "Chủ đề 2", Summary: "Tóm tắt", ArticleIDs: []int64{3}}}}
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
