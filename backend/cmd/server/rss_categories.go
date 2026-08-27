package main

import "strings"

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

// rssItemCategory returns the first supported <category> value provided by an
// RSS item. Feeds use inconsistent casing and surrounding whitespace, so tags
// are normalized before matching. Articles without one of these tags keep the
// source's default category.
func rssItemCategory(categories []string, fallback string) string {
	for _, raw := range categories {
		slug := strings.ToLower(strings.TrimSpace(raw))
		for _, definition := range rssItemCategoryDefinitions {
			if slug == definition.slug {
				return definition.slug
			}
		}
	}
	return fallback
}
