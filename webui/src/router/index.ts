import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/LoginView.vue'),
    meta: { guest: true },
  },
  {
    path: '/onboarding',
    name: 'Onboarding',
    component: () => import('@/views/OnboardingView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/',
    component: () => import('@/components/AppLayout.vue'),
    meta: { requiresAuth: true },
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/DashboardView.vue'),
      },
      {
        path: 'contacts/new',
        name: 'AddContact',
        component: () => import('@/views/AddContactView.vue'),
      },
      {
        path: 'contacts',
        name: 'Contacts',
        component: () => import('@/views/ContactsView.vue'),
      },
      {
        path: 'contacts/:id',
        name: 'ContactDetail',
        component: () => import('@/views/ContactDetailView.vue'),
      },
      {
        path: 'forms',
        name: 'Forms',
        component: () => import('@/views/FormsView.vue'),
      },
      {
        path: 'static-files',
        name: 'StaticFiles',
        component: () => import('@/views/StaticFilesView.vue'),
      },
      {
        path: 'email',
        name: 'Email',
        component: () => import('@/views/EmailView.vue'),
        meta: { requiresAdmin: true },
      },
      {
        path: 'api-keys',
        redirect: '/settings?tab=api-keys',
      },
      {
        path: 'banners',
        name: 'Banners',
        component: () => import('@/views/BannersView.vue'),
      },
      {
        path: 'contexts',
        redirect: '/settings?tab=websites',
      },
      {
        path: 'contexts/:id',
        name: 'ContextDetail',
        component: () => import('@/views/ContextDetailView.vue'),
        meta: { requiresAdmin: true },
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/SettingsView.vue'),
        meta: { requiresAdmin: true },
      },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/dashboard',
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to, from, next) => {
  const token = localStorage.getItem('token')

  if (to.meta.requiresAuth && !token) {
    next({ name: 'Login', query: { redirect: to.fullPath } })
    return
  }
  if (to.meta.guest && token) {
    next({ name: 'Dashboard' })
    return
  }

  // Onboarding: an authenticated user with no contexts must create a first
  // context before reaching any app route.
  if (token && to.meta.requiresAuth && to.name !== 'Onboarding') {
    try {
      const { useAuthStore } = await import('@/stores/auth')
      const authStore = useAuthStore()
      if (authStore.user) {
        // Store already initialized in this session (SPA navigation).
        if (authStore.contexts.length === 0) {
          next({ name: 'Onboarding' })
          return
        }
      } else {
        // Fresh page load: verify against the API before the layout's init().
        const { listContexts } = await import('@/api/client')
        const list = await listContexts()
        if (list.length === 0) {
          next({ name: 'Onboarding' })
          return
        }
        authStore.contexts = list.map((c) => ({ id: c.ID, slug: c.Slug, name: c.Name }))
        localStorage.setItem('contexts', JSON.stringify(authStore.contexts))
        if (!authStore.contexts.some((c) => c.id === authStore.currentContextId)) {
          authStore.currentContextId = list[0].ID
          localStorage.setItem('context_id', list[0].ID)
        }
      }
    } catch {
      // API unreachable: fall through to normal navigation.
    }
  }

  next()
})

export default router
