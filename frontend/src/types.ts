export type Article = {
  id: number; title: string; description: string; summary: string; url: string; image_url: string
  source: string; category: string; published_at: string; is_saved: boolean
}
export type Category = { slug: string; name: string }
export type TelegramUser = { telegram_id: number; username: string; first_name: string; last_name: string; photo_url: string }
