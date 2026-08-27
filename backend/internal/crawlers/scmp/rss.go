// Package scmp owns SCMP-specific feed definitions.
package scmp

import "telegram-news/internal/crawlers/rss"

// Feeds contains the broad SCMP feed. Add category-specific official feeds
// here when needed; article URL deduplication happens in the shared RSS worker.
var Feeds = []rss.Source{
	{
		Name:     "SCMP All News",
		URL:      "https://www.scmp.com/rss/feed",
		Category: "world",
		CountryCode: "HK",
		CountryName: "Hong Kong",
	},
}
