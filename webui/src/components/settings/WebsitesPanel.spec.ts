import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import WebsitesPanel from './WebsitesPanel.vue'
import { useAuthStore } from '@/stores/auth'

const mocks = vi.hoisted(() => ({
  listContexts: vi.fn(async () => [
    {
      ID: 'ctx-1',
      Name: 'Site One',
      Slug: 'site-one',
      Domain: 'example.com',
      IsActive: true,
      CreatedAt: '2026-01-01T00:00:00Z',
    },
  ]),
  deleteContext: vi.fn(async () => {}),
  createContext: vi.fn(async () => {}),
  updateContext: vi.fn(async () => {}),
}))

vi.mock('@/api/client', () => mocks)
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({
    confirmStore: { value: { show: false } },
    show: vi.fn(async () => true),
    confirm: vi.fn(),
    cancel: vi.fn(),
  }),
}))

async function mountPanel() {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore(pinia).currentContextId = 'ctx-1'
  const wrapper = mount(WebsitesPanel, {
    global: { plugins: [pinia] },
  })
  await flushPromises()
  return wrapper
}

describe('WebsitesPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('surfaces the server error when delete fails', async () => {
    mocks.deleteContext.mockRejectedValueOnce({
      response: { data: { error: 'context has active members' } },
    })
    const wrapper = await mountPanel()

    const deleteButton = wrapper.find('button[title="Delete"]')
    expect(deleteButton.exists()).toBe(true)
    await deleteButton.trigger('click')
    await flushPromises()

    expect(mocks.deleteContext).toHaveBeenCalledWith('ctx-1')
    expect(wrapper.text()).toContain('context has active members')
  })

  it('falls back to a generic message when the server error is empty', async () => {
    mocks.deleteContext.mockRejectedValueOnce(new Error('network down'))
    const wrapper = await mountPanel()

    await wrapper.find('button[title="Delete"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Failed to delete website.')
  })
})