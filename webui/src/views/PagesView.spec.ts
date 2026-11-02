import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import PagesView from './PagesView.vue'
import { useAuthStore } from '@/stores/auth'

const mocks = vi.hoisted(() => ({
  listPages: vi.fn(async () => [] as any[]),
  createPage: vi.fn(async (d: any) => d),
  updatePage: vi.fn(async (_id: string, d: any) => d),
  deletePage: vi.fn(async () => {}),
}))

vi.mock('@/api/client', () => mocks)

let wrapper: VueWrapper

function bodyButtons() {
  return Array.from(document.body.querySelectorAll('button'))
}

async function mountView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  const auth = useAuthStore(pinia)
  auth.currentContextId = 'ctx-1'
  wrapper = mount(PagesView, {
    attachTo: document.body,
    global: { plugins: [pinia] },
  })
  await flushPromises()
  return wrapper
}

async function openCreateModal() {
  bodyButtons().find((b) => b.textContent?.includes('Create Page'))!.click()
  await flushPromises()
}

async function fillModal(slug: string, title: string) {
  const inputs = Array.from(document.body.querySelectorAll('input[type="text"]'))
  const slugInput = inputs.find((i) => i.placeholder === 'cookie-policy')!
  slugInput.value = slug
  slugInput.dispatchEvent(new Event('input'))
  const titleInput = inputs.find((i) => i.placeholder === 'Cookie Policy')!
  titleInput.value = title
  titleInput.dispatchEvent(new Event('input'))
  await flushPromises()
}

afterEach(() => {
  wrapper?.unmount()
  document.body.innerHTML = ''
  vi.clearAllMocks()
  mocks.listPages.mockImplementation(async () => [])
})

describe('PagesView', () => {
  it('lists pages with their public URL', async () => {
    mocks.listPages.mockImplementation(async () => [
      {
        ID: '1',
        ContextID: 'ctx-1',
        Slug: 'cookie-policy',
        Title: 'Cookie Policy',
        Content: '<p>We use cookies.</p>',
        IsActive: true,
        CreatedAt: '2026-01-01T00:00:00Z',
        UpdatedAt: '2026-01-01T00:00:00Z',
      },
    ])
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('cookie-policy')
    expect(wrapper.text()).toContain('Cookie Policy')
    expect(wrapper.text()).toContain('/page/ctx-1/cookie-policy')
  })

  it('creates a page and calls the API', async () => {
    await mountView()
    await openCreateModal()
    await fillModal('about', 'About Us')

    bodyButtons().find((b) => b.textContent?.trim() === 'Create')!.click()
    await flushPromises()

    expect(mocks.createPage).toHaveBeenCalledTimes(1)
    const [data] = (mocks.createPage as any).mock.calls[0]
    expect(data.slug).toBe('about')
    expect(data.title).toBe('About Us')
    expect(data.is_active).toBe(true)
  })

  it('shows an edit action for each page', async () => {
    mocks.listPages.mockImplementation(async () => [
      {
        ID: '1',
        ContextID: 'ctx-1',
        Slug: 'cookie-policy',
        Title: 'Cookie Policy',
        Content: '',
        IsActive: true,
        CreatedAt: '2026-01-01T00:00:00Z',
        UpdatedAt: '2026-01-01T00:00:00Z',
      },
    ])
    const wrapper = await mountView()
    const editBtn = bodyButtons().find((b) => b.getAttribute('title') === 'Edit')
    expect(editBtn).toBeTruthy()
  })

  it('shows a delete action for each page', async () => {
    mocks.listPages.mockImplementation(async () => [
      {
        ID: '1',
        ContextID: 'ctx-1',
        Slug: 'cookie-policy',
        Title: 'Cookie Policy',
        Content: '',
        IsActive: true,
        CreatedAt: '2026-01-01T00:00:00Z',
        UpdatedAt: '2026-01-01T00:00:00Z',
      },
    ])
    const wrapper = await mountView()
    const deleteBtn = bodyButtons().find((b) => b.getAttribute('title') === 'Delete')
    expect(deleteBtn).toBeTruthy()
  })
})