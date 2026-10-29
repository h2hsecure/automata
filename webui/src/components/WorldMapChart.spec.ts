import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import WorldMapChart from './WorldMapChart.vue'
import { colorForCount } from '@/lib/worldMap'

vi.mock('@/assets/world-countries.json', () => ({
  default: {
    US: { name: 'United States', d: 'M0 0L10 0L10 10Z', cx: 10, cy: 20 },
    FR: { name: 'France', d: 'M20 0L30 0L30 10Z', cx: 25, cy: 30 },
  },
}))

describe('WorldMapChart', () => {
  it('renders one path per country in the world file', () => {
    const wrapper = mount(WorldMapChart, { props: { countries: [] } })
    expect(wrapper.find('path[data-code="US"]').exists()).toBe(true)
    expect(wrapper.find('path[data-code="FR"]').exists()).toBe(true)
  })

  it('fills countries by contact count and shows a number badge', () => {
    const wrapper = mount(WorldMapChart, {
      props: {
        countries: [
          { code: 'US', count: 5, cities: ['New York'] },
          { code: 'FR', count: 1, cities: [] },
        ],
      },
    })

    const us = wrapper.find('path[data-code="US"]')
    const fr = wrapper.find('path[data-code="FR"]')
    expect(us.attributes('fill')).toBe(colorForCount(5, 5))
    expect(fr.attributes('fill')).toBe(colorForCount(1, 5))

    // number badge at the US centroid
    const badges = wrapper.findAll('.map-badge text')
    expect(badges.map((b) => b.text())).toContain('5')
    expect(badges.map((b) => b.text())).toContain('1')
    expect(wrapper.find('circle.map-badge-dot').attributes('cx')).toBe('10')
  })

  it('leaves countries without data at the neutral color', () => {
    const wrapper = mount(WorldMapChart, {
      props: { countries: [{ code: 'US', count: 3, cities: [] }] },
    })
    expect(wrapper.find('path[data-code="FR"]').attributes('fill')).toBe('#e2e8f0')
  })

  it('does not crash and keeps rendering the map when a contact country is missing from the world file', () => {
    // HK is a real ISO code that is not present in the (mocked) world file —
    // mirrors the production case where a GeoCountryCode has no map entry.
    expect(() =>
      mount(WorldMapChart, {
        props: {
          countries: [
            { code: 'US', count: 4, cities: [] },
            { code: 'HK', count: 1, cities: [] },
          ],
        },
      }),
    ).not.toThrow()

    const wrapper = mount(WorldMapChart, {
      props: {
        countries: [
          { code: 'US', count: 4, cities: [] },
          { code: 'HK', count: 1, cities: [] },
        ],
      },
    })

    // The map itself still renders with its country paths...
    expect(wrapper.find('svg').exists()).toBe(true)
    expect(wrapper.find('path[data-code="US"]').exists()).toBe(true)
    expect(wrapper.find('path[data-code="FR"]').exists()).toBe(true)
    // ...and the known country gets its badge, while the unknown one is skipped.
    expect(wrapper.findAll('.map-badge').length).toBe(1)
    expect(wrapper.findAll('.map-badge text').map((b) => b.text())).toContain('4')
  })
})