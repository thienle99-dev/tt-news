import { useMemo, useState } from 'react'
import type { Category, Country, Source } from './types'

type Locale = 'en' | 'vi'
type Filters = { category: string; source: string; country: string; query: string }
type IconName = 'search' | 'pin' | 'building' | 'tag' | 'chevron'

const categoryLabel = (slug: string, locale: Locale) => {
  const labels: Record<string, [string, string]> = { technology: ['Technology', 'Công nghệ'], world: ['World', 'Thế giới'], society: ['Society', 'Xã hội'], culture: ['Culture', 'Văn hóa'], sports: ['Sports', 'Thể thao'], education: ['Education', 'Giáo dục'], health: ['Health', 'Sức khỏe'], science: ['Science', 'Khoa học'] }
  return labels[slug]?.[locale === 'vi' ? 1 : 0] ?? slug
}

const categoryMark = (slug: string) => ({ technology: '◆', world: '◉', society: '●', culture: '◇', sports: '▸', education: '▤', health: '✚', science: '✦' }[slug] ?? '•')

const countryFlag = (code: string) => {
  if (code === 'GLOBAL') return '◉'
  if (!/^[A-Z]{2}$/.test(code)) return ''
  return String.fromCodePoint(...[...code].map(character => 0x1F1E6 + character.charCodeAt(0) - 65))
}

function FilterIcon({ name }: { name: IconName }) {
  const common = { fill: 'none', stroke: 'currentColor', strokeWidth: 1.8, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }
  const paths = { search: <><circle {...common} cx="10.8" cy="10.8" r="5.8" /><path {...common} d="m16 16 4 4" /></>, pin: <><path {...common} d="M19 10c0 5-7 10-7 10S5 15 5 10a7 7 0 1 1 14 0Z" /><circle {...common} cx="12" cy="10" r="2.2" /></>, building: <><path {...common} d="M4 21V5h12v16M16 9h4v12M8 9h4m-4 4h4m-4 4h4M2 21h20" /></>, tag: <path {...common} d="M20 13 13 20 4 11V4h7l9 9Z" />, chevron: <path {...common} d="m7 10 5 5 5-5" /> }
  return <svg className={`filter-icon filter-icon-${name}`} viewBox="0 0 24 24" aria-hidden="true">{paths[name]}</svg>
}

export function FilterControls({ locale, filter, setFilter, search, setSearch, countries, sources, categories }: { locale: Locale; filter: Filters; setFilter: (filter: Filters) => void; search: string; setSearch: (value: string) => void; countries: Country[]; sources: Source[]; categories: Category[] }) {
  const [sourcesOpen, setSourcesOpen] = useState(false)
  const [sourceSearch, setSourceSearch] = useState('')
  const selected = filter.source.split(',').filter(Boolean)
  const visibleSources = useMemo(() => {
    const term = sourceSearch.trim().toLocaleLowerCase()
    return (filter.country ? sources.filter(source => source.country_code === filter.country) : sources).filter(source => !term || source.name.toLocaleLowerCase().includes(term))
  }, [filter.country, sourceSearch, sources])
  const copy = locale === 'vi' ? { search: 'Tìm trong bản tin', country: 'Quốc gia', source: 'Nguồn tin', category: 'Thể loại', allCountries: 'Tất cả quốc gia', allSources: 'Tất cả nguồn', allCategories: 'Tất cả thể loại', selected: 'đã chọn', searchSources: 'Tìm nguồn tin', noSources: 'Không tìm thấy nguồn phù hợp' } : { search: 'Search briefs', country: 'Country', source: 'Sources', category: 'Category', allCountries: 'All countries', allSources: 'All sources', allCategories: 'All categories', selected: 'selected', searchSources: 'Search sources', noSources: 'No matching sources' }
  const changeCountry = (country: string) => {
    const retainedSources = sources.filter(source => source.country_code === country && selected.includes(String(source.id))).map(source => String(source.id)).join(',')
    setFilter({ ...filter, country, source: retainedSources })
  }
  const toggleSource = (id: string) => setFilter({ ...filter, source: selected.includes(id) ? selected.filter(value => value !== id).join(',') : [...selected, id].join(',') })
  return <section className="filter-controls" aria-label={locale === 'vi' ? 'Bộ lọc tin' : 'News filters'}>
    <label className="filter-search" htmlFor="brief-search"><span>{copy.search}</span><div className="filter-input-wrap"><FilterIcon name="search" /><input id="brief-search" type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder={copy.search} autoComplete="off" /></div></label>
    <label className="filter-field" htmlFor="country-filter"><span>{copy.country}</span><div className="filter-select-wrap"><FilterIcon name="pin" /><select id="country-filter" value={filter.country} onChange={event => changeCountry(event.target.value)}><option value="">{copy.allCountries}</option>{countries.map(item => <option value={item.code} key={item.code}>{countryFlag(item.code)} {item.name}</option>)}</select><FilterIcon name="chevron" /></div></label>
    <div className="source-select filter-field"><span id="source-filter-label">{copy.source}</span><button type="button" className="source-select-trigger" aria-labelledby="source-filter-label" aria-expanded={sourcesOpen} aria-controls="source-filter-menu" onClick={() => setSourcesOpen(value => !value)}><FilterIcon name="building" /><span>{selected.length ? `${selected.length} ${copy.selected}` : copy.allSources}</span><FilterIcon name="chevron" /></button>{sourcesOpen && <div className="source-select-menu" id="source-filter-menu" role="group" aria-labelledby="source-filter-label"><label className="source-search" htmlFor="source-search"><FilterIcon name="search" /><span className="sr-only">{copy.searchSources}</span><input id="source-search" type="search" value={sourceSearch} onChange={event => setSourceSearch(event.target.value)} placeholder={copy.searchSources} autoFocus /></label><div className="source-options">{visibleSources.map(source => <label key={source.id}><input type="checkbox" checked={selected.includes(String(source.id))} onChange={() => toggleSource(String(source.id))} />{source.name}</label>)}{!visibleSources.length && <p>{copy.noSources}</p>}</div></div>}</div>
    <div className="category-chips" role="group" aria-label={copy.category}><span><FilterIcon name="tag" />{copy.category}</span><div>{[{ slug: '', label: copy.allCategories }, ...categories.map(item => ({ slug: item.slug, label: categoryLabel(item.slug, locale) }))].map(item => <button type="button" className={filter.category === item.slug ? 'selected' : ''} aria-pressed={filter.category === item.slug} onClick={() => setFilter({ ...filter, category: item.slug })} key={item.slug || 'all'}>{item.slug && <b aria-hidden="true">{categoryMark(item.slug)}</b>}{item.label}</button>)}</div></div>
  </section>
}
