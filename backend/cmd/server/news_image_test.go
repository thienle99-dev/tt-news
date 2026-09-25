package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type newsImageRoundTripper func(*http.Request) (*http.Response, error)

func (roundTrip newsImageRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}

func TestAllowedNewsImageURL(t *testing.T) {
	for _, test := range []struct {
		name string
		url  string
		want bool
	}{
		{name: "https publisher image", url: "https://cdn.example.org/news/photo.webp", want: true},
		{name: "http publisher image", url: "http://images.example.org/photo.jpg", want: true},
		{name: "credentials", url: "https://user:pass@cdn.example.org/photo.jpg"},
		{name: "local IPv4", url: "http://127.0.0.1/photo.jpg"},
		{name: "private IPv4", url: "http://10.0.0.4/photo.jpg"},
		{name: "local IPv6", url: "http://[::1]/photo.jpg"},
		{name: "unsupported scheme", url: "file:///etc/passwd"},
		{name: "unsupported port", url: "https://cdn.example.org:8443/photo.jpg"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := allowedNewsImageURL(test.url); got != test.want {
				t.Fatalf("allowedNewsImageURL(%q) = %t, want %t", test.url, got, test.want)
			}
		})
	}
}

func TestIsPublicNewsImageIP(t *testing.T) {
	for _, test := range []struct {
		ip   string
		want bool
	}{
		{ip: "1.1.1.1", want: true},
		{ip: "2606:4700:4700::1111", want: true},
		{ip: "10.1.2.3"},
		{ip: "127.0.0.1"},
		{ip: "169.254.10.20"},
		{ip: "100.64.0.1"},
		{ip: "192.0.2.1"},
		{ip: "240.0.0.1"},
		{ip: "::ffff:127.0.0.1"},
		{ip: "fc00::1"},
		{ip: "2001:db8::1"},
	} {
		t.Run(test.ip, func(t *testing.T) {
			if got := isPublicNewsImageIP(net.ParseIP(test.ip)); got != test.want {
				t.Fatalf("isPublicNewsImageIP(%q) = %t, want %t", test.ip, got, test.want)
			}
		})
	}
}

func TestProxyNewsImageWithClient(t *testing.T) {
	for _, test := range []struct {
		name          string
		status        int
		contentType   string
		contentLength int64
		body          string
		wantStatus    int
		wantContent   string
	}{
		{name: "image success", status: http.StatusOK, contentType: "image/png", contentLength: -1, body: "image-bytes", wantStatus: http.StatusOK, wantContent: "image-bytes"},
		{name: "reject non-image", status: http.StatusOK, contentType: "text/html", contentLength: -1, body: "not an image", wantStatus: http.StatusBadGateway},
		{name: "reject oversized content length", status: http.StatusOK, contentType: "image/jpeg", contentLength: maxNewsImageBytes + 1, wantStatus: http.StatusBadGateway},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: newsImageRoundTripper(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode:    test.status,
					Header:        http.Header{"Content-Type": []string{test.contentType}},
					ContentLength: test.contentLength,
					Body:          io.NopCloser(strings.NewReader(test.body)),
				}, nil
			})}
			request := httptest.NewRequest(http.MethodGet, "/api/articles/42/image", nil)
			response := httptest.NewRecorder()
			proxyNewsImageWithClient(response, request, "https://cdn.example.org/photo.jpg", true, client)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantStatus == http.StatusBadGateway && !strings.Contains(response.Body.String(), "article image source is unavailable") {
				t.Fatalf("error body = %q, want article image source unavailable", response.Body.String())
			}
			if test.wantStatus == http.StatusOK && response.Body.String() != test.wantContent {
				t.Fatalf("body = %q, want %q", response.Body.String(), test.wantContent)
			}
			if test.wantStatus == http.StatusOK {
				if response.Header().Get("Content-Type") != test.contentType {
					t.Fatalf("Content-Type = %q, want %q", response.Header().Get("Content-Type"), test.contentType)
				}
				if response.Header().Get("X-Content-Type-Options") != "nosniff" {
					t.Fatalf("X-Content-Type-Options = %q, want nosniff", response.Header().Get("X-Content-Type-Options"))
				}
			}
		})
	}
}

func TestWrapArticleMedia(t *testing.T) {
	newsArticle := article{ID: 42, ImageURL: "https://cdn.example.org/photo.jpg"}
	(&server{}).wrapArticleMedia(&newsArticle)
	if newsArticle.ImageURL != "/api/articles/42/image" {
		t.Fatalf("News image URL = %q, want same-origin proxy path", newsArticle.ImageURL)
	}

	threadPost := article{ID: 43, ImageURL: "https://fbcdn.net/photo.jpg", ThreadAvatarURL: "https://fbcdn.net/avatar.jpg", ThreadPostID: "post-43"}
	(&server{}).wrapArticleMedia(&threadPost)
	if threadPost.ImageURL != "/api/threads/posts/43/image" || threadPost.ThreadAvatarURL != "/api/threads/posts/43/avatar" {
		t.Fatalf("Threads media URLs changed unexpectedly: image=%q avatar=%q", threadPost.ImageURL, threadPost.ThreadAvatarURL)
	}
}
