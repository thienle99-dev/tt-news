// Package articletext fetches and extracts readable text from public article pages.
package articletext

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const defaultMaxBytes int64 = 2 << 20

type Client struct {
	HTTPClient *http.Client
	UserAgent  string
	MaxBytes   int64
}

func (c Client) Fetch(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
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
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("article page returned %s", res.Status)
	}
	if !strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "html") {
		return "", errors.New("article page is not HTML")
	}
	maxBytes := c.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBytes+1))
	if err != nil {
		return "", err
	}
	if int64(len(body)) > maxBytes {
		return "", fmt.Errorf("article page exceeds %d byte limit", maxBytes)
	}
	return ExtractHTML(string(body)), nil
}

func ExtractHTML(markup string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(markup))
	if err != nil {
		return ""
	}
	for _, selector := range []string{"article", "[itemprop='articleBody']", "main", ".article-body", ".article__body", ".story-body", ".entry-content", ".post-content"} {
		if text := selectionText(doc.Find(selector).First()); text != "" {
			return text
		}
	}
	return selectionText(doc.Find("body").First())
}

func PlainText(markup string) string {
	if !strings.Contains(markup, "<") {
		return clean(markup)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(markup))
	if err != nil {
		return clean(markup)
	}
	return clean(doc.Text())
}

func selectionText(selection *goquery.Selection) string {
	if selection.Length() == 0 {
		return ""
	}
	paragraphs := make([]string, 0)
	selection.Find("p").Each(func(_ int, paragraph *goquery.Selection) {
		if text := clean(paragraph.Text()); text != "" {
			paragraphs = append(paragraphs, text)
		}
	})
	if len(paragraphs) > 0 {
		return strings.Join(paragraphs, "\n\n")
	}
	return clean(selection.Text())
}

func clean(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
