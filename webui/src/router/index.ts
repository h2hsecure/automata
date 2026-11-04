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
        path: 'forms/:id/submissions',
        name: 'FormSubmissions',
        component: () => import('@/views/FormSubmissionsView.vue'),
      },
      {
        path: 'static-files',
        name: 'StaticFiles',
        component: () => import('@/views/StaticFilesView.vue'),
      },
      {
        path: 'pages',
        name: 'Pages',
        component: () => import('@/views/PagesView.vue'),
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
  const { useAuthStore } = await import('@/stores/auth')
  const authStore = useAuthStore()

  // Cookie-only auth: the session lives in an HttpOnly cookie the browser
  // can't read, so verify it against the API once before guarding.
  if (!authStore.sessionChecked) {
    await authStore.restoreSession()
  }

  const authed = authStore.isAuthenticated
  if (to.meta.requiresAuth && !authed) {
    next({ name: 'Login', query: { redirect: to.fullPath } })
    return
  }
  if (to.meta.guest && authed) {
    next({ name: 'Dashboard' })
    return
  }

  // Onboarding: an authenticated user with no contexts must create a first
  // context before reaching any app route.
  if (authed && to.meta.requiresAuth && to.name !== 'Onboarding' && authStore.contexts.length === 0) {
    next({ name: 'Onboarding' })
    return
  }

  next()
})

export default router
