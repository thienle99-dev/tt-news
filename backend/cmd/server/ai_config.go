package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	translationservice "telegram-news/internal/translation"
)

type aiSettings struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"-"`
	Model   string `json:"model"`
}

type aiRuntimeConfig struct {
	mu       sync.RWMutex
	settings aiSettings
	source   string
}

func aiSettingsFromConfig(cfg config) aiSettings {
	return aiSettings{BaseURL: cfg.AIURL, APIKey: cfg.AIKey, Model: cfg.AIModel}
}

func loadAISettings(db *sql.DB, cfg config) (aiSettings, string, error) {
	settings := aiSettingsFromConfig(cfg)
	var stored aiSettings
	err := db.QueryRow("SELECT base_url,api_key,model FROM ai_config WHERE id=1").Scan(&stored.BaseURL, &stored.APIKey, &stored.Model)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, "env", nil
	}
	if err != nil {
		return aiSettings{}, "", err
	}
	return stored, "saved", nil
}

func newAIRuntimeConfig(db *sql.DB, cfg config) (*aiRuntimeConfig, error) {
	settings, source, err := loadAISettings(db, cfg)
	if err != nil {
		return nil, err
	}
	return &aiRuntimeConfig{settings: settings, source: source}, nil
}

func (s *server) currentAISettings() (aiSettings, string) {
	if s.ai == nil {
		return aiSettingsFromConfig(s.cfg), "env"
	}
	s.ai.mu.RLock()
	defer s.ai.mu.RUnlock()
	return s.ai.settings, s.ai.source
}

func (s *server) aiClient() translationservice.Client {
	settings, _ := s.currentAISettings()
	return translationservice.Client{URL: settings.BaseURL, APIKey: settings.APIKey, Model: settings.Model}
}

func (s *server) aiConfigured() bool {
	settings, _ := s.currentAISettings()
	return settings.BaseURL != "" && settings.APIKey != "" && settings.Model != ""
}

func (s *server) updateAISettings(settings aiSettings, source string) {
	if s.ai == nil {
		s.ai = &aiRuntimeConfig{}
	}
	s.ai.mu.Lock()
	s.ai.settings = settings
	s.ai.source = source
	s.ai.mu.Unlock()
}

type aiConfigRequest struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

func decodeAIConfigRequest(r *http.Request) (aiConfigRequest, error) {
	var input aiConfigRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
	err := decoder.Decode(&input)
	input.BaseURL = strings.TrimSpace(input.BaseURL)
	input.APIKey = strings.TrimSpace(input.APIKey)
	input.Model = strings.TrimSpace(input.Model)
	return input, err
}

func (s *server) effectiveAISettings(input aiConfigRequest) (aiSettings, error) {
	current, _ := s.currentAISettings()
	if input.BaseURL == "" {
		input.BaseURL = current.BaseURL
	}
	if input.APIKey == "" {
		input.APIKey = current.APIKey
	}
	if input.Model == "" {
		input.Model = current.Model
	}
	baseURL, err := translationservice.NormalizeBaseURL(input.BaseURL)
	if err != nil {
		return aiSettings{}, err
	}
	if input.APIKey == "" {
		return aiSettings{}, errors.New("AI API key is required")
	}
	return aiSettings{BaseURL: baseURL, APIKey: input.APIKey, Model: input.Model}, nil
}

func (s *server) adminAIConfig(w http.ResponseWriter, _ *http.Request) {
	settings, source := s.currentAISettings()
	jsonOut(w, http.StatusOK, map[string]any{
		"base_url":           settings.BaseURL,
		"model":              settings.Model,
		"api_key_configured": settings.APIKey != "",
		"source":             source,
	})
}

func (s *server) adminAIModels(w http.ResponseWriter, r *http.Request) {
	input, err := decodeAIConfigRequest(r)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	settings, err := s.effectiveAISettings(input)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	models, err := (translationservice.Client{URL: settings.BaseURL, APIKey: settings.APIKey}).Models(r.Context())
	if err != nil {
		jsonErr(w, http.StatusBadGateway, err.Error())
		return
	}
	jsonOut(w, http.StatusOK, models)
}

func (s *server) adminTestAI(w http.ResponseWriter, r *http.Request) {
	input, err := decodeAIConfigRequest(r)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	settings, err := s.effectiveAISettings(input)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if settings.Model == "" {
		jsonErr(w, http.StatusBadRequest, "AI model is required")
		return
	}
	reply, err := (translationservice.Client{URL: settings.BaseURL, APIKey: settings.APIKey, Model: settings.Model}).Test(r.Context())
	if err != nil {
		jsonErr(w, http.StatusBadGateway, err.Error())
		return
	}
	jsonOut(w, http.StatusOK, map[string]any{"ok": true, "reply": reply})
}

func (s *server) adminSaveAIConfig(w http.ResponseWriter, r *http.Request) {
	input, err := decodeAIConfigRequest(r)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	settings, err := s.effectiveAISettings(input)
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if settings.Model == "" {
		jsonErr(w, http.StatusBadRequest, "AI model is required")
		return
	}
	_, err = s.db.ExecContext(r.Context(), `INSERT INTO ai_config(id,base_url,api_key,model,updated_at) VALUES(1,?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET base_url=excluded.base_url,api_key=excluded.api_key,model=excluded.model,updated_at=CURRENT_TIMESTAMP`, settings.BaseURL, settings.APIKey, settings.Model)
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not save AI configuration")
		return
	}
	s.updateAISettings(settings, "saved")
	s.adminAIConfig(w, r)
}

func (s *server) adminResetAIConfig(w http.ResponseWriter, r *http.Request) {
	if _, err := s.db.ExecContext(r.Context(), "DELETE FROM ai_config WHERE id=1"); err != nil {
		jsonErr(w, http.StatusInternalServerError, "could not reset AI configuration")
		return
	}
	s.updateAISettings(aiSettingsFromConfig(s.cfg), "env")
	s.adminAIConfig(w, r)
}

func loadPersistedAIConfig(ctx context.Context, db *sql.DB, cfg config) (config, error) {
	settings, _, err := loadAISettings(db, cfg)
	if err != nil {
		return cfg, err
	}
	select {
	case <-ctx.Done():
		return cfg, ctx.Err()
	default:
	}
	cfg.AIURL, cfg.AIKey, cfg.AIModel = settings.BaseURL, settings.APIKey, settings.Model
	return cfg, nil
}
