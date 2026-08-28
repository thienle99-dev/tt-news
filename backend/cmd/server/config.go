package main

import (
	"os"
	"strconv"
	"time"
)

type config struct {
	Port, DBPath, BotToken, MiniAppURL, AdminToken string
	GoldRatesURL                                   string
	AIURL, AIKey, AIModel                          string
	RSSInterval, AuthMaxAge                        time.Duration
	FeaturedBriefInterval, FeaturedBriefWindow     time.Duration
	ContentCleanupInterval                         time.Duration
	RSSContentUserAgent                            string
	RSSFetchWorkers                                int
	ContentCleanupLimit                            int
	DevAuth                                        bool
	AIBackgroundScanning                           bool
	AITranslateEnabled                             bool
	AITranslateLanguage                            string
	RSSTranslateVietnamese                         bool
	DevUserID                                      int64
}

func loadConfig() config {
	return config{
		Port:                   env("PORT", "8080"),
		DBPath:                 env("DATABASE_PATH", "/data/news.db"),
		BotToken:               os.Getenv("TELEGRAM_BOT_TOKEN"),
		MiniAppURL:             os.Getenv("MINI_APP_URL"),
		AdminToken:             os.Getenv("ADMIN_TOKEN"),
		GoldRatesURL:           env("GOLD_RATES_URL", goldRatesURL),
		AIURL:                  os.Getenv("AI_URL"),
		AIKey:                  os.Getenv("AI_KEY"),
		AIModel:                env("AI_MODEL", "gpt-4o-mini"),
		AIBackgroundScanning:   env("AI_BACKGROUND_SCANNING", "false") == "true",
		AITranslateEnabled:     env("AI_TRANSLATE_ENABLED", env("AI_BACKGROUND_SCANNING", "false")) == "true",
		AITranslateLanguage:    env("AI_TRANSLATE_LANGUAGE", "Vietnamese"),
		RSSTranslateVietnamese: env("RSS_TRANSLATE_VI", "true") == "true",
		RSSInterval:            duration("RSS_FETCH_INTERVAL", 10*time.Minute),
		FeaturedBriefInterval:  duration("FEATURED_BRIEF_INTERVAL", 6*time.Hour),
		FeaturedBriefWindow:    duration("FEATURED_BRIEF_WINDOW", 24*time.Hour),
		ContentCleanupInterval: duration("CONTENT_CLEANUP_INTERVAL", 24*time.Hour),
		ContentCleanupLimit:    positiveIntEnv("CONTENT_CLEANUP_LIMIT", 100),
		RSSContentUserAgent:    env("RSS_CONTENT_USER_AGENT", "TelegramNewsRSSBot/1.0"),
		RSSFetchWorkers:        positiveIntEnv("RSS_FETCH_WORKERS", 8),
		AuthMaxAge:             duration("TELEGRAM_AUTH_MAX_AGE", 24*time.Hour),
		DevAuth:                env("DEV_AUTH", "false") == "true",
		DevUserID:              intEnv("DEV_USER_ID", 999001),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func duration(key string, fallback time.Duration) time.Duration {
	if value, err := time.ParseDuration(env(key, "")); err == nil && value > 0 {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int64) int64 {
	if value, err := strconv.ParseInt(env(key, ""), 10, 64); err == nil {
		return value
	}
	return fallback
}

func positiveIntEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err == nil && value > 0 {
		return value
	}
	return fallback
}
