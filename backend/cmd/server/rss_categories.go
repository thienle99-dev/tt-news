package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

type categoryDefinition struct {
	slug string
	name string
}

var rssItemCategoryDefinitions = []categoryDefinition{
	{slug: "security", name: "Bảo mật"},
	{slug: "ai", name: "Trí tuệ nhân tạo"},
	{slug: "llm", name: "Mô hình ngôn ngữ lớn"},
	{slug: "cybersecurity", name: "An ninh mạng"},
}

var rssCategoryAliases = map[string]string{
	"artificial intelligence": "ai", "machine learning": "ai", "generative ai": "ai", "large language models": "llm", "large language model": "llm",
	"cyber security": "cybersecurity", "information security": "security", "infosec": "security",
}

func rssCategoryDefinition(raw string) (categoryDefinition, bool) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return categoryDefinition{}, false
	}
	normalized := strings.ToLower(name)
	if alias, ok := rssCategoryAliases[normalized]; ok {
		normalized = alias
	}
	for _, definition := range rssItemCategoryDefinitions {
		if normalized == definition.slug {
			return definition, true
		}
	}
	var slug strings.Builder
	separator := false
	for _, character := range normalized {
		if character <= unicode.MaxASCII && (unicode.IsLetter(character) || unicode.IsDigit(character)) {
			slug.WriteRune(character)
			separator = false
		} else if slug.Len() > 0 {
			separator = true
		}
		if separator && slug.Len() > 0 && !strings.HasSuffix(slug.String(), "-") {
			slug.WriteByte('-')
		}
	}
	value := strings.Trim(slug.String(), "-")
	if value == "" {
		digest := sha256.Sum256([]byte(normalized))
		value = "rss-" + hex.EncodeToString(digest[:6])
	}
	return categoryDefinition{slug: value, name: name}, true
}

// rssItemCategories preserves every category exposed by a feed item. The
// source category remains attached as a reliable fallback for feeds with no
// usable tags.
func rssItemCategories(categories []string, fallback string) []categoryDefinition {
	primary := rssItemCategory(categories, fallback)
	definitions := []categoryDefinition{{slug: primary, name: primary}}
	seen := map[string]bool{primary: true}
	for _, raw := range categories {
		definition, ok := rssCategoryDefinition(raw)
		if !ok || seen[definition.slug] {
			continue
		}
		seen[definition.slug] = true
		definitions = append(definitions, definition)
	}
	return definitions
}

// rssItemCategory returns the first supported <category> value provided by an
// RSS item. Feeds use inconsistent casing and surrounding whitespace, so tags
// are normalized before matching. Articles without one of these tags keep the
// source's default category.
func rssItemCategory(categories []string, fallback string) string {
	for _, raw := range categories {
		slug := strings.ToLower(strings.TrimSpace(raw))
		if alias, ok := rssCategoryAliases[slug]; ok {
			slug = alias
		}
		for _, definition := range rssItemCategoryDefinitions {
			if slug == definition.slug {
				return definition.slug
			}
		}
	}
	return fallback
}
