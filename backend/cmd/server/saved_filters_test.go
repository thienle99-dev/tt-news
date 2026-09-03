package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestSavedFiltersCanBeCreatedListedAndDeletedByOwner(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO users(id,telegram_id,first_name) VALUES(9101,9101,'Filter owner'),(9102,9102,'Other user')`); err != nil {
		t.Fatal(err)
	}
	s := &server{db: db}
	withUser := func(request *http.Request, id int64) *http.Request { return request.WithContext(context.WithValue(request.Context(), userKey, user{ID: id})) }

	request := withUser(httptest.NewRequest(http.MethodPost, "/api/saved-filters", strings.NewReader(`{"name":"Tech sources","filter":{"source":"7,8,9","period":"7d","sort":"newest"}}`)), 9101)
	response := httptest.NewRecorder()
	s.createSavedFilter(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", response.Code, response.Body.String())
	}
	var created savedFilter
	if err = json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.Name != "Tech sources" || created.Filter.Source != "7,8,9" || created.Filter.Period != "7d" {
		t.Fatalf("unexpected created filter: %#v", created)
	}

	request = withUser(httptest.NewRequest(http.MethodGet, "/api/saved-filters", nil), 9101)
	response = httptest.NewRecorder()
	s.listSavedFilters(response, request)
	var listed []savedFilter
	if err = json.NewDecoder(response.Body).Decode(&listed); err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("unexpected saved filters: %#v", listed)
	}

	request = withUser(httptest.NewRequest(http.MethodDelete, "/api/saved-filters/"+strconv.FormatInt(created.ID, 10), nil), 9102)
	response = httptest.NewRecorder()
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", strconv.FormatInt(created.ID, 10))
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	s.deleteSavedFilter(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("non-owner delete status = %d", response.Code)
	}

	request = withUser(httptest.NewRequest(http.MethodDelete, "/api/saved-filters/"+strconv.FormatInt(created.ID, 10), nil), 9101)
	response = httptest.NewRecorder()
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	s.deleteSavedFilter(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("owner delete status = %d: %s", response.Code, response.Body.String())
	}
}
