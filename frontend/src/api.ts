import type { Article, Category, Country, FeaturedBrief, GoldRate, SavedCollection, SavedOrganization, Source, TelegramUser, Translation } from './types'

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
  submitAIFeedback: (id: number, issueType: 'incorrect' | 'missing', reason: string) => request<{ id: number }>(`/api/articles/${id}/ai-feedback`, { method: 'POST', body: JSON.stringify({ issue_type: issueType, reason }), headers: { 'Content-Type': 'application/json' } }),
  categories: () => request<Category[]>('/api/categories'),
  sources: () => request<Source[]>('/api/sources'),
  countries: () => request<Country[]>('/api/countries'),
  goldRates: () => request<GoldRate[]>('/api/gold-rates'),
  featured: (language: string) => request<FeaturedBrief | null>(`/api/featured?lang=${encodeURIComponent(language)}`),
  saved: (params: URLSearchParams) => request<Article[]>(`/api/saved?${params}`),
  toggleSaved: (article: Article) => request<{ saved: boolean }>(`/api/saved/${article.id}`, { method: article.is_saved ? 'DELETE' : 'POST' }),
  bulkUnsave: (articleIDs: number[]) => request<{ removed: number }>('/api/saved', { method: 'DELETE', body: JSON.stringify({ article_ids: articleIDs }), headers: { 'Content-Type': 'application/json' } }),
  savedOrganization: () => request<SavedOrganization>('/api/saved/organization'),
  createSavedFolder: (name: string) => request<SavedCollection>('/api/saved/folders', { method: 'POST', body: JSON.stringify({ name }), headers: { 'Content-Type': 'application/json' } }),
  createSavedTag: (name: string) => request<SavedCollection>('/api/saved/tags', { method: 'POST', body: JSON.stringify({ name }), headers: { 'Content-Type': 'application/json' } }),
  setSavedFolders: (articleID: number, ids: number[]) => request<{ ids: number[] }>(`/api/saved/${articleID}/folders`, { method: 'PUT', body: JSON.stringify({ ids }), headers: { 'Content-Type': 'application/json' } }),
  setSavedTags: (articleID: number, ids: number[]) => request<{ ids: number[] }>(`/api/saved/${articleID}/tags`, { method: 'PUT', body: JSON.stringify({ ids }), headers: { 'Content-Type': 'application/json' } }),
  startReading: (id: number) => request<{ status: string; is_read: boolean }>(`/api/articles/${id}/reading`, { method: 'POST' }),
  markRead: (id: number) => request<{ status: string; is_read: boolean }>(`/api/articles/${id}/read`, { method: 'POST' }),
  readingHistory: (language: string) => request<Article[]>(`/api/reading-history?lang=${encodeURIComponent(language)}`),
  clearReadingHistory: () => request<{ cleared: boolean }>('/api/reading-history', { method: 'DELETE' }),
  me: () => request<TelegramUser>('/api/me'),
}
