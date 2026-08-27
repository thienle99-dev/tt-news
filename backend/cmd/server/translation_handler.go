package main

import (
	"database/sql"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"telegram-news/internal/articletext"
	translationservice "telegram-news/internal/translation"
)

func (s *server) translateVietnamese(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s.translationMu.Lock()
	defer s.translationMu.Unlock()

	var translated translation
	err := s.db.QueryRowContext(r.Context(), `SELECT title,description,summary FROM article_translations WHERE article_id=? AND language_code='vi'`, id).Scan(&translated.Title, &translated.Description, &translated.Summary)
	if err == nil {
		jsonOut(w, http.StatusOK, translated)
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusInternalServerError, "could not load translation")
		return
	}

	var article article
	err = s.db.QueryRowContext(r.Context(), `SELECT id,title,description,summary FROM articles WHERE id=?`, id).Scan(&article.ID, &article.Title, &article.Description, &article.Summary)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load article")
		return
	}
	fields, err := (translationservice.Client{URL: s.cfg.AIURL, APIKey: s.cfg.AIKey, Model: s.cfg.AIModel}).Vietnamese(r.Context(), translationservice.Fields{Title: article.Title, Summary: article.Summary})
	if err != nil {
		log.Printf("translate article %d: %v", article.ID, err)
		jsonErr(w, http.StatusServiceUnavailable, "could not translate article")
		return
	}
	translated = translationResult(fields)
	if _, err = s.db.ExecContext(r.Context(), `INSERT INTO article_translations(article_id,language_code,title,description,summary) VALUES(?,'vi',?,?,?)`, article.ID, translated.Title, translated.Description, translated.Summary); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not save translation")
		return
	}
	jsonOut(w, http.StatusOK, translated)
}

func translationResult(fields translationservice.Fields) translation {
	return translation{Title: fields.Title, Description: fields.Description, Summary: fields.Summary}
}

func (s *server) resummarizeArticle(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s.translationMu.Lock()
	defer s.translationMu.Unlock()

	var article article
	err := s.db.QueryRowContext(r.Context(), `SELECT id,title,description,full_content,url FROM articles WHERE id=?`, id).Scan(&article.ID, &article.Title, &article.Description, &article.FullContent, &article.URL)
	if errors.Is(err, sql.ErrNoRows) {
		jsonErr(w, http.StatusNotFound, "article not found")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load article")
		return
	}

	body := article.FullContent
	extracted, fetchErr := (articletext.Client{UserAgent: s.cfg.RSSContentUserAgent}).FetchContent(r.Context(), article.URL)
	if fetchErr != nil {
		log.Printf("resummarize article %d: could not refresh full content: %v", article.ID, fetchErr)
	} else if len(extracted.Text) > len(body) {
		body = extracted.Text
	}
	if len(body) < 300 {
		jsonErr(w, http.StatusConflict, "full article content is unavailable; crawl the source again")
		return
	}
	log.Printf("resummarize article id=%d: sending AI request (content_chars=%d model=%q)", article.ID, len(body), s.cfg.AIModel)
	brief, err := (translationservice.Client{URL: s.cfg.AIURL, APIKey: s.cfg.AIKey, Model: s.cfg.AIModel}).Summarize(r.Context(), article.Title, body)
	if err != nil {
		log.Printf("resummarize article id=%d: AI request failed: %v", article.ID, err)
		jsonErr(w, http.StatusServiceUnavailable, "could not resummarize article")
		return
	}
	if _, err = s.db.ExecContext(r.Context(), `UPDATE articles SET title=?,full_content=?,summary=? WHERE id=?`, brief.Title, body, brief.Summary, article.ID); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not save summary")
		return
	}
	if _, err = s.db.ExecContext(r.Context(), `DELETE FROM article_translations WHERE article_id=? AND language_code='vi'`, article.ID); err != nil {
		log.Printf("resummarize article %d translation cleanup: %v", article.ID, err)
	}
	jsonOut(w, http.StatusOK, translationResult(brief))
}
