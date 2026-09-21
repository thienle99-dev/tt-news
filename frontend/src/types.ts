export type ArticleCategory = { slug: string; name: string }
export type Article = {
  id: number; title: string; description: string; original_content?: string; summary: string; url: string; image_url: string; content_images: string[]
  source: string; source_id: number; country_code: string; country_name: string; category: string; categories: ArticleCategory[]; published_at: string; is_saved: boolean; is_read: boolean; is_hidden: boolean; folder_ids: number[]; tag_ids: number[]
  thread_author?: string; thread_display_name?: string; thread_avatar_url?: string; thread_likes?: number; thread_replies?: number; thread_reposts?: number
}
export type ThreadsTarget = { id: number; kind: 'profile' | 'keyword'; query: string; enabled: boolean; last_fetch_at: string; last_success_at: string; last_error: string; last_inserted: number }
export type ThreadsAuthorModeration = { username: string; blocked: boolean; post_count: number; profile_target_id?: number; profile_enabled: boolean }
export type ThreadComment = { id: string; author: string; display_name?: string; avatar_url?: string; body: string; published_at: string; likes: number }
export type Category = { id: number; slug: string; name: string }
export type Source = { id: number; name: string; country_code: string; country_name: string }
export type Country = { code: string; name: string }
export type Translation = Pick<Article, 'title' | 'description' | 'summary'>
export type FeaturedTopic = { id: number; position: number; title: string; summary: string; why_it_matters: string; articles: Article[] }
export type FeaturedBrief = { id: number; generated_at: string; window_start: string; window_end: string; title: string; intro: string; takeaways: string[]; topics: FeaturedTopic[] }
export type TelegramUser = { telegram_id: number; username: string; first_name: string; last_name: string; photo_url: string }
export type GoldRate = { code: string; name: string; buy_price: number; sell_price: number; unit: string; trend: string; trend_value: string; last_updated: string }
export type SavedCollection = { id: number; name: string; count: number }
export type SavedOrganization = { folders: SavedCollection[]; tags: SavedCollection[] }
export type AIModel = { id: string; owned_by?: string }
export type AIConfig = { base_url: string; model: string; api_key_configured: boolean; source: 'env' | 'saved' }
export type AIConfigInput = { base_url: string; api_key: string; model: string }
export type DailyDigestPreferences = { enabled: boolean; category_ids: number[]; source_ids: number[] }
export type SavedFilterValues = { category: string; source: string; country: string; query: string; period: string; sort: string }
export type SavedFilter = { id: number; name: string; filter: SavedFilterValues; created_at: string }
export type ArticleWatch = { id: number; kind: 'article' | 'topic'; article_id?: number; category_id?: number; title: string; category_slug?: string; category_name?: string; keyword?: string; enabled: boolean; created_at: string }
export type MediumReaderArticle = {
  requested_url: string; canonical_url: string; resolved_url: string; title: string; subtitle?: string; author?: string
  published_at?: string; reading_time_minutes?: number; access: 'public' | 'author_free_link' | 'preview'; warning?: string
  content_html: string; cached_at: string
}
