export type Article = {
  id: number; title: string; description: string; summary: string; url: string; image_url: string; content_images: string[]
  source: string; source_id: number; country_code: string; country_name: string; category: string; published_at: string; is_saved: boolean
}
export type Category = { slug: string; name: string }
export type Source = { id: number; name: string; country_code: string; country_name: string }
export type Country = { code: string; name: string }
export type Translation = Pick<Article, 'title' | 'description' | 'summary'>
export type FeaturedTopic = { id: number; position: number; title: string; summary: string; articles: Article[] }
export type FeaturedBrief = { id: number; generated_at: string; window_start: string; window_end: string; title: string; intro: string; topics: FeaturedTopic[] }
export type TelegramUser = { telegram_id: number; username: string; first_name: string; last_name: string; photo_url: string }
