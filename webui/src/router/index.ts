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
        path: 'contacts',
        name: 'Contacts',
        component: () => import('@/views/ContactsView.vue'),
      },
      {
        path: 'tenants',
        name: 'Tenants',
        component: () => import('@/views/TenantsView.vue'),
        meta: { requiresAdmin: true },
      },
      {
        path: 'forms',
        name: 'Forms',
        component: () => import('@/views/FormsView.vue'),
      },
      {
        path: 'api-keys',
        name: 'ApiKeys',
        component: () => import('@/views/ApiKeysView.vue'),
        meta: { requiresAdmin: true },
      },
      {
        path: 'banners',
        name: 'Banners',
        component: () => import('@/views/BannersView.vue'),
      },
      {
        path: 'tracking',
        name: 'Tracking',
        component: () => import('@/views/TrackingView.vue'),
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
  } else if (to.meta.guest && token) {
    next({ name: 'Dashboard' })
  } else {
    next()
  }
})

export default router
