import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { login as loginApi, logout as logoutApi, getMe } from '@/api/client'
import type { User, LoginResponse } from '@/types/api'

interface Context {
  id: string
  slug: string
  name: string
}

interface AuthState {
  token: string | null
  user: User | null
  contexts: Context[]
  currentContextId: string
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('token'))
  const user = ref<User | null>(null)
  const contexts = ref<Context[]>([])
  const currentContextId = ref(localStorage.getItem('context_id') || '')
  const loading = ref(false)
  const error = ref<string | null>(null)
  const theme = ref(localStorage.getItem('theme') || 'light')

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.is_admin ?? false)
  const currentContext = computed(() => contexts.value.find((c) => c.id === currentContextId.value) || null)

  async function login(email: string, password: string, tenant?: string) {
    loading.value = true
    error.value = null
    try {
      const response = await loginApi(email, password, tenant)
      token.value = response.token
      localStorage.setItem('token', response.token)

      if (response.user_id) {
        user.value = {
          id: response.user_id,
          email: response.email,
          is_admin: response.is_admin || false,
          tenant_id: response.tenant_id || '',
        }
      }

      if (response.contexts && response.contexts.length > 0) {
        contexts.value = response.contexts
        if (!currentContextId.value) {
          currentContextId.value = response.contexts[0].id
          localStorage.setItem('context_id', response.contexts[0].id)
        }
      }

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
      if (token.value) await logoutApi()
    } catch {
      // Ignore logout errors
    } finally {
      token.value = null
      user.value = null
      contexts.value = []
      currentContextId.value = ''
      localStorage.removeItem('token')
      localStorage.removeItem('context_id')
      localStorage.removeItem('user')
    }
  }

  async function init() {
    if (!token.value) return
    try {
      user.value = await getMe()
      // Load contexts from localStorage or API
      const savedContexts = localStorage.getItem('contexts')
      if (savedContexts) {
        contexts.value = JSON.parse(savedContexts)
      }
    } catch {
      await logout()
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
    token,
    user,
    contexts,
    currentContextId,
    loading,
    error,
    theme,
    isAuthenticated,
    isAdmin,
    currentContext,
    login,
    logout,
    init,
    switchContext,
  }
})
