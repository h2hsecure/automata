import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import FormsView from './FormsView.vue'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/api/client', () => ({
  listForms: vi.fn(async () => []),
  createForm: vi.fn(async () => ({})),
  updateForm: vi.fn(async () => ({})),
  deleteForm: vi.fn(async () => {}),
}))

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

    const rows = document.body.querySelectorAll('div.rounded-md.border')
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