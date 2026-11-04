import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import DashboardView from './DashboardView.vue'
import { useAuthStore } from '@/stores/auth'

const mocks = vi.hoisted(() => ({
  getDashboardStats: vi.fn(async () => ({
    total_visitors: 10,
    active_visitors: 2,
    page_views: 50,
    top_pages: [{ URL: '/home', Count: 30 }],
    device_breakdown: { desktop: 30, mobile: 20 },
    top_referrers: [{ Referrer: 'google.com', Count: 10 }],
  })),
  listContacts: vi.fn(async () => ({
    data: [
      { ID: 'c1', GeoCountryCode: 'US', GeoCity: 'New York', Hidden: false, UpdatedAt: '2026-10-01T00:00:00Z' },
      { ID: 'c2', GeoCountryCode: 'TR', GeoCity: 'Istanbul', Hidden: true, UpdatedAt: '2026-10-02T00:00:00Z' },
    ],
    total: 2,
    page: 1,
    limit: 100,
  })),
  listForms: vi.fn(async () => [{ ID: 'f1', Name: 'Form 1' }]),
  listPages: vi.fn(async () => [
    { ID: 'p1', Slug: 'cookie-policy', Title: 'Cookie Policy' },
    { ID: 'p2', Slug: 'about', Title: 'About' },
  ]),
  getEmailStats: vi.fn(async () => {
    throw new Error('email down')
  }),
}))

vi.mock('@/api/client', () => mocks)

async function mountView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  const auth = useAuthStore(pinia)
  auth.currentContextId = 'ctx-1'
  const wrapper = mount(DashboardView, {
    attachTo: document.body,
    global: {
      plugins: [pinia],
      stubs: {
        'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' },
      },
    },
  })
  await flushPromises()
  return wrapper
}

describe('DashboardView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('shows loading states before API data resolves, not empty states', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore(pinia)
    auth.currentContextId = 'ctx-1'
    const wrapper = mount(DashboardView, {
      attachTo: document.body,
      global: {
        plugins: [pinia],
        stubs: {
          'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' },
        },
      },
    })
    // mounted, but the API promises have not resolved yet
    expect(wrapper.text()).toContain('Loading…')
    expect(wrapper.text()).not.toContain('No geo data')
    expect(wrapper.text()).not.toContain('No page view data yet')
    expect(wrapper.text()).not.toContain('No device data yet')
    expect(wrapper.text()).not.toContain('No referrer data yet')
  })

  it('renders a contact locations map card with the last 90 days of contacts', async () => {
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('Contact Locations')
    // both US and TR (incl. the hidden contact) get a number badge
    const badgeTexts = wrapper.findAll('.map-badge text').map((t) => t.text())
    expect(badgeTexts).toEqual(expect.arrayContaining(['1', '1']))
    expect(wrapper.find('path[data-code="US"]').exists()).toBe(true)
  })

  it('fetches map contacts with a since window of ~90 days and includes hidden', async () => {
    await mountView()
    const calls = mocks.listContacts.mock.calls
    const mapCall = calls.find(
      (c) => c[1] === 100 && c[4] === undefined && typeof c[5] === 'string',
    )
    expect(mapCall).toBeDefined()
    const since = new Date(mapCall![5] as string).getTime()
    const expected = Date.now() - 90 * 24 * 60 * 60 * 1000
    expect(Math.abs(since - expected)).toBeLessThan(2 * 60 * 60 * 1000)
  })

  it('has no events tab', async () => {
    const wrapper = await mountView()
    expect(wrapper.text()).not.toContain('Events')
    expect(wrapper.text()).not.toContain('Events Log')
  })

  it('links the forms card to /forms and the pages card to /pages', async () => {
    const wrapper = await mountView()
    const formsLink = wrapper.find('a[href="/forms"]')
    expect(formsLink.exists()).toBe(true)
    expect(formsLink.text()).toContain('Forms')
    const pagesLink = wrapper.find('a[href="/pages"]')
    expect(pagesLink.exists()).toBe(true)
    expect(pagesLink.text()).toContain('Pages')
    expect(pagesLink.text()).toContain('2')
    expect(mocks.listPages).toHaveBeenCalled()
  })

  it('keeps the top pages list with counts', async () => {
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('Top Pages')
    expect(wrapper.text()).toContain('/home')
    expect(wrapper.text()).toContain('30')
  })

  it('a stale loadOverview response does not overwrite fresh state', async () => {
    // The first (mount-time) loadOverview hangs on getDashboardStats...
    let resolveStale!: (v: unknown) => void
    mocks.getDashboardStats.mockImplementationOnce(
      () => new Promise((resolve) => { resolveStale = resolve })
    )
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore(pinia)
    auth.currentContextId = 'ctx-1'
    const wrapper = mount(DashboardView, {
      attachTo: document.body,
      global: {
        plugins: [pinia],
        stubs: {
          'router-link': { props: ['to'], template: '<a :href="to"><slot /></a>' },
        },
      },
    })
    // ...a context switch starts a fresh loadOverview (resolves via the
    // default mock)...
    auth.currentContextId = 'ctx-2'
    await flushPromises()
    // ...and then the stale response arrives late with bogus data.
    resolveStale({
      total_visitors: 999,
      active_visitors: 0,
      page_views: 0,
      top_pages: [],
      device_breakdown: {},
      top_referrers: [],
    })
    await flushPromises()
    expect(wrapper.text()).not.toContain('999')
    // fresh data (total_visitors: 10 from the default mock) is what renders
    expect(wrapper.text()).toContain('10')
  })
})