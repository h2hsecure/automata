import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import FormSubmissionsView from './FormSubmissionsView.vue'

const mocks = vi.hoisted(() => ({
  getForm: vi.fn(async (id: string) => ({
    ID: id,
    ContextID: 'ctx-1',
    Slug: 'contact-us',
    Name: 'Contact Us',
    Fields: [
      { slug: 'email', type: 'email', label: 'Email', required: true, validation: null, order: 0 },
      { slug: 'name', type: 'text', label: 'Full Name', required: false, validation: null, order: 1 },
    ],
    Settings: null,
    CreatedAt: '2026-01-01T00:00:00Z',
    UpdatedAt: '2026-01-01T00:00:00Z',
  })),
  listSubmissions: vi.fn(async () => ({ data: [] as any[], total: 0, page: 1, limit: 20 })),
  deleteSubmission: vi.fn(async () => {}),
}))

vi.mock('@/api/client', () => mocks)

let router: ReturnType<typeof createRouter>
let wrapper: VueWrapper

function makeSubmission(over: Record<string, any> = {}) {
  return {
    ID: 'sub-1',
    FormID: 'form-1',
    ContextID: 'ctx-1',
    Data: { email: 'e@x.com', name: 'Ann' },
    Files: [] as any[],
    CreatedAt: '2026-01-02T10:00:00Z',
    ...over,
  }
}

async function mountView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/forms/:id/submissions', component: FormSubmissionsView },
      { path: '/:pathMatch(.*)*', component: { template: '<div />' } },
    ],
  })
  await router.push('/forms/form-1/submissions')
  await router.isReady()
  wrapper = mount(FormSubmissionsView, {
    attachTo: document.body,
    global: { plugins: [pinia, router] },
  })
  await flushPromises()
  return wrapper
}

afterEach(() => {
  wrapper?.unmount()
  document.body.innerHTML = ''
})

describe('FormSubmissionsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listSubmissions.mockResolvedValue({ data: [] as any[], total: 0, page: 1, limit: 20 })
  })

  it('loads the form and renders dynamic columns with field values', async () => {
    mocks.listSubmissions.mockResolvedValue({ data: [makeSubmission()], total: 1, page: 1, limit: 20 })
    await mountView()

    expect(mocks.getForm).toHaveBeenCalledWith('form-1')
    const headers = Array.from(document.body.querySelectorAll('th')).map((th) => th.textContent)
    expect(headers).toEqual(['Email', 'Full Name', 'Files', 'Created'])
    expect(document.body.textContent).toContain('e@x.com')
    expect(document.body.textContent).toContain('Ann')
  })

  it('shows an empty state when there are no submissions', async () => {
    await mountView()
    expect(document.body.textContent).toContain('No submissions yet.')
  })

  it('paginates submissions (limit 20) with Prev/Next and a summary', async () => {
    mocks.listSubmissions.mockResolvedValue({ data: [makeSubmission()], total: 45, page: 1, limit: 20 })
    await mountView()

    expect(document.body.textContent).toContain('Showing 1-20 of 45')

    const next = Array.from(document.body.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Next'
    )!
    next.click()
    await flushPromises()

    const lastCall = (mocks.listSubmissions as any).mock.calls.at(-1)
    expect(lastCall).toEqual(['form-1', 2, 20])
    expect(document.body.textContent).toContain('Showing 21-40 of 45')
  })

  it('deletes a submission through the destructive confirm dialog and reloads', async () => {
    mocks.listSubmissions.mockResolvedValue({ data: [makeSubmission()], total: 1, page: 1, limit: 20 })
    await mountView()

    const deleteBtn = Array.from(document.body.querySelectorAll('button')).find(
      (b) => b.title === 'Delete submission'
    )!
    deleteBtn.click()
    await flushPromises()

    // Confirm dialog is teleported to body.
    const confirmBtn = Array.from(document.body.querySelectorAll('button')).find(
      (b) => b.textContent?.trim() === 'Confirm'
    )!
    confirmBtn.click()
    await flushPromises()

    expect(mocks.deleteSubmission).toHaveBeenCalledWith('form-1', 'sub-1')
    // The list is reloaded after the delete.
    expect((mocks.listSubmissions as any).mock.calls.length).toBe(2)
  })

  it('back button navigates to /forms', async () => {
    await mountView()

    const back = Array.from(document.body.querySelectorAll('button')).find(
      (b) => b.textContent?.includes('Back')
    )!
    back.click()
    await flushPromises()

    expect(router.currentRoute.value.path).toBe('/forms')
  })
})