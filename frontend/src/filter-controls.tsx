import { useState } from 'react'
import type { Category, Country, Source } from './types'

type Locale = 'en' | 'vi'
type Filters = { category: string; source: string; country: string; query: string }

const categoryLabel = (slug: string, locale: Locale) => {
  const labels: Record<string, [string, string]> = {
    technology: ['Technology', 'Công nghệ'], world: ['World', 'Thế giới'], society: ['Society', 'Xã hội'],
    culture: ['Culture', 'Văn hóa'], sports: ['Sports', 'Thể thao'], education: ['Education', 'Giáo dục'],
    health: ['Health', 'Sức khỏe'], science: ['Science', 'Khoa học'],
  }
  return labels[slug]?.[locale === 'vi' ? 1 : 0] ?? slug
}

export function FilterControls({ locale, filter, setFilter, countries, sources, categories }: { locale: Locale; filter: Filters; setFilter: (filter: Filters) => void; countries: Country[]; sources: Source[]; categories: Category[] }) {
  const [sourcesOpen, setSourcesOpen] = useState(false)
  const selected = filter.source.split(',').filter(Boolean)
  const visibleSources = filter.country ? sources.filter(source => source.country_code === filter.country) : sources
  const copy = locale === 'vi' ? { country: 'Quốc gia', source: 'Nguồn tin', category: 'Thể loại', allCountries: 'Tất cả quốc gia', allSources: 'Tất cả nguồn', allCategories: 'Tất cả thể loại', selected: 'đã chọn' } : { country: 'Country', source: 'Sources', category: 'Category', allCountries: 'All countries', allSources: 'All sources', allCategories: 'All categories', selected: 'selected' }
  const changeCountry = (country: string) => {
    const retainedSources = sources.filter(source => source.country_code === country && selected.includes(String(source.id))).map(source => String(source.id)).join(',')
    setFilter({ ...filter, country, source: retainedSources })
  }
  const toggleSource = (id: string) => setFilter({ ...filter, source: selected.includes(id) ? selected.filter(value => value !== id).join(',') : [...selected, id].join(',') })
  return <section className="filter-controls" aria-label={locale === 'vi' ? 'Bộ lọc tin' : 'News filters'}>
    <label><span>{copy.country}</span><select value={filter.country} onChange={event => changeCountry(event.target.value)}><option value="">{copy.allCountries}</option>{countries.map(item => <option value={item.code} key={item.code}>{item.name}</option>)}</select></label>
    <div className="source-select"><span>{copy.source}</span><button type="button" className="source-select-trigger" aria-expanded={sourcesOpen} onClick={() => setSourcesOpen(value => !value)}>{selected.length ? `${selected.length} ${copy.selected}` : copy.allSources}<span aria-hidden="true">⌄</span></button>{sourcesOpen && <div className="source-select-menu">{visibleSources.map(source => <label key={source.id}><input type="checkbox" checked={selected.includes(String(source.id))} onChange={() => toggleSource(String(source.id))} />{source.name}</label>)}{!visibleSources.length && <p>{copy.allSources}</p>}</div>}</div>
    <label><span>{copy.category}</span><select value={filter.category} onChange={event => setFilter({ ...filter, category: event.target.value })}><option value="">{copy.allCategories}</option>{categories.map(item => <option value={item.slug} key={item.slug}>{categoryLabel(item.slug, locale)}</option>)}</select></label>
  </section>
}
