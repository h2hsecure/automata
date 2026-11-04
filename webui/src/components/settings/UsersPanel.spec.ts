import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import UsersPanel from './UsersPanel.vue'

const mocks = vi.hoisted(() => ({
  listUsers: vi.fn(async () => [
    { id: 'u1', email: 'a@example.com', is_admin: false, created_at: '2026-01-01T00:00:00Z' },
  ]),
  deleteUser: vi.fn(async () => {}),
  listContexts: vi.fn(async () => []),
  listContextUsers: vi.fn(async () => []),
  createUser: vi.fn(async () => {}),
  updateUser: vi.fn(async () => {}),
  assignContextUser: vi.fn(async () => {}),
  removeContextUser: vi.fn(async () => {}),
}))

vi.mock('@/api/client', () => mocks)
vi.mock('@/composables/useConfirm', () => ({
  useConfirm: () => ({
    // ConfirmDialog (child) also imports useConfirm and reads confirmStore
    confirmStore: { value: { show: false, options: null } },
    show: vi.fn(async () => true),
    confirm: vi.fn(),
    cancel: vi.fn(),
  }),
}))

describe('UsersPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('surfaces the server error when delete fails', async () => {
    mocks.deleteUser.mockRejectedValueOnce({
      response: { data: { error: 'user has active sessions' } },
    })
    const wrapper = mount(UsersPanel)
    await flushPromises()

    const deleteButton = wrapper.find('button[title="Delete"]')
    expect(deleteButton.exists()).toBe(true)
    await deleteButton.trigger('click')
    await flushPromises()

    expect(mocks.deleteUser).toHaveBeenCalledWith('u1')
    expect(wrapper.text()).toContain('user has active sessions')
  })

  it('falls back to a generic message when the server error is empty', async () => {
    mocks.deleteUser.mockRejectedValueOnce(new Error('network down'))
    const wrapper = mount(UsersPanel)
    await flushPromises()

    await wrapper.find('button[title="Delete"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Failed to delete user.')
  })
})