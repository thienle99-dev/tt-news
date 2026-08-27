package articletext

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractHTMLPrefersArticleParagraphs(t *testing.T) {
	markup := `<html><body><nav>Navigation</nav><article><p>First paragraph.</p><p>Second paragraph.</p></article><footer>Footer</footer></body></html>`
	if got, want := ExtractHTML(markup), "First paragraph.\n\nSecond paragraph."; got != want {
		t.Fatalf("ExtractHTML() = %q, want %q", got, want)
	}
}

func TestFetchRejectsNonHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept"), "text/html") {
			t.Fatal("missing HTML accept header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	if _, err := (Client{}).Fetch(context.Background(), server.URL); err == nil {
		t.Fatal("Fetch() succeeded for a non-HTML response")
	}
}
