import type { Article, Category, Country, FeaturedBrief, GoldRate, Source, TelegramUser, Translation } from './types'

const initData = window.Telegram?.WebApp?.initData ?? ''
const headers = (): HeadersInit => initData ? { Authorization: `tma ${initData}` } : {}

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(url, { ...options, headers: { ...headers(), ...options.headers } })
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: 'Request failed' }))
    throw new Error(body.error ?? 'Request failed')
  }
  return response.json() as Promise<T>
}

export const api = {
  articles: (params: URLSearchParams) => request<Article[]>(`/api/articles?${params}`),
  article: (id: number) => request<Article>(`/api/articles/${id}`),
	translateVietnamese: (id: number) => request<Translation>(`/api/articles/${id}/translations/vi`, { method: 'POST' }),
  resummarize: (id: number) => request<Pick<Article, 'title' | 'summary'>>(`/api/articles/${id}/resummarize`, { method: 'POST' }),
  categories: () => request<Category[]>('/api/categories'),
  sources: () => request<Source[]>('/api/sources'),
  countries: () => request<Country[]>('/api/countries'),
  goldRates: () => request<GoldRate[]>('/api/gold-rates'),
  featured: (language: string) => request<FeaturedBrief | null>(`/api/featured?lang=${encodeURIComponent(language)}`),
  saved: (language: string) => request<Article[]>(`/api/saved?lang=${encodeURIComponent(language)}`),
  toggleSaved: (article: Article) => request<{ saved: boolean }>(`/api/saved/${article.id}`, { method: article.is_saved ? 'DELETE' : 'POST' }),
  me: () => request<TelegramUser>('/api/me'),
}
