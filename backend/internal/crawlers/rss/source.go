// Package rss contains the common contract for all RSS-based sites.
package rss

// Source is a declarative RSS feed. Site packages only need to provide these
// fields; the shared worker handles HTTP fetching, parsing, deduplication and
// persistence.
type Source struct {
	Name        string
	URL         string
	Category    string
	CountryCode string
	CountryName string
}
