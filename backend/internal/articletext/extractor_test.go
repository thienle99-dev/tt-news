package articletext

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestExtractHTMLPrefersArticleParagraphs(t *testing.T) {
	markup := `<html><body><nav>Navigation</nav><article><p>First paragraph.</p><p>Second paragraph.</p></article><footer>Footer</footer></body></html>`
	if got, want := ExtractHTML(markup), "First paragraph.\n\nSecond paragraph."; got != want {
		t.Fatalf("ExtractHTML() = %q, want %q", got, want)
	}
}

func TestFetchRejectsNonHTML(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.Header.Get("Accept"), "text/html") {
			t.Fatal("missing HTML accept header")
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	if _, err := (Client{HTTPClient: client}).Fetch(context.Background(), "https://example.test/article"); err == nil {
		t.Fatal("Fetch() succeeded for a non-HTML response")
	}
}

func TestFetchRejectsOversizedHTML(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader("<p>too large</p>"))}, nil
	})}
	if _, err := (Client{HTTPClient: client, MaxBytes: 8}).Fetch(context.Background(), "https://example.test/article"); err == nil {
		t.Fatal("Fetch() succeeded for an oversized response")
	}
}

func TestFetchContentKeepsArticleImages(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(*http.Request) (*http.Response, error) {
		body := `<article><p>Article body.</p><img src="/images/one.jpg"><img data-src="https://cdn.example.test/two.jpg"></article>`
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	content, err := (Client{HTTPClient: client}).FetchContent(context.Background(), "https://example.test/news/item")
	if err != nil {
		t.Fatal(err)
	}
	if content.Text != "Article body." {
		t.Fatalf("text = %q", content.Text)
	}
	if got, want := strings.Join(content.Images, ","), "https://example.test/images/one.jpg,https://cdn.example.test/two.jpg"; got != want {
		t.Fatalf("images = %q, want %q", got, want)
	}
}

func TestFetchContentUsesOpenGraphImageWhenArticleHasNoImage(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(req *http.Request) (*http.Response, error) {
		body := `<html><head><meta property="og:image" content="/images/cover.jpg"></head><body><article><p>Article text</p></article></body></html>`
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	content, err := (Client{HTTPClient: client}).FetchContent(context.Background(), "https://example.test/article")
	if err != nil {
		t.Fatalf("FetchContent() error = %v", err)
	}
	if got, want := strings.Join(content.Images, ","), "https://example.test/images/cover.jpg"; got != want {
		t.Fatalf("images = %q, want %q", got, want)
	}
}
