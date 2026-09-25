<script setup lang="ts">
import { ref, computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import {
  LayoutDashboard,
  Users,
  FileText,
  Image,
  Key,
  Settings,
  BarChart3,
  Menu,
  X,
  Zap,
} from 'lucide-vue-next'

interface NavItem {
  name: string
  path: string
  icon: any
  adminOnly?: boolean
}

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const route = useRoute()
const { useAuthStore } = await import('@/stores/auth')
const authStore = useAuthStore()

const navItems: NavItem[] = [
  { name: 'Dashboard', path: '/dashboard', icon: LayoutDashboard },
  { name: 'Contacts', path: '/contacts', icon: Users },
  { name: 'Forms', path: '/forms', icon: FileText },
  { name: 'Banners', path: '/banners', icon: Image },
  { name: 'Tracking', path: '/tracking', icon: BarChart3 },
  { name: 'API Keys', path: '/api-keys', icon: Key, adminOnly: true },
  { name: 'Tenants', path: '/tenants', icon: Settings, adminOnly: true },
]

const filteredItems = computed(() =>
  navItems.filter((item) => !item.adminOnly || authStore.isAdmin)
)

const isActive = (path: string) => route.path === path
</script>

<template>
  <!-- Mobile overlay -->
  <div
    v-if="!props.open"
    class="fixed inset-0 z-40 bg-black/50 lg:hidden"
    @click="emit('update:open', false)"
  />

  <!-- Sidebar -->
  <aside
    :class="[
      'fixed top-0 left-0 z-50 flex h-full flex-col bg-card border-r transition-all duration-200',
      'w-64',
      { 'translate-x-[-100%]': !props.open && window.innerWidth < 1024 },
    ]"
    :style="{ width: props.open ? '16rem' : '4rem' }"
  >
    <!-- Logo -->
    <div
      :class="[
        'flex h-16 items-center border-b px-4',
        { 'justify-center': !props.open },
      ]"
    >
      <Zap
        :class="[
          'h-6 w-6 text-primary flex-shrink-0',
          { 'mx-auto': !props.open },
        ]"
      />
      <span
        v-if="props.open"
        class="ml-2 font-bold text-lg whitespace-nowrap"
      >
        Automata
      </span>
      <button
        v-if="props.open"
        class="ml-auto p-1 hover:bg-muted rounded"
        @click="emit('update:open', false)"
      >
        <Menu class="h-4 w-4" />
      </button>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 overflow-y-auto py-4">
      <ul class="space-y-1 px-2">
        <li v-for="item in filteredItems" :key="item.path">
          <RouterLink
            :to="item.path"
            :class="[
              'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
              isActive(item.path)
                ? 'bg-primary text-primary-foreground'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground',
              { 'justify-center': !props.open },
            ]"
          >
            <component
              :is="item.icon"
              class="h-4 w-4 flex-shrink-0"
            />
            <span v-if="props.open" class="whitespace-nowrap">{{
              item.name
            }}</span>
          </RouterLink>
        </li>
      </ul>
    </nav>

    <!-- Footer -->
    <div
      v-if="props.open"
      class="border-t px-4 py-3 text-xs text-muted-foreground"
    >
      Automata v0.1.0
    </div>
  </aside>
</template>
