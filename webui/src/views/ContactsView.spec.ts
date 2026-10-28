import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import ContactsView from './ContactsView.vue'
import { useAuthStore } from '@/stores/auth'

const mocks = vi.hoisted(() => ({
  listContacts: vi.fn(async () => ({
    data: [] as any[],
    total: 0,
    page: 1,
    limit: 20,
  })),
  deleteContact: vi.fn(async () => {}),
  updateContact: vi.fn(async (_id: string, d: any) => d),
}))

vi.mock('@/api/client', () => mocks)

async function mountView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore(pinia).currentContextId = 'ctx-1'
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
  })
  const wrapper = mount(ContactsView, {
    attachTo: document.body,
    global: {
      plugins: [pinia, router],
    },
  })
  await flushPromises()
  return wrapper
}

// The hidden param is the 5th argument of listContacts
// (page, limit, search, contextId, hidden)
function lastHiddenArg(): unknown {
  const calls = mocks.listContacts.mock.calls
  return calls[calls.length - 1]?.[4]
}

describe('ContactsView hidden filter', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('defaults to "All" and includes hidden contacts (no hidden param)', async () => {
    await mountView()
    expect(mocks.listContacts).toHaveBeenCalled()
    expect(lastHiddenArg()).toBeUndefined()
  })

  it('renders the three filter options', async () => {
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('All')
    expect(wrapper.text()).toContain('Hidden only')
    expect(wrapper.text()).toContain('Visible only')
  })

  it('passes hidden=true when "Hidden only" is selected', async () => {
    const wrapper = await mountView()
    const buttons = wrapper.findAll('button')
    const hiddenBtn = buttons.find((b) => b.text() === 'Hidden only')
    expect(hiddenBtn).toBeDefined()
    await hiddenBtn!.trigger('click')
    await flushPromises()
    expect(lastHiddenArg()).toBe(true)
  })

  it('passes hidden=false when "Visible only" is selected', async () => {
    const wrapper = await mountView()
    const buttons = wrapper.findAll('button')
    const visibleBtn = buttons.find((b) => b.text() === 'Visible only')
    expect(visibleBtn).toBeDefined()
    await visibleBtn!.trigger('click')
    await flushPromises()
    expect(lastHiddenArg()).toBe(false)
  })

  it('switches back to "All" (no hidden param)', async () => {
    const wrapper = await mountView()
    const buttons = wrapper.findAll('button')
    const hiddenBtn = buttons.find((b) => b.text() === 'Hidden only')
    await hiddenBtn!.trigger('click')
    await flushPromises()
    const allBtn = wrapper.findAll('button').find((b) => b.text() === 'All')
    await allBtn!.trigger('click')
    await flushPromises()
    expect(lastHiddenArg()).toBeUndefined()
  })
})