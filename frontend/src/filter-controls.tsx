import { useMemo, useState } from 'react'
import type { Category, Country, Source } from './types'

type Locale = 'en' | 'vi'
type Filters = { category: string; source: string; country: string; query: string; period: string; sort: string }
type IconName = 'search' | 'pin' | 'building' | 'folder' | 'tag' | 'clock' | 'sort' | 'chevron' | 'close'
const filterCategorySlugs = new Set(['technology', 'programming', 'world', 'society', 'culture', 'sports', 'education', 'health', 'science', 'security', 'ai', 'llm', 'cybersecurity'])

const categoryLabel = (slug: string, locale: Locale, fallback = slug) => {
  const labels: Record<string, [string, string]> = { technology: ['Technology', 'Công nghệ'], programming: ['Programming', 'Lập trình'], world: ['World', 'Thế giới'], business: ['Business', 'Kinh doanh'], society: ['Society', 'Xã hội'], culture: ['Culture', 'Văn hóa'], sports: ['Sports', 'Thể thao'], education: ['Education', 'Giáo dục'], health: ['Health', 'Sức khỏe'], science: ['Science', 'Khoa học'], security: ['Security', 'Bảo mật'], ai: ['AI', 'Trí tuệ nhân tạo'], llm: ['LLM', 'Mô hình ngôn ngữ lớn'], cybersecurity: ['Cybersecurity', 'An ninh mạng'] }
  return labels[slug]?.[locale === 'vi' ? 1 : 0] ?? fallback
}

const categoryMark = (slug: string) => ({ technology: '◆', programming: '⌘', world: '◉', society: '●', culture: '◇', sports: '▸', education: '▤', health: '✚', science: '✦', security: '◈', ai: '◎', llm: '▣', cybersecurity: '◉' }[slug] ?? '•')

const countryFlag = (code: string) => {
  if (code === 'GLOBAL') return '◉'
  if (!/^[A-Z]{2}$/.test(code)) return ''
  return String.fromCodePoint(...[...code].map(character => 0x1F1E6 + character.charCodeAt(0) - 65))
}

export function FilterIcon({ name }: { name: IconName }) {
  const common = { fill: 'none', stroke: 'currentColor', strokeWidth: 1.8, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }
  const paths = { search: <><circle {...common} cx="10.8" cy="10.8" r="5.8" /><path {...common} d="m16 16 4 4" /></>, pin: <><path {...common} d="M19 10c0 5-7 10-7 10S5 15 5 10a7 7 0 1 1 14 0Z" /><circle {...common} cx="12" cy="10" r="2.2" /></>, building: <><path {...common} d="M4 21V5h12v16M16 9h4v12M8 9h4m-4 4h4m-4 4h4M2 21h20" /></>, folder: <path {...common} d="M3.5 6.5h6l2 2h9v9a2 2 0 0 1-2 2h-13a2 2 0 0 1-2-2v-11Z" />, tag: <path {...common} d="M20 13 13 20 4 11V4h7l9 9Z" />, clock: <><circle {...common} cx="12" cy="12" r="8.5" /><path {...common} d="M12 7v5l3.5 2" /></>, sort: <><path {...common} d="M8 6h10M8 12h7M8 18h4" /><path {...common} d="m3.5 8 2-2 2 2M5.5 6v12m-2-2 2 2 2-2" /></>, chevron: <path {...common} d="m7 10 5 5 5-5" />, close: <path {...common} d="m7.5 7.5 9 9m0-9-9 9" /> }
  return <svg className={`filter-icon filter-icon-${name}`} viewBox="0 0 24 24" aria-hidden="true">{paths[name]}</svg>
}

export function FilterSelect({ id, label, icon, value, options, onChange, className = '' }: { id: string; label: string; icon: 'clock' | 'sort' | 'folder' | 'tag'; value: string; options: { value: string; label: string }[]; onChange: (value: string) => void; className?: string }) {
  return <label className={`filter-field filter-select-field ${className}`} htmlFor={id}>
    <span>{label}</span>
    <div className="filter-select-wrap">
      <FilterIcon name={icon} />
      <select id={id} value={value} onChange={event => onChange(event.target.value)}>
        {options.map(option => <option value={option.value} key={option.value}>{option.label}</option>)}
      </select>
      <FilterIcon name="chevron" />
    </div>
  </label>
}

export function FilterControls({ locale, filter, setFilter, search, setSearch, countries, sources, categories }: { locale: Locale; filter: Filters; setFilter: (filter: Filters) => void; search: string; setSearch: (value: string) => void; countries: Country[]; sources: Source[]; categories: Category[] }) {
  const [sourcesOpen, setSourcesOpen] = useState(false)
  const [sourceSearch, setSourceSearch] = useState('')
  const selected = filter.source.split(',').filter(Boolean)
  const selectedSources = selected.map(id => sources.find(source => String(source.id) === id)).filter((source): source is Source => Boolean(source))
  const visibleSources = useMemo(() => {
    const term = sourceSearch.trim().toLocaleLowerCase()
    return (filter.country ? sources.filter(source => source.country_code === filter.country) : sources).filter(source => !term || source.name.toLocaleLowerCase().includes(term))
  }, [filter.country, sourceSearch, sources])
  const filterCategories = useMemo(() => categories.filter(item => filterCategorySlugs.has(item.slug)), [categories])
  const copy = locale === 'vi' ? { search: 'Tìm trong bản tin', country: 'Quốc gia', source: 'Nguồn tin', category: 'Thể loại', time: 'Thời gian', sort: 'Sắp xếp', allTime: 'Mọi thời điểm', last24Hours: '24 giờ qua', last7Days: '7 ngày qua', newest: 'Mới nhất', oldest: 'Cũ nhất', relevant: 'Liên quan nhất', worthReading: 'Đáng đọc', allCountries: 'Tất cả quốc gia', allSources: 'Tất cả nguồn', allCategories: 'Tất cả thể loại', selected: 'đã chọn', searchSources: 'Tìm nguồn tin', noSources: 'Không tìm thấy nguồn phù hợp' } : { search: 'Search briefs', country: 'Country', source: 'Sources', category: 'Category', time: 'Time', sort: 'Sort', allTime: 'Any time', last24Hours: 'Last 24 hours', last7Days: 'Last 7 days', newest: 'Newest', oldest: 'Oldest', relevant: 'Most relevant', worthReading: 'Worth reading', allCountries: 'All countries', allSources: 'All sources', allCategories: 'All categories', selected: 'selected', searchSources: 'Search sources', noSources: 'No matching sources' }
  const changeCountry = (country: string) => {
    const retainedSources = sources.filter(source => source.country_code === country && selected.includes(String(source.id))).map(source => String(source.id)).join(',')
    setFilter({ ...filter, country, source: retainedSources })
  }
  const toggleSource = (id: string) => setFilter({ ...filter, source: selected.includes(id) ? selected.filter(value => value !== id).join(',') : [...selected, id].join(',') })
  return <section className="filter-controls" aria-label={locale === 'vi' ? 'Bộ lọc tin' : 'News filters'}>
    <label className="filter-search" htmlFor="brief-search"><span>{copy.search}</span><div className="filter-input-wrap"><FilterIcon name="search" /><input id="brief-search" type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder={copy.search} autoComplete="off" /></div></label>
    <label className="filter-field" htmlFor="country-filter"><span>{copy.country}</span><div className="filter-select-wrap"><FilterIcon name="pin" /><select id="country-filter" value={filter.country} onChange={event => changeCountry(event.target.value)}><option value="">{copy.allCountries}</option>{countries.map(item => <option value={item.code} key={item.code}>{countryFlag(item.code)} {item.name}</option>)}</select><FilterIcon name="chevron" /></div></label>
    <FilterSelect id="period-filter" label={copy.time} icon="clock" value={filter.period} onChange={period => setFilter({ ...filter, period })} options={[{ value: '', label: copy.allTime }, { value: '24h', label: copy.last24Hours }, { value: '7d', label: copy.last7Days }]} />
    <FilterSelect id="sort-filter" label={copy.sort} icon="sort" value={filter.sort} onChange={sort => setFilter({ ...filter, sort })} options={[{ value: 'worth_read', label: copy.worthReading }, { value: 'newest', label: copy.newest }, { value: 'oldest', label: copy.oldest }, { value: 'relevant', label: copy.relevant }]} />
    <div className="source-select filter-field"><span id="source-filter-label">{copy.source}</span><button type="button" className="source-select-trigger" aria-labelledby="source-filter-label" aria-expanded={sourcesOpen} aria-controls="source-filter-menu" onClick={() => setSourcesOpen(value => !value)}><FilterIcon name="building" /><span>{selected.length ? `${selected.length} ${copy.selected}` : copy.allSources}</span><FilterIcon name="chevron" /></button>{selectedSources.length > 0 && <div className="source-selected-tags" aria-label={`${copy.source}: ${selected.length} ${copy.selected}`}>{selectedSources.map(source => <span className="source-selected-tag" key={source.id}><span title={source.name}>{source.name}</span><button type="button" aria-label={`${locale === 'vi' ? 'Bỏ chọn' : 'Remove'} ${source.name}`} onClick={() => toggleSource(String(source.id))}><FilterIcon name="close" /></button></span>)}</div>}{sourcesOpen && <div className="source-select-menu" id="source-filter-menu" role="group" aria-labelledby="source-filter-label"><label className="source-search" htmlFor="source-search"><FilterIcon name="search" /><span className="sr-only">{copy.searchSources}</span><input id="source-search" type="search" value={sourceSearch} onChange={event => setSourceSearch(event.target.value)} placeholder={copy.searchSources} autoFocus /></label><div className="source-options">{visibleSources.map(source => <label key={source.id}><input type="checkbox" checked={selected.includes(String(source.id))} onChange={() => toggleSource(String(source.id))} />{source.name}</label>)}{!visibleSources.length && <p>{copy.noSources}</p>}</div></div>}</div>
    <div className="category-chips" role="group" aria-label={copy.category}><span><FilterIcon name="tag" />{copy.category}</span><div>{[{ slug: '', label: copy.allCategories }, ...filterCategories.map(item => ({ slug: item.slug, label: categoryLabel(item.slug, locale, item.name) }))].map(item => <button type="button" className={`category-chip category-${item.slug || 'all'}${filter.category === item.slug ? ' selected' : ''}`} aria-pressed={filter.category === item.slug} onClick={() => setFilter({ ...filter, category: item.slug })} key={item.slug || 'all'}>{item.slug && <b aria-hidden="true">{categoryMark(item.slug)}</b>}{item.label}</button>)}</div></div>
  </section>
}
