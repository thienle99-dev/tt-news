import type { AIConfig, AIConfigInput, AIModel, Article, Category, Country, FeaturedBrief, GoldRate, MediumReaderArticle, SavedCollection, SavedOrganization, Source, TelegramUser, Translation } from './types'

const initData = window.Telegram?.WebApp?.initData ?? ''
const headers = (): HeadersInit => initData ? { Authorization: `tma ${initData}` } : {}
const adminOptions = (token: string, method = 'GET', body?: unknown): RequestInit => ({
  method,
  headers: { 'X-Admin-Token': token, ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) },
  ...(body === undefined ? {} : { body: JSON.stringify(body) }),
})

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(url, { ...options, headers: { ...headers(), ...options.headers } })
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: 'Request failed' }))
    throw new Error(body.error ?? 'Request failed')
  }
  return response.json() as Promise<T>
}

export const api = {
	readMedium: (url: string) => request<MediumReaderArticle>('/api/reader/articles', { method: 'POST', body: JSON.stringify({ url }), headers: { 'Content-Type': 'application/json' } }),
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
  adminStatus: (token: string) => request<{ sources: { id: number; name: string; enabled: boolean; last_fetch_at: string; last_success_at: string; last_error: string; last_inserted: number }[]; translation_queue: number; jobs: { id: number; kind: string; title: string; status: 'running' | 'queued' | 'completed' | 'failed' | 'skipped'; detail: string; target_count: number; started_at: string; finished_at: string }[]; ai: { model: string; configured: boolean; translations_generated: number; featured_briefs: number; feedback: number; cost_tracking: string } }>('/api/admin/status', { headers: { 'X-Admin-Token': token } }),
  adminFetchRSS: (token: string, sourceIDs: number[] = []) => request<{ status: string; source_count: number }>('/api/admin/rss/fetch', adminOptions(token, 'POST', { source_ids: sourceIDs })),
  adminEnqueueTranslations: (token: string, sourceIDs: number[]) => request<{ status: string; queued: number; source_count: number }>('/api/admin/translations/enqueue', adminOptions(token, 'POST', { source_ids: sourceIDs })),
  adminRegenerateFeatured: (token: string, articleCount: number) => request<{ status: string; article_count: number }>('/api/admin/featured/regenerate', adminOptions(token, 'POST', { article_count: articleCount })),
  adminUpdateSource: (token: string, id: number, enabled: boolean) => request<{ enabled: boolean }>(`/api/admin/sources/${id}`, { method: 'PATCH', body: JSON.stringify({ enabled }), headers: { 'X-Admin-Token': token, 'Content-Type': 'application/json' } }),
  adminAIConfig: (token: string) => request<AIConfig>('/api/admin/ai/config', adminOptions(token)),
  adminAIModels: (token: string, input: AIConfigInput) => request<AIModel[]>('/api/admin/ai/models', adminOptions(token, 'POST', input)),
  adminAITest: (token: string, input: AIConfigInput) => request<{ ok: boolean; reply: string }>('/api/admin/ai/test', adminOptions(token, 'POST', input)),
  adminAISave: (token: string, input: AIConfigInput) => request<AIConfig>('/api/admin/ai/config', adminOptions(token, 'PUT', input)),
  adminAIReset: (token: string) => request<AIConfig>('/api/admin/ai/config', adminOptions(token, 'DELETE')),
  me: () => request<TelegramUser>('/api/me'),
}
