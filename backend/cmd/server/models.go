package main

type user struct {
	ID         int64
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	PhotoURL   string `json:"photo_url"`
}

type source struct {
	ID            int64
	Name, FeedURL string
	CategoryID    int64
	Category      string
	CountryCode   string
	CountryName   string
}

type article struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Summary     string `json:"summary"`
	URL         string `json:"url"`
	ImageURL    string `json:"image_url"`
	Source      string `json:"source"`
	SourceID    int64  `json:"source_id"`
	CountryCode string `json:"country_code"`
	CountryName string `json:"country_name"`
	Category    string `json:"category"`
	PublishedAt string `json:"published_at"`
	IsSaved     bool   `json:"is_saved"`
}

type translation struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Summary     string `json:"summary"`
}

type ctxKey string

const userKey ctxKey = "user"
