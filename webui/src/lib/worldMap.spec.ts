import { describe, it, expect } from 'vitest'
import { aggregateContacts, colorForCount } from './worldMap'

describe('aggregateContacts', () => {
  it('counts contacts per country code', () => {
    const result = aggregateContacts([
      { GeoCountryCode: 'US', GeoCity: 'New York' },
      { GeoCountryCode: 'US', GeoCity: 'Seattle' },
      { GeoCountryCode: 'TR', GeoCity: 'Istanbul' },
    ])
    expect(result).toEqual([
      { code: 'US', count: 2, cities: ['New York', 'Seattle'] },
      { code: 'TR', count: 1, cities: ['Istanbul'] },
    ])
  })

  it('skips contacts without a country code', () => {
    const result = aggregateContacts([
      { GeoCountryCode: '' },
      { GeoCountryCode: undefined },
      {},
      { GeoCountryCode: 'US', GeoCity: 'Boston' },
    ])
    expect(result).toEqual([{ code: 'US', count: 1, cities: ['Boston'] }])
  })

  it('normalizes country codes to uppercase', () => {
    const result = aggregateContacts([{ GeoCountryCode: ' de ', GeoCity: 'Berlin' }])
    expect(result).toEqual([{ code: 'DE', count: 1, cities: ['Berlin'] }])
  })

  it('orders cities by frequency and caps at 5', () => {
    const contacts = [
      { GeoCountryCode: 'US', GeoCity: 'A' },
      { GeoCountryCode: 'US', GeoCity: 'B' },
      { GeoCountryCode: 'US', GeoCity: 'B' },
      { GeoCountryCode: 'US', GeoCity: 'C' },
      { GeoCountryCode: 'US', GeoCity: 'C' },
      { GeoCountryCode: 'US', GeoCity: 'D' },
      { GeoCountryCode: 'US', GeoCity: 'E' },
      { GeoCountryCode: 'US', GeoCity: 'F' },
    ]
    const result = aggregateContacts(contacts)
    expect(result[0].count).toBe(8)
    expect(result[0].cities).toEqual(['B', 'C', 'A', 'D', 'E'])
  })
})

describe('colorForCount', () => {
  it('returns the neutral color for zero counts', () => {
    expect(colorForCount(0, 10)).toBe('#e2e8f0')
  })

  it('returns the darkest color at max', () => {
    expect(colorForCount(10, 10)).toBe('hsl(221 83% 45%)')
  })

  it('is monotonic: more contacts = darker', () => {
    // parse lightness from "hsl(221 83% NN%)"
    const lightness = (c: string) => Number(c.match(/(\d+)%\)/)?.[1])
    expect(lightness(colorForCount(1, 10))).toBeGreaterThan(lightness(colorForCount(9, 10)))
    expect(lightness(colorForCount(5, 10))).toBeLessThan(lightness(colorForCount(1, 10)))
  })
})