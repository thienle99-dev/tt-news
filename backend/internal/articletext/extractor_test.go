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

func TestExtractHTMLIgnoresSSRAndNonContentNodes(t *testing.T) {
	markup := `<html><head><style>.payload{display:none}</style></head><body><script>{"require":[["CometSSR"]]}</script><noscript>hydration fallback</noscript><p>Readable article text.</p></body></html>`
	if got, want := ExtractHTML(markup), "Readable article text."; got != want {
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

func TestFetchContentFallsBackTo9to5GoogleAPIOnForbiddenPage(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/2026/08/25/example-post/" {
			return &http.Response{StatusCode: http.StatusForbidden, Status: "403 Forbidden", Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader("forbidden"))}, nil
		}
		if req.URL.Path != "/wp-json/wp/v2/posts" || req.URL.Query().Get("slug") != "example-post" {
			t.Fatalf("unexpected fallback URL: %s", req.URL)
		}
		body := `[{"content":{"rendered":"<article><p>Full article from the public API.</p><img src=\"/cover.jpg\"></article>"}}]`
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	content, err := (Client{HTTPClient: client}).FetchContent(context.Background(), "https://9to5google.com/2026/08/25/example-post/")
	if err != nil {
		t.Fatalf("FetchContent() error = %v", err)
	}
	if got, want := content.Text, "Full article from the public API."; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	if got, want := strings.Join(content.Images, ","), "https://9to5google.com/cover.jpg"; got != want {
		t.Fatalf("images = %q, want %q", got, want)
	}
}

func TestFetchContentFallsBackTo9to5MacAPIWhenPageIsOnlyAShell(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/2026/08/25/example-post/" {
			return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader(`<main><p>Loading article…</p></main>`))}, nil
		}
		if req.URL.Path != "/wp-json/wp/v2/posts" || req.URL.Query().Get("slug") != "example-post" {
			t.Fatalf("unexpected fallback URL: %s", req.URL)
		}
		body := `[{"content":{"rendered":"<article><p>Full public article from the WordPress API.</p></article>"}}]`
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	content, err := (Client{HTTPClient: client}).FetchContent(context.Background(), "https://9to5mac.com/2026/08/25/example-post/")
	if err != nil {
		t.Fatalf("FetchContent() error = %v", err)
	}
	if got, want := content.Text, "Full public article from the WordPress API."; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}

func TestFetchContentFallsBackToReaderForForbiddenGizmochinaPage(t *testing.T) {
	client := &http.Client{Transport: roundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "www.gizmochina.com" {
			return &http.Response{StatusCode: http.StatusForbidden, Status: "403 Forbidden", Header: http.Header{"Content-Type": []string{"text/html"}}, Body: io.NopCloser(strings.NewReader("challenge"))}, nil
		}
		if req.URL.Host != "r.jina.ai" || req.URL.Path != "/http://www.gizmochina.com/2026/08/27/example/" {
			t.Fatalf("unexpected fallback URL: %s", req.URL)
		}
		body := `Title: Example

URL Source: https://www.gizmochina.com/2026/08/27/example/

Markdown Content:
Full product specifications include a 500 ml capacity and 316L stainless steel liner.

![Product image](https://cdn.example.test/product.jpg)`
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"text/plain"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	content, err := (Client{HTTPClient: client}).FetchContent(context.Background(), "https://www.gizmochina.com/2026/08/27/example/")
	if err != nil {
		t.Fatalf("FetchContent() error = %v", err)
	}
	if !strings.HasPrefix(content.Text, "Full product specifications") || strings.Contains(content.Text, "URL Source:") {
		t.Fatalf("text = %q", content.Text)
	}
	if got, want := strings.Join(content.Images, ","), "https://cdn.example.test/product.jpg"; got != want {
		t.Fatalf("images = %q, want %q", got, want)
	}
}
