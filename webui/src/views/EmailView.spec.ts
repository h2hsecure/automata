import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import EmailView from './EmailView.vue'
import { useAuthStore } from '@/stores/auth'

const mocks = vi.hoisted(() => ({
  listEmailTemplates: vi.fn(async () => [] as any[]),
  createEmailTemplate: vi.fn(async (d: any) => d),
  updateEmailTemplate: vi.fn(async (_id: string, d: any) => d),
  deleteEmailTemplate: vi.fn(async () => {}),
  sendTemplateToContact: vi.fn(async () => ({ job_id: 'job-1' })),
  listInbox: vi.fn(async () => ({ messages: [], count: 0 })),
  markInboxRead: vi.fn(async () => {}),
  listForms: vi.fn(async () => [
    {
      ID: 'form-1',
      ContextID: 'ctx-1',
      Slug: 'contact-us',
      Name: 'Contact Us',
      Fields: [],
      Settings: null,
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
    },
  ]),
  listContacts: vi.fn(async () => ({ data: [], total: 0, page: 1, limit: 20 })),
}))

vi.mock('@/api/client', () => mocks)

let wrapper: VueWrapper

function bodyButtons() {
  return Array.from(document.body.querySelectorAll('button'))
}

function bodySelects() {
  return Array.from(document.body.querySelectorAll('select'))
}

async function mountView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  const auth = useAuthStore(pinia)
  auth.currentContextId = 'ctx-1'
  wrapper = mount(EmailView, {
    attachTo: document.body,
    global: { plugins: [pinia] },
  })
  await flushPromises()
  return wrapper
}

async function openCreateModal() {
  bodyButtons().find((b) => b.textContent?.includes('New Template'))!.click()
  await flushPromises()
}

// Set the kind select (the one offering standalone/form) to the given value.
async function setKind(kind: string) {
  const kindSelect = bodySelects().find((s) =>
    Array.from(s.options).some((o) => o.value === 'standalone')
  )!
  kindSelect.value = kind
  kindSelect.dispatchEvent(new Event('change'))
  await flushPromises()
}

function audienceSelect() {
  return bodySelects().find((s) => Array.from(s.options).some((o) => o.value === 'admin'))
}

async function fillModal() {
  // Text inputs in the modal, in order: key, subject.
  const inputs = Array.from(document.body.querySelectorAll('input[type="text"]'))
  const keyInput = inputs.find((i) => i.placeholder === 'welcome')!
  keyInput.value = 'form_reply'
  keyInput.dispatchEvent(new Event('input'))
  const subjectInput = inputs.find((i) => i.placeholder?.startsWith('Welcome'))!
  subjectInput.value = 'Re: your message'
  subjectInput.dispatchEvent(new Event('input'))
  await flushPromises()
}

afterEach(() => {
  wrapper?.unmount()
  document.body.innerHTML = ''
  vi.clearAllMocks()
  vi.resetAllMocks()
  mocks.listEmailTemplates.mockImplementation(async () => [])
  mocks.listForms.mockImplementation(async () => [
    {
      ID: 'form-1',
      ContextID: 'ctx-1',
      Slug: 'contact-us',
      Name: 'Contact Us',
      Fields: [],
      Settings: null,
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
    },
  ])
})

describe('EmailView template audience', () => {
  it('shows an audience picker for form-integrated templates, defaulting to submitter', async () => {
    await mountView()
    await openCreateModal()

    expect(audienceSelect()).toBeUndefined()

    await setKind('form')
    const select = audienceSelect()
    expect(select).toBeTruthy()
    expect(select!.value).toBe('submitter')
  })

  it('hides the audience picker for standalone templates', async () => {
    await mountView()
    await openCreateModal()
    await setKind('standalone')
    expect(audienceSelect()).toBeUndefined()
  })

  it('sends the chosen audience with the create payload for form templates', async () => {
    await mountView()
    await openCreateModal()
    await fillModal()
    await setKind('form')

    const formSelect = bodySelects().find((s) =>
      Array.from(s.options).some((o) => o.value === 'form-1')
    )!
    formSelect.value = 'form-1'
    formSelect.dispatchEvent(new Event('change'))
    await flushPromises()

    const select = audienceSelect()!
    select.value = 'admin'
    select.dispatchEvent(new Event('change'))
    await flushPromises()

    bodyButtons().find((b) => b.textContent?.trim() === 'Create')!.click()
    await flushPromises()

    expect(mocks.createEmailTemplate).toHaveBeenCalled()
    const payload = (mocks.createEmailTemplate as any).mock.calls[0][0]
    expect(payload.kind).toBe('form')
    expect(payload.form_id).toBe('form-1')
    expect(payload.audience).toBe('admin')
  })

  it('preselects the audience when editing a form template', async () => {
    mocks.listEmailTemplates.mockImplementation(async () => [
      {
        id: 'tpl-1',
        context_id: 'ctx-1',
        key: 'form_reply',
        subject: 'Re',
        body_text: 'b',
        is_default: false,
        kind: 'form',
        form_id: 'form-1',
        audience: 'admin',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ])
    await mountView()

    bodyButtons().find((b) => b.title === 'Edit')!.click()
    await flushPromises()

    expect(audienceSelect()!.value).toBe('admin')
  })
})