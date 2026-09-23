package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"telegram-news/internal/articletext"
)

const (
	minimumArticleContentLength    = 300
	articleContentExtractorVersion = "2026-08-28.2"
)

type articleContentTask struct {
	ID  int64
	URL string
}

type articleContentResult struct {
	articleContentTask
	content articletext.Content
	err     error
}

// fetchMissingArticleContent fills article bodies after the RSS crawl has
// completed. Keeping it separate means a slow or blocked publisher page never
// delays the RSS feed itself.
func (s *server) fetchMissingArticleContent(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.url FROM articles a
		LEFT JOIN article_content_fetches f ON f.article_id=a.id
		WHERE length(trim(a.full_content))<? AND a.url<>''
		AND a.thread_post_id=''
		AND f.article_id IS NULL
		ORDER BY a.published_at DESC,a.id DESC LIMIT ?`, minimumArticleContentLength, s.cfg.RSSContentFetchLimit)
	if err != nil {
		log.Printf("article content worker: load queue: %v", err)
		return
	}
	tasks := make([]articleContentTask, 0, s.cfg.RSSContentFetchLimit)
	for rows.Next() {
		var task articleContentTask
		if err = rows.Scan(&task.ID, &task.URL); err != nil {
			rows.Close()
			log.Printf("article content worker: scan queue: %v", err)
			return
		}
		tasks = append(tasks, task)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		log.Printf("article content worker: read queue: %v", err)
		return
	}
	rows.Close()
	if len(tasks) == 0 {
		return
	}

	job, jobCtx, err := s.startJob(ctx, "article_content", "Lấy nội dung bài viết", "scheduled", len(tasks))
	if err != nil {
		log.Printf("article content worker skipped: %v", err)
		return
	}
	started := time.Now()
	completed, failed := int64(0), int64(0)
	defer func() {
		detail := fmt.Sprintf("%d/%d bài đầy đủ · %d lỗi · %s", completed, len(tasks), failed, time.Since(started).Round(time.Second))
		s.finishJob(context.Background(), job, "completed", detail, completed, failed)
	}()

	workers := min(s.cfg.RSSContentFetchWorkers, len(tasks))
	input := make(chan articleContentTask)
	output := make(chan articleContentResult, len(tasks))
	var group sync.WaitGroup
	client := articletext.Client{UserAgent: s.cfg.RSSContentUserAgent}
	var hostMu sync.Mutex
	hostSlots := map[string]chan struct{}{}
	acquireHost := func(ctx context.Context, rawURL string) (func(), error) {
		parsed, parseErr := url.Parse(rawURL)
		if parseErr != nil || parsed.Hostname() == "" {
			return func() {}, nil
		}
		host := strings.ToLower(parsed.Hostname())
		hostMu.Lock()
		slots := hostSlots[host]
		if slots == nil {
			slots = make(chan struct{}, 2)
			hostSlots[host] = slots
		}
		hostMu.Unlock()
		select {
		case slots <- struct{}{}:
			return func() { <-slots }, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	for range workers {
		group.Add(1)
		go func() {
			defer group.Done()
			for task := range input {
				release, waitErr := acquireHost(jobCtx, task.URL)
				if waitErr != nil {
					output <- articleContentResult{articleContentTask: task, err: waitErr}
					continue
				}
				content, fetchErr := client.FetchContent(jobCtx, task.URL)
				release()
				if fetchErr == nil && len(content.Text) < minimumArticleContentLength {
					fetchErr = fmt.Errorf("extracted content is too short (%d characters)", len(content.Text))
				}
				output <- articleContentResult{articleContentTask: task, content: content, err: fetchErr}
			}
		}()
	}
	go func() {
		defer close(input)
		for _, task := range tasks {
			select {
			case input <- task:
			case <-jobCtx.Done():
				return
			}
		}
	}()
	go func() {
		group.Wait()
		close(output)
	}()

	processed := 0
	for result := range output {
		processed++
		if result.err != nil {
			failed++
			s.recordArticleContentFetch(jobCtx, result.ID, result.err.Error())
			log.Printf("article content %s: %v", result.URL, result.err)
		} else if err = s.saveArticleContent(jobCtx, result); err != nil {
			failed++
			s.recordArticleContentFetch(jobCtx, result.ID, err.Error())
			log.Printf("article content save %s: %v", result.URL, err)
		} else {
			completed++
			_, _ = s.db.ExecContext(jobCtx, `DELETE FROM article_content_fetches WHERE article_id=?`, result.ID)
		}
		s.updateJob(jobCtx, job, "Đang tải nội dung", result.URL, int64(processed), failed)
	}
}

func (s *server) saveArticleContent(ctx context.Context, result articleContentResult) error {
	images := "[]"
	if len(result.content.Images) > 0 {
		encoded, err := json.Marshal(result.content.Images)
		if err != nil {
			return err
		}
		images = string(encoded)
	}
	image := ""
	if len(result.content.Images) > 0 {
		image = result.content.Images[0]
	}
	_, err := s.db.ExecContext(ctx, `UPDATE articles SET full_content=?,
		content_images=CASE WHEN ?<>'[]' THEN ? ELSE content_images END,
		image_url=CASE WHEN image_url='' AND ?<>'' THEN ? ELSE image_url END
		WHERE id=?`, result.content.Text, images, images, image, image, result.ID)
	return err
}

func (s *server) recordArticleContentFetch(ctx context.Context, articleID int64, detail string) {
	_, _ = s.db.ExecContext(ctx, `INSERT INTO article_content_fetches(article_id,attempts,attempted_at,last_error,extractor_version)
		VALUES(?,1,?,?,?) ON CONFLICT(article_id) DO UPDATE SET attempts=article_content_fetches.attempts+1,attempted_at=excluded.attempted_at,last_error=excluded.last_error,extractor_version=excluded.extractor_version`, articleID, time.Now().UTC().Format(time.RFC3339), detail, articleContentExtractorVersion)
}
