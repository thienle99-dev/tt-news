package main

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestNormalizeMediumURL(t *testing.T) {
	valid, err := normalizeMediumURL("https://medium.com/@leo-godin/claude-code-is-great-6db35d8685f0?utm_source=x&sk=allowed", true)
	if err != nil || valid.String() != "https://medium.com/@leo-godin/claude-code-is-great-6db35d8685f0?sk=allowed" {
		t.Fatalf("normalized URL = %v, %v", valid, err)
	}
	for _, raw := range []string{
		"http://medium.com/@a/story-6db35d8685f0",
		"https://medium.com.evil.test/@a/story-6db35d8685f0",
		"https://user@medium.com/@a/story-6db35d8685f0",
		"https://127.0.0.1/@a/story-6db35d8685f0",
		"https://medium.com:8443/@a/story-6db35d8685f0",
	} {
		if _, err := normalizeMediumURL(raw, true); err == nil {
			t.Fatalf("%q was accepted", raw)
		}
	}
}

func TestExtractMediumReaderSanitizesHTML(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
    <html><head><meta property="og:title" content="Safe title"><meta name="author" content="Author"></head><body>
    <article><h1>Safe title</h1><p>Hello <strong>world</strong>.</p><script>alert(1)</script><p><a href="javascript:alert(1)" onclick="x()">bad</a></p><img src="/cover.jpg" onerror="x()"></article>
    </body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	base, err := normalizeMediumURL("https://writer.medium.com/story-6db35d8685f0", true)
	if err != nil {
		t.Fatal(err)
	}
	article := extractMediumReader(doc, base)
	if article.Title != "Safe title" || !strings.Contains(article.ContentHTML, "Hello") {
		t.Fatalf("unexpected article: %+v", article)
	}
	for _, unsafe := range []string{"script", "javascript:", "onclick", "onerror"} {
		if strings.Contains(strings.ToLower(article.ContentHTML), unsafe) {
			t.Fatalf("unsafe HTML %q remained: %s", unsafe, article.ContentHTML)
		}
	}
	if !strings.Contains(article.ContentHTML, `src="https://writer.medium.com/cover.jpg"`) {
		t.Fatalf("image URL was not resolved: %s", article.ContentHTML)
	}
}

func TestAuthorFreeLinkMatchesOnlySameStory(t *testing.T) {
	base, _ := normalizeMediumURL("https://medium.com/@author/story-6db35d8685f0", true)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(`<a href="/other-111111111111?sk=no">other</a><a href="https://author.medium.com/story-6db35d8685f0?sk=yes">Read free</a>`))
	free := authorFreeLink(doc, base)
	if free == nil || free.Query().Get("sk") != "yes" {
		t.Fatalf("free link = %v", free)
	}
}
