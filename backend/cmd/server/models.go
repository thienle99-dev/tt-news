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
	ID            int64    `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	FullContent   string   `json:"original_content,omitempty"`
	Summary       string   `json:"summary"`
	URL           string   `json:"url"`
	ImageURL      string   `json:"image_url"`
	ContentImages []string `json:"content_images"`
	Source        string   `json:"source"`
	SourceID      int64    `json:"source_id"`
	CountryCode   string   `json:"country_code"`
	CountryName   string   `json:"country_name"`
	Category      string   `json:"category"`
	Categories    []string `json:"categories"`
	PublishedAt   string   `json:"published_at"`
	IsSaved       bool     `json:"is_saved"`
	IsRead        bool     `json:"is_read"`
	FolderIDs     []int64  `json:"folder_ids,omitempty"`
	TagIDs        []int64  `json:"tag_ids,omitempty"`
}

type savedCollection struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type savedOrganization struct {
	Folders []savedCollection `json:"folders"`
	Tags    []savedCollection `json:"tags"`
}

type translation struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Summary     string `json:"summary"`
}

type featuredTopic struct {
	ID       int64     `json:"id"`
	Position int       `json:"position"`
	Title    string    `json:"title"`
	Summary  string    `json:"summary"`
	Articles []article `json:"articles"`
}

type featuredBrief struct {
	ID          int64           `json:"id"`
	GeneratedAt string          `json:"generated_at"`
	WindowStart string          `json:"window_start"`
	WindowEnd   string          `json:"window_end"`
	Title       string          `json:"title"`
	Intro       string          `json:"intro"`
	Topics      []featuredTopic `json:"topics"`
}

type ctxKey string

const userKey ctxKey = "user"
