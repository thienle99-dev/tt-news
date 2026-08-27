import { useEffect, useMemo, useState } from 'react'
import { api } from './api'
import type { Article, Category, TelegramUser } from './types'

type Tab = 'home' | 'saved' | 'settings'
type Theme = 'system' | 'light' | 'dark'

function formatDate(value: string) {
  const date = new Date(value)
  const seconds = Math.max(0, Math.floor((Date.now() - date.getTime()) / 1000))
  if (seconds < 60) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function openArticle(url: string) {
  if (window.Telegram?.WebApp) window.Telegram.WebApp.openLink(url)
  else window.open(url, '_blank', 'noopener,noreferrer')
}

function ArticleCard({ article, onToggle }: { article: Article; onToggle: (article: Article) => void }) {
  return <article className="card">
    <button className="card-main" onClick={() => openArticle(article.url)} aria-label={`Open ${article.title}`}>
      {article.image_url ? <img src={article.image_url} alt="" loading="lazy" /> : <div className="image-placeholder">NEWS</div>}
      <div className="copy"><div className="meta"><span>{article.source}</span><span>{formatDate(article.published_at)}</span></div><h2>{article.title}</h2>{article.summary && <p className="summary">{article.summary}</p>}<span className="tag">{article.category}</span></div>
    </button>
    <button className={`save ${article.is_saved ? 'active' : ''}`} onClick={() => onToggle(article)} aria-label="Save article">{article.is_saved ? '★' : '☆'}</button>
  </article>
}

function Feed({ savedOnly = false }: { savedOnly?: boolean }) {
  const [articles, setArticles] = useState<Article[]>([])
  const [categories, setCategories] = useState<Category[]>([])
  const [category, setCategory] = useState('')
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const params = useMemo(() => new URLSearchParams({ ...(category ? { category } : {}), ...(query.trim() ? { q: query.trim() } : {}), limit: '30' }), [category, query])
  useEffect(() => { if (!savedOnly) api.categories().then(setCategories).catch(() => undefined) }, [savedOnly])
  useEffect(() => { setLoading(true); setError(''); const timer = window.setTimeout(() => { (savedOnly ? api.saved() : api.articles(params)).then(setArticles).catch(e => setError(e.message)).finally(() => setLoading(false)) }, savedOnly ? 0 : 250); return () => clearTimeout(timer) }, [savedOnly, params])
  const toggle = async (article: Article) => { const next = !article.is_saved; setArticles(items => items.map(item => item.id === article.id ? { ...item, is_saved: next } : item)); try { await api.toggleSaved(article); if (savedOnly && !next) setArticles(items => items.filter(item => item.id !== article.id)) } catch (e) { setArticles(items => items.map(item => item.id === article.id ? { ...item, is_saved: !next } : item)); setError(e instanceof Error ? e.message : 'Could not save article') } }
  return <section className="page"><div className="page-head"><h1>{savedOnly ? 'Saved' : 'Today’s news'}</h1>{!savedOnly && <input className="search" value={query} onChange={e => setQuery(e.target.value)} placeholder="Search articles" />}</div>{!savedOnly && <div className="chips"><button className={!category ? 'selected' : ''} onClick={() => setCategory('')}>All</button>{categories.map(c => <button className={category === c.slug ? 'selected' : ''} onClick={() => setCategory(c.slug)} key={c.slug}>{c.name}</button>)}</div>}{loading && <p className="state">Loading news…</p>}{error && <p className="state error">{error}</p>}{!loading && !error && articles.length === 0 && <p className="state">{savedOnly ? 'No saved articles yet.' : 'No articles match your search.'}</p>}<div className="feed">{articles.map(a => <ArticleCard key={a.id} article={a} onToggle={toggle} />)}</div></section>
}

function Settings() {
  const [theme, setTheme] = useState<Theme>((localStorage.getItem('theme') as Theme) || 'system')
  const [me, setMe] = useState<TelegramUser | null>(null)
  useEffect(() => { api.me().then(setMe).catch(() => setMe(null)) }, [])
  useEffect(() => { localStorage.setItem('theme', theme); document.documentElement.dataset.theme = theme }, [theme])
  return <section className="page"><h1>Settings</h1><div className="settings-card"><p className="label">Account</p><p>{me ? `${me.first_name} ${me.last_name}`.trim() : 'Open this app from Telegram to sign in'}</p>{me?.username && <p className="muted">@{me.username}</p>}</div><div className="settings-card"><p className="label">Appearance</p><div className="segmented">{(['system', 'light', 'dark'] as Theme[]).map(option => <button className={theme === option ? 'selected' : ''} key={option} onClick={() => setTheme(option)}>{option}</button>)}</div></div><p className="muted">Telegram News · MVP</p></section>
}

export default function App() {
  const [tab, setTab] = useState<Tab>('home')
  useEffect(() => { const app = window.Telegram?.WebApp; if (!app) return; const sync = () => document.documentElement.dataset.telegramTheme = app.colorScheme; sync(); app.ready(); app.expand(); app.onEvent('themeChanged', sync); return () => app.offEvent('themeChanged', sync) }, [])
  return <main><div className="app-shell">{tab === 'home' && <Feed />}{tab === 'saved' && <Feed savedOnly />}{tab === 'settings' && <Settings />}</div><nav>{([['home', '⌂', 'Home'], ['saved', '☆', 'Saved'], ['settings', '⚙', 'Settings']] as [Tab, string, string][]).map(([id, icon, label]) => <button key={id} className={tab === id ? 'nav-active' : ''} onClick={() => setTab(id)}><span>{icon}</span>{label}</button>)}</nav></main>
}
