import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import StaticFilesView from './StaticFilesView.vue'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/api/client', () => ({
  listStaticFiles: vi.fn(async () => [
    {
      ID: '1',
      ContextID: 'ctx-1',
      Name: 'logo.png',
      Token: 'ctx-1/abc.png',
      Mime: 'image/png',
      Size: 1234,
      CreatedAt: '2026-01-01T00:00:00Z',
    },
  ]),
  uploadStaticFile: vi.fn(async () => ({})),
  deleteStaticFile: vi.fn(async () => {}),
}))

import { listStaticFiles, uploadStaticFile, deleteStaticFile } from '@/api/client'

async function mountView() {
  const pinia = createPinia()
  setActivePinia(pinia)
  const auth = useAuthStore(pinia)
  auth.currentContextId = 'ctx-1'
  const wrapper = mount(StaticFilesView, {
    global: { plugins: [pinia] },
  })
  await flushPromises()
  return wrapper
}

describe('StaticFilesView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('lists static files with their public URL', async () => {
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('logo.png')
    expect(wrapper.text()).toContain('/static/ctx-1/logo.png')
  })

  it('uploads a file with its URL name', async () => {
    const wrapper = await mountView()

    const fileInput = wrapper.find('input[type="file"]')
    expect(fileInput.exists()).toBe(true)
    const file = new File(['hello'], 'logo.png', { type: 'image/png' })
    const dt = new DataTransfer()
    dt.items.add(file)
    fileInput.element.files = dt.files
    await fileInput.trigger('change')

    // Name field defaults to the chosen file name (or set explicitly).
    const inputs = wrapper.findAll('input[type="text"]')
    await inputs[0].setValue('logo')

    const uploadBtn = wrapper
      .findAll('button')
      .find((b) => b.text().includes('Upload'))!
    await uploadBtn.trigger('click')
    await flushPromises()

    expect(uploadStaticFile).toHaveBeenCalledTimes(1)
    const [uploaded, name] = (uploadStaticFile as any).mock.calls[0]
    expect(uploaded.name).toBe('logo.png')
    expect(name).toBe('logo')
  })

  it('shows a delete action for each file', async () => {
    const wrapper = await mountView()
    const deleteBtn = wrapper
      .findAll('button')
      .find((b) => b.attributes('title') === 'Delete')
    expect(deleteBtn).toBeTruthy()
  })
})