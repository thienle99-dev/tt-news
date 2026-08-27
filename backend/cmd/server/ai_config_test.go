package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSavedAIConfigOverridesEnvAndCanReset(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	envConfig := config{AIURL: "https://env.example", AIKey: "env-key", AIModel: "env-model"}
	settings, source, err := loadAISettings(db, envConfig)
	if err != nil {
		t.Fatal(err)
	}
	if source != "env" || settings.Model != "env-model" {
		t.Fatalf("unexpected env settings: source=%q settings=%#v", source, settings)
	}
	if _, err = db.Exec("INSERT INTO ai_config(id,base_url,api_key,model) VALUES(1,'https://saved.example','saved-key','saved-model')"); err != nil {
		t.Fatal(err)
	}
	settings, source, err = loadAISettings(db, envConfig)
	if err != nil {
		t.Fatal(err)
	}
	if source != "saved" || settings.BaseURL != "https://saved.example" || settings.APIKey != "saved-key" || settings.Model != "saved-model" {
		t.Fatalf("unexpected saved settings: source=%q settings=%#v", source, settings)
	}
}

func TestAIConfigRoutesRequireAdminAndNeverReturnAPIKey(t *testing.T) {
	db, err := openDB(filepath.Join(t.TempDir(), "news.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db); err != nil {
		t.Fatal(err)
	}
	cfg := config{AdminToken: "admin-secret", AIURL: "https://env.example", AIKey: "env-key", AIModel: "env-model"}
	ai, err := newAIRuntimeConfig(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := &server{db: db, cfg: cfg, ai: ai}

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ai/config", nil)
	response := httptest.NewRecorder()
	s.routes().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", response.Code)
	}

	request = httptest.NewRequest(http.MethodPut, "/api/admin/ai/config", strings.NewReader(`{"base_url":"https://saved.example","api_key":"provider-secret","model":"chat-model"}`))
	request.Header.Set("X-Admin-Token", "admin-secret")
	response = httptest.NewRecorder()
	s.routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("save status = %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "provider-secret") || strings.Contains(response.Body.String(), "env-key") {
		t.Fatalf("response exposed API key: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"source":"saved"`) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
}
