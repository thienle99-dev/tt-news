package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const savedFiltersLimit = 20

type savedFilterValues struct {
	Category string `json:"category"`
	Source   string `json:"source"`
	Country  string `json:"country"`
	Query    string `json:"query"`
	Period   string `json:"period"`
	Sort     string `json:"sort"`
}

type savedFilter struct {
	ID        int64             `json:"id"`
	Name      string            `json:"name"`
	Filter    savedFilterValues `json:"filter"`
	CreatedAt string            `json:"created_at"`
}

func (s *server) listSavedFilters(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(), `SELECT id,name,filter_json,created_at FROM saved_filters WHERE user_id=? ORDER BY updated_at DESC,id DESC`, currentUser(r).ID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not load saved filters")
		return
	}
	defer rows.Close()
	filters := []savedFilter{}
	for rows.Next() {
		var item savedFilter
		var raw string
		if err = rows.Scan(&item.ID, &item.Name, &raw, &item.CreatedAt); err != nil {
			jsonErr(w, http.StatusInternalServerError, "could not read saved filters")
			return
		}
		if json.Unmarshal([]byte(raw), &item.Filter) != nil {
			item.Filter = savedFilterValues{}
		}
		filters = append(filters, item)
	}
	if err = rows.Err(); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not read saved filters")
		return
	}
	jsonOut(w, http.StatusOK, filters)
}

func (s *server) createSavedFilter(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name   string            `json:"name"`
		Filter savedFilterValues `json:"filter"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&input); err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid saved filter")
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		jsonErr(w, http.StatusBadRequest, "saved filter name must be between 1 and 80 characters")
		return
	}
	input.Filter = normalizeSavedFilter(input.Filter)
	raw, err := json.Marshal(input.Filter)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid saved filter")
		return
	}
	userID := currentUser(r).ID
	var count int
	if err = s.db.QueryRowContext(r.Context(), `SELECT count(*) FROM saved_filters WHERE user_id=?`, userID).Scan(&count); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not save filter")
		return
	}
	if count >= savedFiltersLimit {
		jsonErr(w, http.StatusConflict, "you can save up to 20 filters")
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := s.db.ExecContext(r.Context(), `INSERT INTO saved_filters(user_id,name,filter_json,created_at,updated_at) VALUES(?,?,?,?,?)`, userID, input.Name, string(raw), now, now)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not save filter")
		return
	}
	id, _ := result.LastInsertId()
	jsonOut(w, http.StatusCreated, savedFilter{ID: id, Name: input.Name, Filter: input.Filter, CreatedAt: now})
}

func (s *server) deleteSavedFilter(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		jsonErr(w, http.StatusBadRequest, "invalid saved filter id")
		return
	}
	result, err := s.db.ExecContext(r.Context(), `DELETE FROM saved_filters WHERE id=? AND user_id=?`, id, currentUser(r).ID)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not delete saved filter")
		return
	}
	deleted, _ := result.RowsAffected()
	if deleted == 0 {
		jsonErr(w, http.StatusNotFound, "saved filter not found")
		return
	}
	jsonOut(w, http.StatusOK, map[string]bool{"deleted": true})
}

func normalizeSavedFilter(filter savedFilterValues) savedFilterValues {
	filter.Category = strings.TrimSpace(filter.Category)
	filter.Source = strings.Join(strings.FieldsFunc(filter.Source, func(r rune) bool { return r == ',' || r == ' ' }), ",")
	filter.Country = strings.ToUpper(strings.TrimSpace(filter.Country))
	filter.Query = strings.TrimSpace(filter.Query)
	if filter.Period != "24h" && filter.Period != "7d" {
		filter.Period = ""
	}
	if filter.Sort != "newest" && filter.Sort != "oldest" && filter.Sort != "relevant" && filter.Sort != "worth_read" {
		filter.Sort = "newest"
	}
	return filter
}
