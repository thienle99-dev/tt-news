package main

import "testing"

func TestRSSItemCategory(t *testing.T) {
	tests := []struct {
		name       string
		categories []string
		fallback   string
		want       string
	}{
		{name: "matches case insensitively", categories: []string{"News", " AI "}, fallback: "technology", want: "ai"},
		{name: "accepts each supported category", categories: []string{"cybersecurity"}, fallback: "technology", want: "cybersecurity"},
		{name: "maps common AI aliases", categories: []string{"Artificial Intelligence"}, fallback: "technology", want: "ai"},
		{name: "keeps source category when no supported tag exists", categories: []string{"technology"}, fallback: "programming", want: "programming"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rssItemCategory(test.categories, test.fallback); got != test.want {
				t.Fatalf("rssItemCategory(%v, %q) = %q, want %q", test.categories, test.fallback, got, test.want)
			}
		})
	}
}

func TestRSSItemCategoriesKeepsEveryTag(t *testing.T) {
	got := rssItemCategories([]string{"Artificial Intelligence", "Research & Development", "Artificial Intelligence"}, "technology")
	want := []string{"ai", "research-development"}
	if len(got) != len(want) {
		t.Fatalf("rssItemCategories returned %d categories, want %d", len(got), len(want))
	}
	for index, slug := range want {
		if got[index].slug != slug {
			t.Fatalf("category %d = %q, want %q", index, got[index].slug, slug)
		}
	}
}
