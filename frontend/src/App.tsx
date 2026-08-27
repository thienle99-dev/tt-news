import { useEffect, useMemo, useState } from 'react'
import { api } from './api'
import type { Article, Category, Country, Source } from './types'

type Tab = 'home' | 'saved' | 'settings'
type Filters = { category: string; source: string; country: string; query: string }
const flags: Record<string, string> = { GLOBAL: '◉', HK: '🇭🇰', GB: '🇬🇧', US: '🇺🇸' }
const ago = (v: string) => { const n = Math.floor((Date.now() - new Date(v).getTime()) / 60000); return n < 60 ? `${Math.max(1, n)} phút` : n < 1440 ? `${Math.floor(n / 60)} giờ` : new Date(v).toLocaleDateString('vi-VN', { day: '2-digit', month: 'short' }) }
const open = (url: string) => window.Telegram?.WebApp ? window.Telegram.WebApp.openLink(url) : window.open(url, '_blank', 'noopener,noreferrer')

function Card({ a, hero, toggle }: { a: Article; hero?: boolean; toggle: (a: Article) => void }) {
  return <article className={hero ? 'hero-card' : 'news-card'} onClick={() => open(a.url)}>
    {a.image_url ? <img src={a.image_url} alt="" /> : <div className="cover-placeholder">TIN TỨC</div>}
    <div className="card-copy"><div className="eyebrow">{a.source} · {flags[a.country_code] || '◉'} {a.country_name} · {ago(a.published_at)}</div><h2>{a.title}</h2>{a.summary && <p>{a.summary}</p>}<span className="category-label">{a.category}</span></div>
    <button className={`save ${a.is_saved ? 'active' : ''}`} onClick={e => { e.stopPropagation(); toggle(a) }}>{a.is_saved ? '★' : '☆'}</button>
  </article>
}

function Home({ saved = false }: { saved?: boolean }) {
  const [items, setItems] = useState<Article[]>([]); const [cats, setCats] = useState<Category[]>([]); const [sources, setSources] = useState<Source[]>([]); const [countries, setCountries] = useState<Country[]>([]); const [f, setF] = useState<Filters>({ category: '', source: '', country: '', query: '' }); const [loading, setLoading] = useState(true); const [show, setShow] = useState(false)
  const params = useMemo(() => new URLSearchParams({ limit: '30', ...(f.category && { category: f.category }), ...(f.source && { source: f.source }), ...(f.country && { country: f.country }), ...(f.query && { q: f.query }) }), [f])
  useEffect(() => { if (!saved) Promise.all([api.categories(), api.sources(), api.countries()]).then(([a,b,c]) => { setCats(a); setSources(b); setCountries(c) }) }, [saved])
  useEffect(() => { setLoading(true); (saved ? api.saved() : api.articles(params)).then(setItems).finally(() => setLoading(false)) }, [saved, params])
  const toggle = async (a: Article) => { setItems(x => x.map(v => v.id === a.id ? { ...v, is_saved: !v.is_saved } : v)); await api.toggleSaved(a); if (saved && !a.is_saved) setItems(x => x.filter(v => v.id !== a.id)) }
  const hero = !saved ? items[0] : undefined; const list = hero ? items.slice(1) : items
  return <section className="page editorial"><header className="masthead"><div><p>ĐIỂM TIN HÔM NAY</p><h1>{saved ? 'Đã lưu' : 'Tin tức'}</h1></div>{!saved && <button className="filter-button" onClick={() => setShow(true)}>☷ Lọc</button>}</header>
    {!saved && <><div className="topic-row"><button className={!f.category ? 'selected' : ''} onClick={() => setF({ ...f, category: '' })}>Mới nhất</button>{cats.map(c => <button className={f.category === c.slug ? 'selected' : ''} onClick={() => setF({ ...f, category: c.slug })} key={c.slug}>{c.name}</button>)}</div><div className="rail">{sources.map(s => <button className={f.source === String(s.id) ? 'source-pill active' : 'source-pill'} onClick={() => setF({ ...f, source: f.source === String(s.id) ? '' : String(s.id) })} key={s.id}><b>{s.name}</b><span>{flags[s.country_code] || '◉'} {s.country_name}</span></button>)}</div><div className="country-row"><button className={!f.country ? 'selected' : ''} onClick={() => setF({ ...f, country: '' })}>Tất cả</button>{countries.map(c => <button className={f.country === c.code ? 'selected' : ''} onClick={() => setF({ ...f, country: c.code })} key={c.code}>{flags[c.code] || '◉'} {c.name}</button>)}</div></>}
    {loading ? <p className="state">Đang tải tin mới…</p> : <>{hero && <><p className="section-kicker">NỔI BẬT</p><Card hero a={hero} toggle={toggle}/></>}<div className="feed"><div className="section-head"><h3>{saved ? 'Bài viết của bạn' : 'Mới nhất'}</h3><span>{list.length} bài</span></div>{list.map(a => <Card a={a} toggle={toggle} key={a.id}/>)}{!items.length && <p className="state">Không tìm thấy bài viết phù hợp.</p>}</div></>}
    {show && <div className="sheet-backdrop" onClick={() => setShow(false)}><section className="filter-sheet" onClick={e => e.stopPropagation()}><h2>Bộ lọc</h2><input value={f.query} onChange={e => setF({ ...f, query: e.target.value })} placeholder="Tìm kiếm"/><select value={f.category} onChange={e => setF({ ...f, category: e.target.value })}><option value="">Tất cả thể loại</option>{cats.map(c => <option value={c.slug} key={c.slug}>{c.name}</option>)}</select><select value={f.source} onChange={e => setF({ ...f, source: e.target.value })}><option value="">Tất cả nguồn</option>{sources.map(s => <option value={s.id} key={s.id}>{s.name}</option>)}</select><select value={f.country} onChange={e => setF({ ...f, country: e.target.value })}><option value="">Tất cả quốc gia</option>{countries.map(c => <option value={c.code} key={c.code}>{c.name}</option>)}</select><button className="primary" onClick={() => setShow(false)}>Xem kết quả</button></section></div>}
  </section>
}
function Settings(){return <section className="page settings"><h1>Cài đặt</h1><div className="settings-card"><small>GIAO DIỆN</small><p>Theme tự động theo Telegram.</p></div></section>}
export default function App(){const [tab,setTab]=useState<Tab>('home');useEffect(()=>{const a=window.Telegram?.WebApp;if(a){a.ready();a.expand()}},[]);return <main><div className="app-shell">{tab==='home'&&<Home/>}{tab==='saved'&&<Home saved/>}{tab==='settings'&&<Settings/>}</div><nav>{([['home','⌂','Trang chủ'],['saved','☆','Đã lưu'],['settings','⚙','Cài đặt']]as[Tab,string,string][]).map(([id,i,l])=><button className={tab===id?'nav-active':''} onClick={()=>setTab(id)} key={id}><span>{i}</span>{l}</button>)}</nav></main>}
