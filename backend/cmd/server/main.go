package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/mmcdole/gofeed"
	_ "modernc.org/sqlite"
	"telegram-news/internal/articletext"
	translationservice "telegram-news/internal/translation"
)

//go:embed migrations/*.sql static/*
var embedded embed.FS

func main() {
	cfg := loadConfig()
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if health(cfg.DBPath) {
			return
		}
		os.Exit(1)
	}
	if len(os.Args) > 1 && os.Args[1] == "reuters-crawl" {
		runReutersOnce(cfg)
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "rss-fetch" {
		filter := ""
		if len(os.Args) > 2 {
			filter = os.Args[2]
		}
		runRSSOnce(cfg, filter)
		return
	}
	db, err := openDB(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		log.Fatal(err)
	}
	app := &server{db: db, cfg: cfg, feed: gofeed.NewParser()}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go app.runRSS(ctx)
	if cfg.ReutersCrawlEnabled {
		go app.runReutersCron(ctx)
	}
	if cfg.BotToken != "" && cfg.MiniAppURL != "" {
		go app.runBot(ctx)
	} else {
		log.Print("Telegram bot disabled: set TELEGRAM_BOT_TOKEN and MINI_APP_URL to enable it")
	}
	h := app.routes()
	httpServer := &http.Server{Addr: ":" + cfg.Port, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		c, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = httpServer.Shutdown(c)
	}()
	log.Printf("news app listening on :%s", cfg.Port)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func runReutersOnce(cfg config) {
	if strings.TrimSpace(cfg.ReutersUserAgent) == "" {
		log.Fatal("set REUTERS_CRAWLER_USER_AGENT before running reuters-crawl")
	}
	db, err := openDB(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		log.Fatal(err)
	}
	app := &server{db: db, cfg: cfg, feed: gofeed.NewParser()}
	app.crawlReuters(context.Background(), time.Now().UTC())
}

func runRSSOnce(cfg config, filter string) {
	db, err := openDB(cfg.DBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		log.Fatal(err)
	}
	app := &server{db: db, cfg: cfg, feed: gofeed.NewParser()}
	app.fetchSources(context.Background(), filter)
}

type server struct {
	db            *sql.DB
	cfg           config
	feed          *gofeed.Parser
	translationMu sync.Mutex
}

func (s *server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(recoverer)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { jsonOut(w, 200, map[string]string{"status": "ok"}) })
	r.Route("/api", func(r chi.Router) {
		r.Use(jsonContent)
		r.Get("/articles", s.listArticles)
		r.Get("/articles/{id}", s.getArticle)
		r.Get("/categories", s.categories)
		r.Get("/sources", s.sources)
		r.Get("/countries", s.countries)
		r.Group(func(r chi.Router) {
			r.Use(s.requireUser)
			r.Post("/articles/{id}/translations/vi", s.translateVietnamese)
			r.Get("/saved", s.saved)
			r.Post("/saved/{id}", s.save)
			r.Delete("/saved/{id}", s.unsave)
			r.Get("/me", s.me)
		})
	})
	sub, _ := fs.Sub(embedded, "static")
	r.Handle("/*", spa(sub))
	return r
}
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				jsonErr(w, 500, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func jsonContent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}
func jsonOut(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func jsonErr(w http.ResponseWriter, status int, msg string) {
	jsonOut(w, status, map[string]string{"error": msg})
}
func spa(files fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(files))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive, nosnippet")
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "." {
			p = ""
		}
		if p != "" {
			if f, e := files.Open(p); e == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

func (s *server) optionalUser(r *http.Request) (user, bool) {
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "tma ")
	if raw != "" {
		u, e := verifyInitData(raw, s.cfg.BotToken, s.cfg.AuthMaxAge)
		if e == nil {
			u, e = s.upsertUser(r.Context(), u)
			if e == nil {
				return u, true
			}
			log.Printf("user upsert: %v", e)
		}
	}
	return user{}, false
}
func (s *server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := s.optionalUser(r)
		if !ok && s.cfg.DevAuth {
			var e error
			u, e = s.upsertUser(r.Context(), user{TelegramID: s.cfg.DevUserID, FirstName: "Local developer", Username: "dev"})
			ok = e == nil
		}
		if !ok {
			jsonErr(w, 401, "valid Telegram initData is required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, u)))
	})
}
func currentUser(r *http.Request) user { u, _ := r.Context().Value(userKey).(user); return u }
func (s *server) upsertUser(ctx context.Context, u user) (user, error) {
	_, e := s.db.ExecContext(ctx, `INSERT INTO users(telegram_id,username,first_name,last_name,photo_url) VALUES(?,?,?,?,?) ON CONFLICT(telegram_id) DO UPDATE SET username=excluded.username,first_name=excluded.first_name,last_name=excluded.last_name,photo_url=excluded.photo_url,updated_at=CURRENT_TIMESTAMP`, u.TelegramID, u.Username, u.FirstName, u.LastName, u.PhotoURL)
	if e != nil {
		return u, e
	}
	e = s.db.QueryRowContext(ctx, "SELECT id FROM users WHERE telegram_id=?", u.TelegramID).Scan(&u.ID)
	return u, e
}
func (s *server) listArticles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	language := q.Get("lang")
	if language != "vi" {
		language = ""
	}
	limit := intEnvFrom(q.Get("limit"), 20)
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	u, logged := s.optionalUser(r)
	savedJoin := "0"
	args := []any{}
	if logged {
		savedJoin = "EXISTS(SELECT 1 FROM saved_articles sa WHERE sa.article_id=a.id AND sa.user_id=?)"
		args = append(args, u.ID)
	}
	translationJoin, title, description, summary := "", "a.title", "a.description", "a.summary"
	if language != "" {
		translationJoin = " LEFT JOIN article_translations tr ON tr.article_id=a.id AND tr.language_code=?"
		title, description, summary = "COALESCE(NULLIF(tr.title,''),a.title)", "a.description", "COALESCE(NULLIF(tr.summary,''),a.summary)"
		args = append(args, language)
	}
	where := []string{"1=1"}
	if c := q.Get("category"); c != "" {
		where = append(where, "c.slug=?")
		args = append(args, c)
	}
	if sourceID := q.Get("source"); sourceID != "" {
		where = append(where, "s.id=?")
		args = append(args, sourceID)
	}
	if country := q.Get("country"); country != "" {
		where = append(where, "s.country_code=?")
		args = append(args, strings.ToUpper(country))
	}
	if term := strings.TrimSpace(q.Get("q")); term != "" {
		where = append(where, fmt.Sprintf("(%s LIKE ? OR %s LIKE ? OR %s LIKE ?)", title, description, summary))
		like := "%" + term + "%"
		args = append(args, like, like, like)
	}
	args = append(args, limit)
	sqlq := fmt.Sprintf(`SELECT a.id,%s,%s,%s,a.url,a.image_url,s.name,s.id,s.country_code,s.country_name,c.slug,a.published_at,%s FROM articles a JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id%s WHERE %s ORDER BY a.published_at DESC,a.id DESC LIMIT ?`, title, description, summary, savedJoin, translationJoin, strings.Join(where, " AND "))
	rows, e := s.db.QueryContext(r.Context(), sqlq, args...)
	if e != nil {
		jsonErr(w, 500, "could not load articles")
		return
	}
	defer rows.Close()
	items := []article{}
	for rows.Next() {
		var a article
		var saved int
		if e = rows.Scan(&a.ID, &a.Title, &a.Description, &a.Summary, &a.URL, &a.ImageURL, &a.Source, &a.SourceID, &a.CountryCode, &a.CountryName, &a.Category, &a.PublishedAt, &saved); e != nil {
			jsonErr(w, 500, "could not read articles")
			return
		}
		a.IsSaved = saved == 1
		items = append(items, a)
	}
	jsonOut(w, 200, items)
}
func (s *server) getArticle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var a article
	var imageJSON string
	e := s.db.QueryRowContext(r.Context(), `SELECT a.id,a.title,a.description,a.summary,a.url,a.image_url,a.content_images,s.name,s.id,s.country_code,s.country_name,c.slug,a.published_at,0 FROM articles a JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id WHERE a.id=?`, id).Scan(&a.ID, &a.Title, &a.Description, &a.Summary, &a.URL, &a.ImageURL, &imageJSON, &a.Source, &a.SourceID, &a.CountryCode, &a.CountryName, &a.Category, &a.PublishedAt, new(int))
	if errors.Is(e, sql.ErrNoRows) {
		jsonErr(w, 404, "article not found")
		return
	}
	if e != nil {
		jsonErr(w, 500, "could not load article")
		return
	}
	a.ContentImages = decodeContentImages(imageJSON)
	jsonOut(w, 200, a)
}

func decodeContentImages(raw string) []string {
	var images []string
	if json.Unmarshal([]byte(raw), &images) != nil {
		return nil
	}
	return images
}
func (s *server) categories(w http.ResponseWriter, r *http.Request) {
	rows, e := s.db.QueryContext(r.Context(), "SELECT slug,name FROM categories ORDER BY name")
	if e != nil {
		jsonErr(w, 500, "could not load categories")
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var slug, name string
		_ = rows.Scan(&slug, &name)
		out = append(out, map[string]string{"slug": slug, "name": name})
	}
	jsonOut(w, 200, out)
}
func (s *server) sources(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,name,country_code,country_name FROM sources WHERE enabled=1 ORDER BY name`)
	if err != nil {
		jsonErr(w, 500, "could not load sources")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var name, code, country string
		if err = rows.Scan(&id, &name, &code, &country); err == nil {
			out = append(out, map[string]any{"id": id, "name": name, "country_code": code, "country_name": country})
		}
	}
	jsonOut(w, 200, out)
}
func (s *server) countries(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT DISTINCT country_code,country_name FROM sources WHERE enabled=1 ORDER BY country_name`)
	if err != nil {
		jsonErr(w, 500, "could not load countries")
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var code, name string
		if err = rows.Scan(&code, &name); err == nil {
			out = append(out, map[string]string{"code": code, "name": name})
		}
	}
	jsonOut(w, 200, out)
}
func (s *server) saved(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	language := r.URL.Query().Get("lang")
	translationJoin, title, description, summary := "", "a.title", "a.description", "a.summary"
	args := []any{u.ID}
	if language == "vi" {
		translationJoin = " LEFT JOIN article_translations tr ON tr.article_id=a.id AND tr.language_code=?"
		title, description, summary = "COALESCE(NULLIF(tr.title,''),a.title)", "a.description", "COALESCE(NULLIF(tr.summary,''),a.summary)"
		args = []any{language, u.ID}
	}
	query := fmt.Sprintf(`SELECT a.id,%s,%s,%s,a.url,a.image_url,s.name,s.id,s.country_code,s.country_name,c.slug,a.published_at,1 FROM saved_articles sa JOIN articles a ON a.id=sa.article_id JOIN sources s ON s.id=a.source_id JOIN categories c ON c.id=a.category_id%s WHERE sa.user_id=? ORDER BY sa.created_at DESC`, title, description, summary, translationJoin)
	rows, e := s.db.QueryContext(r.Context(), query, args...)
	if e != nil {
		jsonErr(w, 500, "could not load saved articles")
		return
	}
	defer rows.Close()
	out := []article{}
	for rows.Next() {
		var a article
		var x int
		if e = rows.Scan(&a.ID, &a.Title, &a.Description, &a.Summary, &a.URL, &a.ImageURL, &a.Source, &a.SourceID, &a.CountryCode, &a.CountryName, &a.Category, &a.PublishedAt, &x); e != nil {
			jsonErr(w, 500, "could not read saved articles")
			return
		}
		a.IsSaved = true
		out = append(out, a)
	}
	jsonOut(w, 200, out)
}
func (s *server) save(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	id := chi.URLParam(r, "id")
	res, e := s.db.ExecContext(r.Context(), "INSERT OR IGNORE INTO saved_articles(user_id,article_id) SELECT ?,id FROM articles WHERE id=?", u.ID, id)
	if e != nil {
		jsonErr(w, 500, "could not save article")
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var exists int
		e = s.db.QueryRowContext(r.Context(), "SELECT 1 FROM articles WHERE id=?", id).Scan(&exists)
		if errors.Is(e, sql.ErrNoRows) {
			jsonErr(w, 404, "article not found")
			return
		}
	}
	jsonOut(w, 200, map[string]bool{"saved": true})
}
func (s *server) unsave(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	_, e := s.db.ExecContext(r.Context(), "DELETE FROM saved_articles WHERE user_id=? AND article_id=?", u.ID, chi.URLParam(r, "id"))
	if e != nil {
		jsonErr(w, 500, "could not remove saved article")
		return
	}
	jsonOut(w, 200, map[string]bool{"saved": false})
}
func (s *server) me(w http.ResponseWriter, r *http.Request) { jsonOut(w, 200, currentUser(r)) }

func verifyInitData(raw, token string, maxAge time.Duration) (user, error) {
	if token == "" {
		return user{}, errors.New("bot token missing")
	}
	v, e := url.ParseQuery(raw)
	if e != nil {
		return user{}, e
	}
	hash := v.Get("hash")
	authDate := v.Get("auth_date")
	userJSON := v.Get("user")
	if hash == "" || authDate == "" || userJSON == "" {
		return user{}, errors.New("missing initData fields")
	}
	v.Del("hash")
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+v.Get(k))
	}
	secretMac := hmac.New(sha256.New, []byte("WebAppData"))
	secretMac.Write([]byte(token))
	mac := hmac.New(sha256.New, secretMac.Sum(nil))
	mac.Write([]byte(strings.Join(parts, "\n")))
	expected := mac.Sum(nil)
	provided, e := hex.DecodeString(hash)
	if e != nil || !hmac.Equal(expected, provided) {
		return user{}, errors.New("invalid initData signature")
	}
	ts, e := strconv.ParseInt(authDate, 10, 64)
	if e != nil || time.Since(time.Unix(ts, 0)) > maxAge {
		return user{}, errors.New("expired initData")
	}
	var payload struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		PhotoURL  string `json:"photo_url"`
	}
	if e = json.Unmarshal([]byte(userJSON), &payload); e != nil || payload.ID == 0 {
		return user{}, errors.New("invalid Telegram user")
	}
	return user{TelegramID: payload.ID, Username: payload.Username, FirstName: payload.FirstName, LastName: payload.LastName, PhotoURL: payload.PhotoURL}, nil
}
func intEnvFrom(v string, d int) int {
	n, e := strconv.Atoi(v)
	if e != nil {
		return d
	}
	return n
}

func (s *server) runRSS(ctx context.Context) {
	s.fetchSources(ctx, "")
	t := time.NewTicker(s.cfg.RSSInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.fetchSources(ctx, "")
		}
	}
}
func (s *server) fetchSources(ctx context.Context, nameFilter string) {
	query := `SELECT s.id,s.name,s.feed_url,s.category_id,c.slug FROM sources s JOIN categories c ON c.id=s.category_id WHERE s.enabled=1`
	args := []any{}
	if strings.TrimSpace(nameFilter) != "" {
		query += " AND lower(s.name) LIKE ?"
		args = append(args, "%"+strings.ToLower(strings.TrimSpace(nameFilter))+"%")
	}
	rows, e := s.db.QueryContext(ctx, query, args...)
	if e != nil {
		log.Printf("rss sources: %v", e)
		return
	}
	sources := []source{}
	for rows.Next() {
		var src source
		if e = rows.Scan(&src.ID, &src.Name, &src.FeedURL, &src.CategoryID, &src.Category); e != nil {
			log.Printf("rss source row: %v", e)
			continue
		}
		sources = append(sources, src)
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		log.Printf("rss sources: %v", e)
		return
	}
	rows.Close()

	log.Printf("rss crawl started (filter=%q)", nameFilter)
	processed := 0
	inserted := 0
	for _, src := range sources {
		processed++
		log.Printf("rss [%d] fetching %s", processed, src.Name)
		created, err := s.fetchSource(ctx, src)
		if err != nil {
			e = err
			log.Printf("rss %s: %v", src.Name, e)
			continue
		}
		inserted += created
		log.Printf("rss [%d] %s: %d new article(s)", processed, src.Name, created)
	}
	log.Printf("rss crawl finished: %d source(s), %d new article(s)", processed, inserted)
}
func (s *server) fetchSource(ctx context.Context, src source) (int, error) {
	feed, e := s.feed.ParseURLWithContext(src.FeedURL, ctx)
	if e != nil {
		return 0, e
	}
	inserted := 0
	for _, item := range feed.Items {
		link := normalizeURL(item.Link)
		title := strings.TrimSpace(item.Title)
		if link == "" || title == "" {
			continue
		}
		var exists int
		e = s.db.QueryRowContext(ctx, "SELECT 1 FROM articles WHERE url=?", link).Scan(&exists)
		if e == nil {
			continue
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return inserted, e
		}
		published := time.Now().UTC()
		if item.PublishedParsed != nil {
			published = *item.PublishedParsed
		} else if item.UpdatedParsed != nil {
			published = *item.UpdatedParsed
		}
		image := ""
		if item.Image != nil {
			image = item.Image.URL
		}
		if image == "" && len(item.Enclosures) > 0 {
			image = item.Enclosures[0].URL
		}
		body := articletext.PlainText(item.Description)
		if body == "" {
			body = articletext.PlainText(item.Content)
		}
		if extracted, fetchErr := (articletext.Client{UserAgent: s.cfg.RSSContentUserAgent}).Fetch(ctx, link); fetchErr != nil {
			log.Printf("rss article %s: %v", link, fetchErr)
		} else if extracted != "" {
			body = extracted
		}
		summary := ""
		if brief, summaryErr := (translationservice.Client{URL: s.cfg.AIURL, APIKey: s.cfg.AIKey, Model: s.cfg.AIModel}).Summarize(ctx, title, body); summaryErr != nil {
			log.Printf("rss summary %s: %v", link, summaryErr)
		} else {
			title, summary = brief.Title, brief.Summary
		}
		result, e := s.db.ExecContext(ctx, `INSERT INTO articles(source_id,category_id,title,description,summary,url,image_url,content_images,published_at) VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(url) DO NOTHING`, src.ID, src.CategoryID, title, "", summary, link, image, "[]", published.UTC().Format(time.RFC3339))
		if e != nil {
			return inserted, e
		}
		if count, _ := result.RowsAffected(); count > 0 {
			inserted += int(count)
		}
	}
	return inserted, nil
}
func normalizeURL(raw string) string {
	u, e := url.Parse(strings.TrimSpace(raw))
	if e != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.Fragment = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	return u.String()
}

func (s *server) runBot(ctx context.Context) {
	offset := 0
	client := &http.Client{Timeout: 40 * time.Second}
	for ctx.Err() == nil {
		var out struct {
			OK     bool `json:"ok"`
			Result []struct {
				UpdateID int `json:"update_id"`
				Message  *struct {
					Chat struct {
						ID int64 `json:"id"`
					} `json:"chat"`
					Text string `json:"text"`
				} `json:"message"`
			} `json:"result"`
		}
		u := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?timeout=25&offset=%d", s.cfg.BotToken, offset)
		req, e := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if e != nil {
			return
		}
		res, e := client.Do(req)
		if e != nil {
			if ctx.Err() == nil {
				log.Printf("bot poll: %v", e)
				time.Sleep(2 * time.Second)
			}
			continue
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if e = json.Unmarshal(body, &out); e != nil || !out.OK {
			log.Printf("bot response: %v", e)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, up := range out.Result {
			offset = up.UpdateID + 1
			if up.Message != nil && strings.HasPrefix(up.Message.Text, "/start") {
				s.sendWebApp(ctx, client, up.Message.Chat.ID)
			}
		}
	}
}
func (s *server) sendWebApp(ctx context.Context, c *http.Client, chatID int64) {
	payload := map[string]any{"chat_id": chatID, "text": "Open your personal news feed:", "reply_markup": map[string]any{"inline_keyboard": [][]any{{map[string]any{"text": "Open News", "web_app": map[string]string{"url": s.cfg.MiniAppURL}}}}}}
	b, _ := json.Marshal(payload)
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegram.org/bot"+s.cfg.BotToken+"/sendMessage", strings.NewReader(string(b)))
	if e != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if _, e = c.Do(req); e != nil {
		log.Printf("bot send: %v", e)
	}
}
