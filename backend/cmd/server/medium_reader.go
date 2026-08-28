package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const mediumReaderMaxBytes int64 = 2 << 20

type mediumReaderArticle struct {
	RequestedURL string `json:"requested_url"`
	CanonicalURL string `json:"canonical_url"`
	ResolvedURL  string `json:"resolved_url"`
	Title        string `json:"title"`
	Subtitle     string `json:"subtitle,omitempty"`
	Author       string `json:"author,omitempty"`
	PublishedAt  string `json:"published_at,omitempty"`
	ReadingTime  int    `json:"reading_time_minutes,omitempty"`
	Access       string `json:"access"`
	Warning      string `json:"warning,omitempty"`
	ContentHTML  string `json:"content_html"`
	CachedAt     string `json:"cached_at"`
}

type mediumReaderRequest struct {
	URL string `json:"url"`
}

func (s *server) mediumReader(w http.ResponseWriter, r *http.Request) {
	var input mediumReaderRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 16<<10)).Decode(&input); err != nil {
		jsonErr(w, http.StatusBadRequest, "a Medium URL is required")
		return
	}
	requested, err := normalizeMediumURL(input.URL, true)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if cached, ok := s.cachedMediumReader(r.Context(), requested.String()); ok {
		jsonOut(w, http.StatusOK, cached)
		return
	}
	article, _, err := fetchMediumReader(r.Context(), requested)
	if err != nil {
		jsonErr(w, http.StatusBadGateway, err.Error())
		return
	}
	article.RequestedURL = requested.String()
	article.CachedAt = time.Now().UTC().Format(time.RFC3339)
	s.cacheMediumReader(r.Context(), requested.String(), article)
	jsonOut(w, http.StatusOK, article)
}

func (s *server) cachedMediumReader(ctx context.Context, key string) (mediumReaderArticle, bool) {
	var payload, expires string
	if err := s.db.QueryRowContext(ctx, `SELECT payload,expires_at FROM medium_reader_articles WHERE cache_key=?`, key).Scan(&payload, &expires); err != nil {
		return mediumReaderArticle{}, false
	}
	expiresAt, err := time.Parse(time.RFC3339, expires)
	if err != nil || !expiresAt.After(time.Now()) {
		return mediumReaderArticle{}, false
	}
	var article mediumReaderArticle
	if json.Unmarshal([]byte(payload), &article) != nil {
		return mediumReaderArticle{}, false
	}
	return article, true
}

func (s *server) cacheMediumReader(ctx context.Context, key string, article mediumReaderArticle) {
	payload, err := json.Marshal(article)
	if err != nil {
		return
	}
	_, _ = s.db.ExecContext(ctx, `INSERT INTO medium_reader_articles(cache_key,payload,expires_at) VALUES(?,?,?) ON CONFLICT(cache_key) DO UPDATE SET payload=excluded.payload,expires_at=excluded.expires_at,created_at=CURRENT_TIMESTAMP`, key, string(payload), time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339))
}

func normalizeMediumURL(raw string, allowSK bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || !isMediumHost(u.Hostname()) {
		return nil, errors.New("enter a valid https://medium.com article URL")
	}
	if strings.Trim(u.Path, "/") == "" {
		return nil, errors.New("the Medium article path is missing")
	}
	u.Fragment = ""
	query := url.Values{}
	if allowSK && u.Query().Get("sk") != "" {
		query.Set("sk", u.Query().Get("sk"))
	}
	u.RawQuery = query.Encode()
	return u, nil
}

func isMediumHost(host string) bool {
	host = strings.ToLower(host)
	return host == "medium.com" || strings.HasSuffix(host, ".medium.com")
}

func fetchMediumReader(ctx context.Context, requested *url.URL) (mediumReaderArticle, string, error) {
	initial, doc, finalURL, err := fetchMediumDocument(ctx, requested.String())
	if err != nil {
		return mediumReaderArticle{}, "", err
	}
	article := extractMediumReader(doc, finalURL)
	article.CanonicalURL = initial
	article.ResolvedURL = finalURL.String()
	if article.Title == "" || article.ContentHTML == "" {
		return mediumReaderArticle{}, "", errors.New("Medium did not return readable public article content")
	}
	cacheKey := initial
	if freeURL := authorFreeLink(doc, finalURL); freeURL != nil {
		_, freeDoc, resolvedFreeURL, fetchErr := fetchMediumDocument(ctx, freeURL.String())
		if fetchErr == nil {
			freeArticle := extractMediumReader(freeDoc, resolvedFreeURL)
			if freeArticle.Title != "" && freeArticle.ContentHTML != "" {
				article = freeArticle
				article.CanonicalURL = initial
				article.ResolvedURL = resolvedFreeURL.String()
				article.Access = "author_free_link"
				cacheKey = freeURL.String()
			}
		}
	}
	if article.Access == "" {
		article.Access = "public"
	}
	if article.Access != "author_free_link" && strings.Contains(strings.ToLower(doc.Text()), "member-only story") {
		article.Access = "preview"
		article.Warning = "Medium returned only the public preview for this member-only story."
	}
	return article, cacheKey, nil
}

func fetchMediumDocument(ctx context.Context, raw string) (string, *goquery.Document, *url.URL, error) {
	start, err := normalizeMediumURL(raw, true)
	if err != nil {
		return "", nil, nil, err
	}
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if _, err := normalizeMediumURL(req.URL.String(), true); err != nil {
			return errors.New("Medium redirected outside the supported site")
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, start.String(), nil)
	if err != nil {
		return "", nil, nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("User-Agent", "SignalBriefReader/1.0 (+public-content-reader)")
	res, err := client.Do(req)
	if err != nil {
		return "", nil, nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return "", nil, nil, fmt.Errorf("Medium returned %s", res.Status)
	}
	if !strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "html") {
		return "", nil, nil, errors.New("Medium did not return an HTML page")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, mediumReaderMaxBytes+1))
	if err != nil {
		return "", nil, nil, err
	}
	if int64(len(body)) > mediumReaderMaxBytes {
		return "", nil, nil, errors.New("Medium article is too large")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return "", nil, nil, err
	}
	finalURL := start
	if res.Request != nil && res.Request.URL != nil {
		finalURL = res.Request.URL
	}
	return normalizedMediumCanonical(finalURL), doc, finalURL, nil
}

func normalizedMediumCanonical(u *url.URL) string {
	copy := *u
	copy.RawQuery = ""
	copy.Fragment = ""
	return copy.String()
}

func authorFreeLink(doc *goquery.Document, base *url.URL) *url.URL {
	storyID := mediumStoryID(base.Path)
	if storyID == "" {
		return nil
	}
	var result *url.URL
	doc.Find("a[href]").EachWithBreak(func(_ int, link *goquery.Selection) bool {
		href, _ := link.Attr("href")
		candidate, err := base.Parse(href)
		if err != nil || !isMediumHost(candidate.Hostname()) || candidate.Scheme != "https" || candidate.Query().Get("sk") == "" || mediumStoryID(candidate.Path) != storyID {
			return true
		}
		result = candidate
		return false
	})
	return result
}

var storyIDPattern = regexp.MustCompile(`(?i)-([a-f0-9]{12})(?:/)?$`)
var readingTimePattern = regexp.MustCompile(`(?i)(\d+)\s+min\s+read`)

func mediumStoryID(path string) string {
	match := storyIDPattern.FindStringSubmatch(path)
	if len(match) != 2 {
		return ""
	}
	return strings.ToLower(match[1])
}

func extractMediumReader(doc *goquery.Document, base *url.URL) mediumReaderArticle {
	articleRoot := doc.Find("article").First()
	if articleRoot.Length() == 0 {
		articleRoot = doc.Find("main").First()
	}
	clone := articleRoot.Clone()
	clone.Find("script,style,nav,footer,form,button,svg,iframe,object,embed,template,link,meta,input,textarea,select").Remove()
	clone.Find("h1").First().Remove()
	sanitizeReaderHTML(clone, base)
	content, _ := clone.Html()
	content = strings.TrimSpace(content)
	title := metaValue(doc, "property", "og:title")
	if title == "" {
		title = strings.TrimSpace(articleRoot.Find("h1").First().Text())
	}
	if title == "" {
		title = strings.TrimSpace(doc.Find("title").First().Text())
	}
	reading := 0
	if match := readingTimePattern.FindStringSubmatch(doc.Text()); len(match) == 2 {
		reading, _ = strconv.Atoi(match[1])
	}
	return mediumReaderArticle{
		Title: title, Subtitle: metaValue(doc, "name", "description"), Author: metaValue(doc, "name", "author"),
		PublishedAt: metaValue(doc, "property", "article:published_time"), ReadingTime: reading, ContentHTML: content,
	}
}

func metaValue(doc *goquery.Document, attribute, value string) string {
	selector := fmt.Sprintf("meta[%s=%q]", attribute, value)
	content, _ := doc.Find(selector).First().Attr("content")
	return strings.TrimSpace(content)
}

func sanitizeReaderHTML(root *goquery.Selection, base *url.URL) {
	root.Find("*").Each(func(_ int, node *goquery.Selection) {
		tag := goquery.NodeName(node)
		if tag == "a" || tag == "img" {
			attribute := "href"
			if tag == "img" {
				attribute = "src"
			}
			raw, _ := node.Attr(attribute)
			u, err := base.Parse(strings.TrimSpace(raw))
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				node.RemoveAttr(attribute)
			} else {
				node.SetAttr(attribute, u.String())
			}
			if tag == "a" {
				node.SetAttr("target", "_blank")
				node.SetAttr("rel", "noopener noreferrer")
			}
		}
		for _, attr := range node.Get(0).Attr {
			if attr.Key != "href" && attr.Key != "src" && attr.Key != "alt" && attr.Key != "title" && attr.Key != "target" && attr.Key != "rel" {
				node.RemoveAttr(attr.Key)
			}
		}
	})
}
