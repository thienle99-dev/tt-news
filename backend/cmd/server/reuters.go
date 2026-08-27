package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"telegram-news/internal/crawler"
)

const reutersBaseURL = "https://www.reuters.com"

var reutersArticleURL = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}/?$`)

type reutersArticle struct {
	Title, Description, Body, URL, ImageURL string
	PublishedAt                             time.Time
}

func (s *server) runReutersCron(ctx context.Context) {
	if strings.TrimSpace(s.cfg.ReutersUserAgent) == "" {
		log.Print("Reuters crawler disabled: set REUTERS_CRAWLER_USER_AGENT after receiving authorization")
		return
	}

	if err := crawler.Start(ctx, reutersJob{server: s}); err != nil {
		log.Printf("Reuters cron disabled: %v", err)
		return
	}
}

type reutersJob struct{ server *server }

func (job reutersJob) Name() string     { return "reuters" }
func (job reutersJob) Schedule() string { return job.server.cfg.ReutersCron }
func (job reutersJob) Run(ctx context.Context) {
	job.server.crawlReuters(ctx, time.Now().UTC())
}

func (s *server) crawlReuters(ctx context.Context, day time.Time) {
	sourceID, err := s.ensureReutersSource(ctx)
	if err != nil {
		log.Printf("Reuters source: %v", err)
		return
	}

	urls, err := s.reutersURLsForDay(ctx, day)
	if err != nil {
		log.Printf("Reuters sitemap: %v", err)
		return
	}
	log.Printf("Reuters crawl: %d article URL(s) found for %s", len(urls), day.Format("2006-01-02"))
	created := 0
	for index, articleURL := range urls {
		log.Printf("Reuters [%d/%d] %s", index+1, len(urls), articleURL)
		if s.articleExists(ctx, articleURL) {
			continue
		}
		article, err := s.parseReutersArticle(ctx, articleURL)
		if err != nil {
			log.Printf("Reuters article %s: %v", articleURL, err)
			continue
		}
		if article.Title == "" {
			continue
		}
		if article.PublishedAt.IsZero() {
			article.PublishedAt = time.Now().UTC()
		}
		summary := s.summarize(ctx, article.Title, article.Body)
		if summary == "" {
			summary = article.Description
		}
		result, err := s.db.ExecContext(ctx, `INSERT INTO articles(source_id,category_id,title,description,summary,url,image_url,published_at)
VALUES(?, (SELECT id FROM categories WHERE slug='world'), ?, ?, ?, ?, ?, ?) ON CONFLICT(url) DO NOTHING`,
			sourceID, article.Title, article.Body, summary, article.URL, article.ImageURL, article.PublishedAt.UTC().Format(time.RFC3339))
		if err != nil {
			log.Printf("Reuters save %s: %v", articleURL, err)
			continue
		}
		if count, _ := result.RowsAffected(); count > 0 {
			created += int(count)
		}
	}
	log.Printf("Reuters crawl finished: %d new article(s)", created)
}

func (s *server) ensureReutersSource(ctx context.Context) (int64, error) {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO sources(name,feed_url,category_id,enabled)
VALUES('Reuters', ?, (SELECT id FROM categories WHERE slug='world'), 0)`, reutersBaseURL+"/sitemap/")
	if err != nil {
		return 0, err
	}
	var id int64
	err = s.db.QueryRowContext(ctx, "SELECT id FROM sources WHERE feed_url=?", reutersBaseURL+"/sitemap/").Scan(&id)
	return id, err
}

func (s *server) articleExists(ctx context.Context, articleURL string) bool {
	var found int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM articles WHERE url=?", articleURL).Scan(&found)
	return err == nil || !errors.Is(err, sql.ErrNoRows)
}

func (s *server) reutersURLsForDay(ctx context.Context, day time.Time) ([]string, error) {
	urls := map[string]struct{}{}
	for page := 1; page <= 100; page++ {
		sitemapURL := fmt.Sprintf("%s/sitemap/%s/%02d/%d/", reutersBaseURL, day.Format("2006-01"), day.Day(), page)
		doc, err := s.reutersDocument(ctx, sitemapURL)
		if err != nil {
			return nil, err
		}
		found := 0
		doc.Find("a[href]").Each(func(_ int, link *goquery.Selection) {
			href, ok := link.Attr("href")
			if !ok || !reutersArticleURL.MatchString(href) {
				return
			}
			parsed, err := url.Parse(href)
			if err != nil || (parsed.Host != "" && parsed.Host != "www.reuters.com" && parsed.Host != "reuters.com") {
				return
			}
			absolute := reutersBaseURL + parsed.Path
			urls[absolute] = struct{}{}
			found++
		})
		if found == 0 {
			break
		}
	}
	result := make([]string, 0, len(urls))
	for articleURL := range urls {
		result = append(result, articleURL)
	}
	return result, nil
}

func (s *server) parseReutersArticle(ctx context.Context, articleURL string) (reutersArticle, error) {
	doc, err := s.reutersDocument(ctx, articleURL)
	if err != nil {
		return reutersArticle{}, err
	}
	article := reutersArticle{URL: articleURL}
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, node *goquery.Selection) bool {
		var value any
		if json.Unmarshal([]byte(node.Text()), &value) != nil {
			return true
		}
		if data, ok := findNewsArticle(value); ok {
			article.Title = jsonString(data["headline"])
			article.Description = jsonString(data["description"])
			article.Body = jsonString(data["articleBody"])
			article.ImageURL = jsonString(data["image"])
			article.PublishedAt, _ = time.Parse(time.RFC3339, jsonString(data["datePublished"]))
			return false
		}
		return true
	})
	if article.Body == "" {
		container := doc.Find("article").First()
		article.Title = firstNonEmpty(article.Title, strings.TrimSpace(container.Find("h1").First().Text()))
		paragraphs := []string{}
		container.Find("p").Each(func(_ int, p *goquery.Selection) {
			if text := strings.TrimSpace(p.Text()); text != "" {
				paragraphs = append(paragraphs, text)
			}
		})
		article.Body = strings.Join(paragraphs, "\n\n")
	}
	return article, nil
}

func (s *server) reutersDocument(ctx context.Context, pageURL string) (*goquery.Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.cfg.ReutersUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden || res.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("access denied or rate limited: HTTP %d", res.StatusCode)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	return goquery.NewDocumentFromReader(strings.NewReader(string(body)))
}

func findNewsArticle(value any) (map[string]any, bool) {
	switch node := value.(type) {
	case []any:
		for _, item := range node {
			if result, ok := findNewsArticle(item); ok {
				return result, true
			}
		}
	case map[string]any:
		if typeName, ok := node["@type"]; ok && strings.Contains(fmt.Sprint(typeName), "NewsArticle") {
			return node, true
		}
		for _, item := range node {
			if result, ok := findNewsArticle(item); ok {
				return result, true
			}
		}
	}
	return nil, false
}

func jsonString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	if image, ok := value.(map[string]any); ok {
		return jsonString(image["url"])
	}
	return ""
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
