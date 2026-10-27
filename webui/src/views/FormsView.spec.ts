import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import FormsView from './FormsView.vue'
import { useAuthStore } from '@/stores/auth'

const mocks = vi.hoisted(() => ({
  listForms: vi.fn(async () => [] as any[]),
  createForm: vi.fn(async (d: any) => d),
  updateForm: vi.fn(async (_id: string, d: any) => d),
  deleteForm: vi.fn(async () => {}),
  listSubmissions: vi.fn(async () => ({ data: [] as any[], total: 0, page: 1, limit: 20 })),
  getSubmission: vi.fn(async (id: string) => ({ ID: id })),
  downloadSubmissionFile: vi.fn(async () => new Blob(['file'])),
}))

vi.mock('@/api/client', () => mocks)

import { createForm } from '@/api/client'

let wrapper: VueWrapper

async function mountView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  const auth = useAuthStore(pinia)
  auth.currentContextId = 'ctx-1'
  wrapper = mount(FormsView, {
    attachTo: document.body,
    global: { plugins: [pinia] },
  })
  await flushPromises()
  return wrapper
}

// The create/edit modal is teleported to <body>, so query there.
function bodyButtons() {
  return Array.from(document.body.querySelectorAll('button'))
}

// Find a checkbox by the text of its surrounding <label>.
function checkboxByLabel(text: string): HTMLInputElement {
  const label = Array.from(document.body.querySelectorAll('label')).find((l) =>
    l.textContent?.includes(text)
  )
  if (!label) throw new Error(`label "${text}" not found`)
  const input = label.querySelector('input[type="checkbox"]')
  if (!input) throw new Error(`checkbox in label "${text}" not found`)
  return input
}

function adminEmailInput(): HTMLInputElement {
  const input = document.body.querySelector(
    'input[placeholder="admin@example.com, ops@example.com"]'
  )
  if (!input) throw new Error('admin recipients input not found')
  return input as HTMLInputElement
}

afterEach(() => {
  wrapper?.unmount()
  document.body.innerHTML = ''
})

describe('FormsView file fields', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('offers a File type in the field type select', async () => {
    await mountView()
    const createBtn = bodyButtons().find((b) => b.textContent?.includes('Create Form'))!
    createBtn.click()
    await flushPromises()

    const options = Array.from(document.body.querySelectorAll('select option')).map(
      (o) => o.getAttribute('value')
    )
    expect(options).toContain('file')
  })

  it('sends the accept list as field validation for file fields', async () => {
    await mountView()
    bodyButtons().find((b) => b.textContent?.includes('Create Form'))!.click()
    await flushPromises()

    // The modal's name input is the first text input in the body.
    const nameInput = document.body.querySelector('input[type="text"]')!
    nameInput.value = 'Contact Form'
    nameInput.dispatchEvent(new Event('input'))
    await flushPromises()

    // Add a third field.
    bodyButtons().find((b) => b.textContent?.includes('Add Field'))!.click()
    await flushPromises()

    // Field rows are the flex boxes; the notifications section is not.
    const rows = document.body.querySelectorAll('div.flex.rounded-md.border')
    expect(rows.length).toBe(3)
    const third = rows[2]

    // Label + name inputs in the row.
    const inputs = third.querySelectorAll('input[type="text"]')
    inputs[0].value = 'Attachment'
    inputs[0].dispatchEvent(new Event('input'))
    inputs[1].value = 'attachment'
    inputs[1].dispatchEvent(new Event('input'))
    await flushPromises()

    // Switch type to file.
    const select = third.querySelector('select')!
    select.value = 'file'
    select.dispatchEvent(new Event('change'))
    await flushPromises()

    // The accept input appears only for file fields.
    const rowInputs = third.querySelectorAll('input[type="text"]')
    expect(rowInputs.length).toBe(3)
    rowInputs[2].value = 'image/png, application/pdf'
    rowInputs[2].dispatchEvent(new Event('input'))
    await flushPromises()

    // Save (footer button labelled "Create").
    const saveBtn = bodyButtons().find((b) => b.textContent?.trim() === 'Create')!
    saveBtn.click()
    await flushPromises()

    expect(createForm).toHaveBeenCalled()
    const payload = (createForm as any).mock.calls[0][0]
    const field = payload.fields.find((f: any) => f.slug === 'attachment')
    expect(field.type).toBe('file')
    expect(field.validation).toEqual({ accept: ['image/png', 'application/pdf'] })
  })
})

describe('FormsView notification settings', () => {
  const settingsWithNotifications = (notifications: any, extra: Record<string, any> = {}) => ({
    notifications,
    ...extra,
  })

  function makeForm(settings: any, extra: Record<string, any> = {}) {
    return {
      ID: 'form-1',
      ContextID: 'ctx-1',
      Slug: 'contact-us',
      Name: 'Contact Us',
      Fields: [
        { slug: 'email', type: 'email', label: 'Email', required: true, validation: null, order: 0 },
      ],
      Settings: settingsWithNotifications(settings, extra),
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
    }
  }

  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listForms.mockResolvedValue([])
  })

  async function openCreate() {
    await mountView()
    bodyButtons().find((b) => b.textContent?.includes('Create Form'))!.click()
    await flushPromises()
  }

  function fillName() {
    const nameInput = document.body.querySelector('input[type="text"]')!
    nameInput.value = 'Contact Form'
    nameInput.dispatchEvent(new Event('input'))
  }

  it('shows notification toggles in the create modal, both enabled by default', async () => {
    await openCreate()
    expect(checkboxByLabel('Notify admins').checked).toBe(true)
    expect(checkboxByLabel('submitter').checked).toBe(true)
    expect(adminEmailInput()).toBeTruthy()
  })

  it('sends settings.notifications in the create payload', async () => {
    await openCreate()
    fillName()
    await flushPromises()
    adminEmailInput().value = 'a@x.com, b@x.com'
    adminEmailInput().dispatchEvent(new Event('input'))
    await flushPromises()

    bodyButtons().find((b) => b.textContent?.trim() === 'Create')!.click()
    await flushPromises()

    const payload = (mocks.createForm as any).mock.calls[0][0]
    expect(payload.settings.notifications).toEqual({
      admin: { enabled: true, to: ['a@x.com', 'b@x.com'] },
      submitter: { enabled: true },
    })
  })

  it('pre-fills notification settings when editing a form', async () => {
    mocks.listForms.mockResolvedValue([
      makeForm({
        admin: { enabled: true, to: ['a@x.com', 'b@x.com'] },
        submitter: { enabled: false },
      }),
    ])
    await mountView()
    bodyButtons().find((b) => b.title === 'Edit')!.click()
    await flushPromises()

    expect(checkboxByLabel('Notify admins').checked).toBe(true)
    expect(adminEmailInput().value).toBe('a@x.com, b@x.com')
    expect(checkboxByLabel('submitter').checked).toBe(false)
  })

  it('preserves other Settings keys when saving an edit', async () => {
    mocks.listForms.mockResolvedValue([
      makeForm(
        { admin: { enabled: false, to: [] }, submitter: { enabled: true } },
        { tracking: { enabled: true } }
      ),
    ])
    await mountView()
    bodyButtons().find((b) => b.title === 'Edit')!.click()
    await flushPromises()

    bodyButtons().find((b) => b.textContent?.trim() === 'Save')!.click()
    await flushPromises()

    const payload = (mocks.updateForm as any).mock.calls[0][1]
    expect(payload.settings.notifications).toEqual({
      admin: { enabled: false, to: [] },
      submitter: { enabled: true },
    })
    expect(payload.settings.tracking).toEqual({ enabled: true })
  })
})

describe('FormsView submissions modal', () => {
  function makeSubmission(over: Record<string, any> = {}) {
    return {
      ID: 'sub-1',
      FormID: 'form-1',
      ContextID: 'ctx-1',
      Data: { email: 'e@x.com', name: 'Ann', company: 'Acme' },
      Files: [] as any[],
      CreatedAt: '2026-01-02T10:00:00Z',
      ...over,
    }
  }

  function formWithFields() {
    return {
      ID: 'form-1',
      ContextID: 'ctx-1',
      Slug: 'contact-us',
      Name: 'Contact Us',
      Fields: [
        { slug: 'email', type: 'email', label: 'Email', required: true, validation: null, order: 0 },
        { slug: 'name', type: 'text', label: 'Full Name', required: false, validation: null, order: 1 },
        { slug: 'company', type: 'text', label: 'Company', required: false, validation: null, order: 2 },
      ],
      Settings: null,
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
    }
  }

  // The submissions modal is teleported; find its fixed wrapper via the title.
  function modalEl(): HTMLElement {
    const h3 = Array.from(document.body.querySelectorAll('h3')).find(
      (h) => h.textContent === 'Submissions'
    )
    if (!h3) throw new Error('Submissions modal not open')
    return h3.closest('div.fixed') as HTMLElement
  }

  async function openSubmissions() {
    mocks.listForms.mockResolvedValue([formWithFields()])
    await mountView()
    bodyButtons().find((b) => b.title === 'Submissions')!.click()
    await flushPromises()
  }

  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listForms.mockResolvedValue([])
    mocks.listSubmissions.mockResolvedValue({ data: [], total: 0, page: 1, limit: 20 })
  })

  it('shows dynamic columns from field slugs to labels with values', async () => {
    mocks.listSubmissions.mockResolvedValue({ data: [makeSubmission()], total: 1, page: 1, limit: 20 })
    await openSubmissions()

    const modal = modalEl()
    const headers = Array.from(modal.querySelectorAll('th')).map((th) => th.textContent)
    expect(headers).toEqual(['Email', 'Full Name', 'Company', 'Files', 'Created'])
    expect(modal.textContent).toContain('e@x.com')
    expect(modal.textContent).toContain('Ann')
    expect(modal.textContent).toContain('Acme')
  })

  it('shows an empty state when there are no submissions', async () => {
    await openSubmissions()
    expect(modalEl().textContent).toContain('No submissions yet.')
  })

  it('downloads attached files through the authenticated client', async () => {
    vi.stubGlobal('URL', { ...URL, createObjectURL: vi.fn(() => 'blob:mock'), revokeObjectURL: vi.fn() })
    try {
      mocks.listSubmissions.mockResolvedValue({
        data: [
          makeSubmission({
            Files: [{ field: 'attachment', name: 'a.pdf', size: 1, type: 'application/pdf', url: '/x' }],
          }),
        ],
        total: 1,
        page: 1,
        limit: 20,
      })
      await openSubmissions()

      const modal = modalEl()
      const fileBtn = Array.from(modal.querySelectorAll('button')).find((b) =>
        b.textContent?.includes('a.pdf')
      )!
      fileBtn.click()
      await flushPromises()

      expect(mocks.downloadSubmissionFile).toHaveBeenCalledWith('form-1', 'sub-1', 0)
    } finally {
      vi.unstubAllGlobals()
    }
  })

  it('paginates submissions (limit 20) with Prev/Next and a summary', async () => {
    mocks.listSubmissions.mockResolvedValue({ data: [makeSubmission()], total: 45, page: 1, limit: 20 })
    await openSubmissions()

    const modal = modalEl()
    expect(modal.textContent).toContain('Showing 1-20 of 45')

    const next = Array.from(modal.querySelectorAll('button')).find((b) =>
      b.textContent?.trim() === 'Next'
    )!
    next.click()
    await flushPromises()

    const lastCall = (mocks.listSubmissions as any).mock.calls.at(-1)
    expect(lastCall).toEqual(['form-1', 2, 20])
    expect(modalEl().textContent).toContain('Showing 21-40 of 45')
  })
})