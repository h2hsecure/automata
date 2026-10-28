// Pure helpers for the dashboard world map (no Vue dependencies).

export interface CountryStat {
  code: string
  count: number
  cities: string[]
}

// Aggregates contacts by ISO alpha-2 country code. Contacts without a code
// are skipped; cities are deduped, ordered by frequency, capped at 5.
export function aggregateContacts(
  contacts: Array<{ GeoCountryCode?: string; GeoCity?: string }>,
): CountryStat[] {
  type Entry = { count: number; cityCounts: Map<string, number> }
  const map = new Map<string, Entry>()

  for (const c of contacts) {
    const code = (c.GeoCountryCode ?? '').trim().toUpperCase()
    if (!code) continue
    const entry = map.get(code) ?? { count: 0, cityCounts: new Map() }
    entry.count += 1
    const city = (c.GeoCity ?? '').trim()
    if (city) entry.cityCounts.set(city, (entry.cityCounts.get(city) ?? 0) + 1)
    map.set(code, entry)
  }

  return [...map.entries()]
    .map(([code, e]) => ({
      code,
      count: e.count,
      cities: [...e.cityCounts.entries()]
        .sort((a, b) => b[1] - a[1])
        .slice(0, 5)
        .map(([city]) => city),
    }))
    .sort((a, b) => b.count - a.count)
}

// Choropleth color: neutral slate for 0, a sequential blue scale (light →
// dark) as the count approaches the max.
export function colorForCount(count: number, max: number): string {
  if (count <= 0) return '#e2e8f0'
  const t = Math.min(1, count / Math.max(1, max))
  const lightness = 82 - t * 37 // 82% → 45%
  return `hsl(221 83% ${Math.round(lightness)}%)`
}