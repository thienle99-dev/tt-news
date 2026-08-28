package main

import "testing"

func TestBestArticleBodyUsesLongestAvailablePublicText(t *testing.T) {
	if got := bestArticleBody("short", "longer RSS description", "fresh page text"); got != "longer RSS description" {
		t.Fatalf("bestArticleBody() = %q", got)
	}
	if got := bestArticleBody("stored", "RSS", "fresh page text that is longer"); got != "fresh page text that is longer" {
		t.Fatalf("bestArticleBody() = %q", got)
	}
}
