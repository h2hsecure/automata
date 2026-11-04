import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import router from '@/router'
import { useAuthStore } from '@/stores/auth'

// seedStore puts the auth store in a post-login state (sessionChecked=true
// so the guard skips the getMe() round-trip) and records the given admin
// flag. A context is always seeded so the onboarding redirect (no contexts)
// never masks the assertion.
function seedStore(isAdmin: boolean) {
  const store = useAuthStore()
  store.user = {
    id: 'u1',
    email: 'admin@example.com',
    is_admin: isAdmin,
    created_at: '2026-01-01T00:00:00Z',
  }
  store.contexts = [{ id: 'c1', slug: 'site', name: 'Site' }]
  store.currentContextId = 'c1'
  store.sessionChecked = true
}

describe('router admin guard', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  it('redirects a non-admin away from a requiresAdmin route to the dashboard', async () => {
    seedStore(false)
    await router.push('/settings')
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('lets an admin through to a requiresAdmin route', async () => {
    seedStore(true)
    await router.push('/settings')
    expect(router.currentRoute.value.path).toBe('/settings')
  })
})