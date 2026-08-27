package articletext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Content struct {
	Text   string
	Images []string
}

// FetchContent returns the article text and image URLs found in the content area.
func (c Client) FetchContent(ctx context.Context, pageURL string) (Content, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return Content{}, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return Content{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusForbidden && is9to5Google(req.URL) {
		return c.fetch9to5GooglePost(ctx, req.URL)
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return Content{}, fmt.Errorf("article page returned %s", res.Status)
	}
	if !strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "html") {
		return Content{}, errors.New("article page is not HTML")
	}
	maxBytes := c.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return Content{}, err
	}
	if int64(len(body)) > maxBytes {
		return Content{}, fmt.Errorf("article page exceeds %d byte limit", maxBytes)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return Content{}, err
	}
	root := richRoot(doc)
	base := req.URL
	if res.Request != nil && res.Request.URL != nil {
		base = res.Request.URL
	}
	images := contentImages(root, base)
	if len(images) == 0 {
		images = metadataImages(doc, base)
	}
	return Content{Text: selectionText(root), Images: images}, nil
}

func is9to5Google(pageURL *url.URL) bool {
	host := strings.ToLower(pageURL.Hostname())
	return host == "9to5google.com" || host == "www.9to5google.com"
}

func (c Client) fetch9to5GooglePost(ctx context.Context, pageURL *url.URL) (Content, error) {
	slug := path.Base(strings.Trim(pageURL.Path, "/"))
	if slug == "" || slug == "." || slug == "/" {
		return Content{}, errors.New("9to5google article slug is missing")
	}
	apiURL := &url.URL{Scheme: pageURL.Scheme, Host: pageURL.Host, Path: "/wp-json/wp/v2/posts"}
	query := apiURL.Query()
	query.Set("slug", slug)
	query.Set("_fields", "content")
	apiURL.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL.String(), nil)
	if err != nil {
		return Content{}, err
	}
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return Content{}, err
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return Content{}, fmt.Errorf("9to5google API returned %s", res.Status)
	}
	maxBytes := c.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return Content{}, err
	}
	if int64(len(body)) > maxBytes {
		return Content{}, fmt.Errorf("9to5google API response exceeds %d byte limit", maxBytes)
	}
	var posts []struct {
		Content struct {
			Rendered string `json:"rendered"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &posts); err != nil {
		return Content{}, fmt.Errorf("decode 9to5google API response: %w", err)
	}
	if len(posts) == 0 || strings.TrimSpace(posts[0].Content.Rendered) == "" {
		return Content{}, errors.New("9to5google API article was not found")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(posts[0].Content.Rendered))
	if err != nil {
		return Content{}, err
	}
	root := richRoot(doc)
	return Content{Text: selectionText(root), Images: contentImages(root, pageURL)}, nil
}

func richRoot(doc *goquery.Document) *goquery.Selection {
	for _, selector := range []string{"article", "[itemprop='articleBody']", ".article-body", ".article__body", ".story-body", ".entry-content", ".post-content", "main"} {
		root := doc.Find(selector).First()
		if selectionText(root) != "" {
			return root
		}
	}
	return doc.Find("body").First()
}

func contentImages(selection *goquery.Selection, base *url.URL) []string {
	seen, images := map[string]bool{}, make([]string, 0)
	selection.Find("img").Each(func(_ int, image *goquery.Selection) {
		raw, ok := image.Attr("src")
		if !ok || strings.TrimSpace(raw) == "" {
			raw, _ = image.Attr("data-src")
		}
		u, err := base.Parse(strings.TrimSpace(raw))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || seen[u.String()] || len(images) == 12 {
			return
		}
		seen[u.String()] = true
		images = append(images, u.String())
	})
	return images
}

func metadataImages(doc *goquery.Document, base *url.URL) []string {
	seen, images := map[string]bool{}, make([]string, 0, 1)
	doc.Find("meta[property='og:image'], meta[name='twitter:image']").Each(func(_ int, meta *goquery.Selection) {
		raw, _ := meta.Attr("content")
		u, err := base.Parse(strings.TrimSpace(raw))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || seen[u.String()] {
			return
		}
		seen[u.String()] = true
		images = append(images, u.String())
	})
	return images
}
