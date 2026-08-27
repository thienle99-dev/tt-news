package main

import (
	"os"
	"strconv"
	"time"
)

type config struct {
	Port, DBPath, BotToken, MiniAppURL string
	AIURL, AIKey, AIModel              string
	ReutersCron, ReutersUserAgent      string
	RSSInterval, AuthMaxAge            time.Duration
	RSSContentUserAgent                string
	DevAuth                            bool
	ReutersCrawlEnabled                bool
	DevUserID                          int64
}

func loadConfig() config {
	return config{
		Port:                env("PORT", "8080"),
		DBPath:              env("DATABASE_PATH", "/data/news.db"),
		BotToken:            os.Getenv("TELEGRAM_BOT_TOKEN"),
		MiniAppURL:          os.Getenv("MINI_APP_URL"),
		AIURL:               os.Getenv("AI_URL"),
		AIKey:               os.Getenv("AI_KEY"),
		AIModel:             env("AI_MODEL", "gpt-4o-mini"),
		ReutersCrawlEnabled: env("REUTERS_CRAWL_ENABLED", "false") == "true",
		ReutersCron:         env("REUTERS_CRON", "15 */2 * * *"),
		ReutersUserAgent:    os.Getenv("REUTERS_CRAWLER_USER_AGENT"),
		RSSInterval:         duration("RSS_FETCH_INTERVAL", 10*time.Minute),
		RSSContentUserAgent: env("RSS_CONTENT_USER_AGENT", "TelegramNewsRSSBot/1.0"),
		AuthMaxAge:          duration("TELEGRAM_AUTH_MAX_AGE", 24*time.Hour),
		DevAuth:             env("DEV_AUTH", "false") == "true",
		DevUserID:           intEnv("DEV_USER_ID", 999001),
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
