import type { Article, Category, TelegramUser } from './types'

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
  categories: () => request<Category[]>('/api/categories'),
  saved: () => request<Article[]>('/api/saved'),
  toggleSaved: (article: Article) => request<{ saved: boolean }>(`/api/saved/${article.id}`, { method: article.is_saved ? 'DELETE' : 'POST' }),
  me: () => request<TelegramUser>('/api/me'),
}
