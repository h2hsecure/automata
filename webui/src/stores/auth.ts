import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as loginApi, logout as logoutApi, getMe, listContexts } from '@/api/client'
import type { User, LoginResponse } from '@/types/api'

interface Context {
  id: string
  slug: string
  name: string
}

interface AuthState {
  user: User | null
  contexts: Context[]
  currentContextId: string
}

// Cookie-only auth: the API JWT lives in an HttpOnly session cookie managed
// by the BFF, never in the browser. The SPA cannot read the cookie, so it
// verifies the session by calling getMe() and tracks that with sessionChecked.
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const contexts = ref<Context[]>([])
  const currentContextId = ref(localStorage.getItem('context_id') || '')
  const loading = ref(false)
  const error = ref<string | null>(null)
  const theme = ref(localStorage.getItem('theme') || 'light')
  // True once the session has been verified (or found invalid) this session.
  const sessionChecked = ref(false)

  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.is_admin ?? false)
  const currentContext = computed(() => contexts.value.find((c) => c.id === currentContextId.value) || null)

  function applyContexts(list: Context[]) {
    contexts.value = list
    localStorage.setItem('contexts', JSON.stringify(list))
    if (!list.some((c) => c.id === currentContextId.value)) {
      currentContextId.value = list.length > 0 ? list[0].id : ''
      localStorage.setItem('context_id', currentContextId.value)
    }
  }

  async function login(email: string, password: string) {
    loading.value = true
    error.value = null
    try {
      // The BFF intercepts this response, stores the JWT in the session
      // cookie, and strips it from the body — `response.token` is absent.
      const response: LoginResponse = await loginApi(email, password)

      user.value = {
        id: response.user_id,
        email: response.email,
        is_admin: response.is_admin || false,
        created_at: '',
      }
      if (response.contexts && response.contexts.length > 0) {
        applyContexts(response.contexts)
      }
      sessionChecked.value = true
      return true
    } catch (e: any) {
      error.value = e.response?.data?.error || 'Login failed. Please check your credentials.'
      throw e
    } finally {
      loading.value = false
    }
  }

  async function logout() {
    try {
      await logoutApi()
    } catch {
      // Ignore logout errors
    } finally {
      user.value = null
      contexts.value = []
      currentContextId.value = ''
      sessionChecked.value = true
      localStorage.removeItem('context_id')
      localStorage.removeItem('contexts')
    }
  }

  // restoreSession verifies the cookie-backed session against the API and
  // populates user + contexts. Idempotent: runs at most once per session.
  async function restoreSession() {
    if (sessionChecked.value) return
    try {
      const me = await getMe()
      user.value = {
        id: me.id,
        email: me.email,
        is_admin: me.is_admin,
        created_at: me.created_at,
      }
      await loadContexts()
    } catch {
      user.value = null
      contexts.value = []
    } finally {
      sessionChecked.value = true
    }
  }

  // init is kept for the AppLayout onMounted hook; it just restores the
  // session (no-op if already checked by the router guard).
  async function init() {
    await restoreSession()
  }

  async function loadContexts() {
    try {
      const list = await listContexts()
      if (list.length > 0) {
        applyContexts(list.map((c) => ({ id: c.ID, slug: c.Slug, name: c.Name })))
        return
      }
    } catch {
      // fall back to saved contexts below
    }
    const savedContexts = localStorage.getItem('contexts')
    if (savedContexts) {
      contexts.value = JSON.parse(savedContexts)
    }
  }

  function switchContext(contextId: string) {
    currentContextId.value = contextId
    localStorage.setItem('context_id', contextId)
  }

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
    localStorage.setItem('theme', theme.value)
  }

  return {
    user,
    contexts,
    currentContextId,
    loading,
    error,
    theme,
    sessionChecked,
    isAuthenticated,
    isAdmin,
    currentContext,
    login,
    logout,
    init,
    restoreSession,
    loadContexts,
    switchContext,
  }
})