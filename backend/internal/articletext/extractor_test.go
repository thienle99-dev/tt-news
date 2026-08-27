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
