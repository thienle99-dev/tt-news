export type ArticleCategory = { slug: string; name: string }
export type Article = {
  id: number; title: string; description: string; original_content?: string; summary: string; url: string; image_url: string; content_images: string[]
  source: string; source_id: number; country_code: string; country_name: string; category: string; categories: ArticleCategory[]; published_at: string; is_saved: boolean; is_read: boolean; folder_ids: number[]; tag_ids: number[]
}
export type Category = { slug: string; name: string }
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
