import type { AIConfig, AIConfigInput, AIModel, Article, ArticleWatch, Category, Country, DailyDigestPreferences, FeaturedBrief, GoldRate, MediumReaderArticle, SavedCollection, SavedFilter, SavedFilterValues, SavedOrganization, Source, TelegramUser, ThreadComment, ThreadsAuthorModeration, ThreadsTarget, Translation } from './types'

const initData = window.Telegram?.WebApp?.initData ?? ''
const headers = (): HeadersInit => initData ? { Authorization: `tma ${initData}` } : {}
const adminOptions = (token: string, method = 'GET', body?: unknown): RequestInit => ({
  method,
  credentials: 'same-origin',
  headers: { ...(token ? { 'X-Admin-Token': token } : {}), ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) },
  ...(body === undefined ? {} : { body: JSON.stringify(body) }),
})

async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(url, { credentials: 'same-origin', ...options, headers: { ...headers(), ...options.headers } })
  if (!response.ok) {
    const body = await response.json().catch(() => ({ error: 'Request failed' }))
    throw new Error(body.error ?? 'Request failed')
  }
  return response.json() as Promise<T>
}

export const api = {
	adminLogin: (token: string) => request<{ authenticated: boolean; expires_at: string }>('/api/admin/login', { method: 'POST', body: JSON.stringify({ token }), headers: { 'Content-Type': 'application/json' } }),
	adminLogout: () => request<{ authenticated: boolean }>('/api/admin/logout', { method: 'POST' }),
	adminSession: () => request<{ authenticated: boolean }>('/api/admin/session'),
	adminSetArticleVisibility: (id: number, hidden: boolean) => request<{ hidden: boolean }>(`/api/admin/articles/${id}/visibility`, adminOptions('', 'PATCH', { hidden })),
	readMedium: (url: string) => request<MediumReaderArticle>('/api/reader/articles', { method: 'POST', body: JSON.stringify({ url }), headers: { 'Content-Type': 'application/json' } }),
  articles: (params: URLSearchParams) => request<Article[]>(`/api/articles?${params}`),
  threads: (params: URLSearchParams) => request<Article[]>(`/api/threads?${params}`),
  threadsTargets: () => request<ThreadsTarget[]>('/api/threads/targets'),
  threadsAuthors: () => request<string[]>('/api/threads/authors'),
	threadComments: (id: number) => request<ThreadComment[]>(`/api/threads/posts/${id}/comments`),
  adminThreadsTargets: (token: string) => request<ThreadsTarget[]>('/api/admin/threads/targets', adminOptions(token)),
	adminThreadsAuthorModeration: (username: string) => request<ThreadsAuthorModeration>(`/api/admin/threads/authors/${encodeURIComponent(username)}/moderation`, adminOptions('')),
	adminBlockThreadsAuthor: (username: string) => request<{ blocked: boolean }>(`/api/admin/threads/authors/${encodeURIComponent(username)}/block`, adminOptions('', 'PUT')),
	adminUnblockThreadsAuthor: (username: string) => request<{ blocked: boolean }>(`/api/admin/threads/authors/${encodeURIComponent(username)}/block`, adminOptions('', 'DELETE')),
	adminDeleteThreadsAuthorPosts: (username: string) => request<{ deleted: number }>(`/api/admin/threads/authors/${encodeURIComponent(username)}/posts`, adminOptions('', 'DELETE')),
  adminCreateThreadsTarget: (token: string, input: { kind: 'profile' | 'keyword'; query: string; enabled?: boolean }) => request<{ id: number; enabled: boolean }>('/api/admin/threads/targets', adminOptions(token, 'POST', input)),
  adminUpdateThreadsTarget: (token: string, id: number, input: { query?: string; enabled?: boolean }) => request<{ updated: boolean }>(`/api/admin/threads/targets/${id}`, adminOptions(token, 'PATCH', input)),
  adminDeleteThreadsTarget: (token: string, id: number) => request<{ deleted: boolean }>(`/api/admin/threads/targets/${id}`, adminOptions(token, 'DELETE')),
  adminDeleteThreadsTargetPosts: (token: string, id: number) => request<{ deleted: number }>(`/api/admin/threads/targets/${id}/posts`, adminOptions(token, 'DELETE')),
  adminFetchThreads: (token: string, targetIDs: number[] = []) => request<{ status: string }>('/api/admin/threads/fetch', adminOptions(token, 'POST', { target_ids: targetIDs })),
  adminDiscoverThreads: (token: string) => request<{ status: string; target_count: number }>('/api/admin/threads/discover', adminOptions(token, 'POST')),
  article: (id: number) => request<Article>(`/api/articles/${id}`),
	relatedArticles: (id: number) => request<Article[]>(`/api/articles/${id}/related`),
	translateArticle: (id: number, language: 'vi' | 'en') => request<Translation>(`/api/articles/${id}/translations/${language}`, { method: 'POST' }),
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
  savedFilters: () => request<SavedFilter[]>('/api/saved-filters'),
  createSavedFilter: (name: string, filter: SavedFilterValues) => request<SavedFilter>('/api/saved-filters', { method: 'POST', body: JSON.stringify({ name, filter }), headers: { 'Content-Type': 'application/json' } }),
  deleteSavedFilter: (id: number) => request<{ deleted: boolean }>(`/api/saved-filters/${id}`, { method: 'DELETE' }),
  dailyDigestPreferences: () => request<DailyDigestPreferences>('/api/daily-digest'),
  updateDailyDigestPreferences: (preferences: DailyDigestPreferences) => request<DailyDigestPreferences>('/api/daily-digest', { method: 'PUT', body: JSON.stringify(preferences), headers: { 'Content-Type': 'application/json' } }),
  watches: () => request<ArticleWatch[]>('/api/watches'),
  followArticle: (id: number) => request<ArticleWatch>(`/api/watches/articles/${id}`, { method: 'POST' }),
  followTopic: (slug: string) => request<ArticleWatch>(`/api/watches/topics/${encodeURIComponent(slug)}`, { method: 'POST' }),
  updateWatch: (id: number, enabled: boolean) => request<ArticleWatch>(`/api/watches/${id}`, { method: 'PATCH', body: JSON.stringify({ enabled }), headers: { 'Content-Type': 'application/json' } }),
  deleteWatch: (id: number) => request<{ deleted: boolean }>(`/api/watches/${id}`, { method: 'DELETE' }),
  adminStatus: (token: string, jobsPage = 1) => request<{ sources: { id: number; name: string; enabled: boolean; last_fetch_at: string; last_success_at: string; last_error: string; last_inserted: number }[]; translation_queue: number; ai_usage: { day: string; prompt_tokens: number; completion_tokens: number; total_tokens: number; requests: number }[]; jobs: { id: number; kind: string; title: string; status: 'running' | 'queued' | 'completed' | 'failed' | 'skipped'; detail: string; target_count: number; completed_count: number; failed_count: number; trigger: string; stage: string; started_at: string; finished_at: string }[]; jobs_page: number; jobs_page_size: number; jobs_total: number; daily_digest: { subscribers: number; sent_today: number; failed_today: number }; ai: { model: string; configured: boolean; translations_generated: number; featured_briefs: number; feedback: number; cost_tracking: string } }>(`/api/admin/status?jobs_page=${jobsPage}`, adminOptions(token)),
  adminFetchRSS: (token: string, sourceIDs: number[] = []) => request<{ status: string; source_count: number }>('/api/admin/rss/fetch', adminOptions(token, 'POST', { source_ids: sourceIDs })),
  adminEnqueueTranslations: (token: string, sourceIDs: number[]) => request<{ status: string; queued: number; source_count: number }>('/api/admin/translations/enqueue', adminOptions(token, 'POST', { source_ids: sourceIDs })),
  adminRegenerateFeatured: (token: string, articleCount: number) => request<{ status: string; article_count: number }>('/api/admin/featured/regenerate', adminOptions(token, 'POST', { article_count: articleCount })),
  adminCancelJob: (token: string, id: number) => request<{ status: string }>(`/api/admin/jobs/${id}`, adminOptions(token, 'DELETE')),
  adminClearJobHistory: (token: string, status: 'completed' | 'failed') => request<{ cleared: number }>(`/api/admin/jobs?status=${status}`, adminOptions(token, 'DELETE')),
  adminUpdateSource: (token: string, id: number, enabled: boolean) => request<{ enabled: boolean }>(`/api/admin/sources/${id}`, { method: 'PATCH', body: JSON.stringify({ enabled }), headers: { 'X-Admin-Token': token, 'Content-Type': 'application/json' } }),
  adminAIConfig: (token: string) => request<AIConfig>('/api/admin/ai/config', adminOptions(token)),
  adminAIModels: (token: string, input: AIConfigInput) => request<AIModel[]>('/api/admin/ai/models', adminOptions(token, 'POST', input)),
  adminAITest: (token: string, input: AIConfigInput) => request<{ ok: boolean; reply: string }>('/api/admin/ai/test', adminOptions(token, 'POST', input)),
  adminAISave: (token: string, input: AIConfigInput) => request<AIConfig>('/api/admin/ai/config', adminOptions(token, 'PUT', input)),
  adminAIReset: (token: string) => request<AIConfig>('/api/admin/ai/config', adminOptions(token, 'DELETE')),
  me: () => request<TelegramUser>('/api/me'),
}
