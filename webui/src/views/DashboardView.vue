<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  Users,
  FileText,
  Image,
  BarChart3,
  TrendingUp,
  Eye,
  MousePointerClick,
  Clock,
} from 'lucide-vue-next'
import Card from '@/components/Card.vue'
import Badge from '@/components/Badge.vue'
import { listContacts, listForms, listBanners, getDashboardStats } from '@/api/client'

const loading = ref(true)
const stats = ref({
  totalVisitors: 0,
  activeVisitors: 0,
  totalPageViews: 0,
  totalContacts: 0,
  totalForms: 0,
  totalBanners: 0,
})

// Simple bar chart data for page views
const pageViews = ref<Array<{ label: string; count: number }>>([
  { label: 'Mon', count: 120 },
  { label: 'Tue', count: 190 },
  { label: 'Wed', count: 150 },
  { label: 'Thu', count: 210 },
  { label: 'Fri', count: 175 },
  { label: 'Sat', count: 90 },
  { label: 'Sun', count: 65 },
])

const topPages = ref<Array<{ path: string; views: number }>>([
  { path: '/home', views: 342 },
  { path: '/pricing', views: 218 },
  { path: '/about', views: 156 },
  { path: '/blog', views: 134 },
  { path: '/contact', views: 89 },
])

async function loadStats() {
  try {
    loading.value = true
    const dashboard = await getDashboardStats()
    stats.value.totalVisitors = dashboard.total_visitors
    stats.value.activeVisitors = dashboard.active_visitors
    stats.value.totalPageViews = dashboard.total_page_views

    const contacts = await listContacts()
    stats.value.totalContacts = contacts.total || 0

    const forms = await listForms()
    stats.value.totalForms = forms.length

    const banners = await listBanners()
    stats.value.totalBanners = banners.length
  } finally {
    loading.value = false
  }
}

onMounted(loadStats)
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold">Dashboard</h1>
      <p class="text-muted-foreground">Overview of your marketing automation platform</p>
    </div>

    <!-- Stats Grid -->
    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <!-- Visitors -->
      <Card>
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-medium text-muted-foreground">Total Visitors</p>
            <p class="text-2xl font-bold">{{ stats.totalVisitors }}</p>
          </div>
          <div class="rounded-lg bg-primary/10 p-3">
            <Users class="h-6 w-6 text-primary" />
          </div>
        </div>
        <div class="mt-2 flex items-center text-sm text-muted-foreground">
          <Clock class="mr-1 h-3 w-3" />
          {{ stats.activeVisitors }} active now
        </div>
      </Card>

      <!-- Page Views -->
      <Card>
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-medium text-muted-foreground">Page Views</p>
            <p class="text-2xl font-bold">{{ stats.totalPageViews }}</p>
          </div>
          <div class="rounded-lg bg-emerald-100 dark:bg-emerald-900/30 p-3">
            <Eye class="h-6 w-6 text-emerald-600 dark:text-emerald-400" />
          </div>
        </div>
        <div class="mt-2 flex items-center text-sm text-emerald-600">
          <TrendingUp class="mr-1 h-3 w-3" />
          +12.5% from last week
        </div>
      </Card>

      <!-- Contacts -->
      <Card>
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-medium text-muted-foreground">Contacts</p>
            <p class="text-2xl font-bold">{{ stats.totalContacts }}</p>
          </div>
          <div class="rounded-lg bg-blue-100 dark:bg-blue-900/30 p-3">
            <MousePointerClick class="h-6 w-6 text-blue-600 dark:text-blue-400" />
          </div>
        </div>
        <div class="mt-2 text-sm text-muted-foreground">
          Tracked contacts
        </div>
      </Card>

      <!-- Forms -->
      <Card>
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-medium text-muted-foreground">Forms</p>
            <p class="text-2xl font-bold">{{ stats.totalForms }}</p>
          </div>
          <div class="rounded-lg bg-amber-100 dark:bg-amber-900/30 p-3">
            <FileText class="h-6 w-6 text-amber-600 dark:text-amber-400" />
          </div>
        </div>
        <div class="mt-2 text-sm text-muted-foreground">
          Active forms
        </div>
      </Card>

      <!-- Banners -->
      <Card>
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-medium text-muted-foreground">Banners</p>
            <p class="text-2xl font-bold">{{ stats.totalBanners }}</p>
          </div>
          <div class="rounded-lg bg-purple-100 dark:bg-purple-900/30 p-3">
            <Image class="h-6 w-6 text-purple-600 dark:text-purple-400" />
          </div>
        </div>
        <div class="mt-2 text-sm text-muted-foreground">
          Active banners
        </div>
      </Card>

      <!-- CTR -->
      <Card>
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-medium text-muted-foreground">Avg. CTR</p>
            <p class="text-2xl font-bold">3.2%</p>
          </div>
          <div class="rounded-lg bg-emerald-100 dark:bg-emerald-900/30 p-3">
            <TrendingUp class="h-6 w-6 text-emerald-600 dark:text-emerald-400" />
          </div>
        </div>
        <div class="mt-2 text-sm text-emerald-600">
          +0.4% from last week
        </div>
      </Card>
    </div>

    <!-- Charts Row -->
    <div class="grid gap-4 lg:grid-cols-2">
      <!-- Page Views Chart -->
      <Card title="Page Views (This Week)" description="Daily page view counts">
        <div class="mt-4 flex items-end gap-2 h-48">
          <div
            v-for="item in pageViews"
            :key="item.label"
            class="flex-1 flex flex-col items-center gap-1"
          >
            <span class="text-xs font-medium">{{ item.count }}</span>
            <div
              class="w-full rounded-t bg-primary opacity-80 hover:opacity-100 transition-opacity"
              :style="{ height: `${(item.count / 210) * 100}%` }"
            />
            <span class="text-xs text-muted-foreground">{{ item.label }}</span>
          </div>
        </div>
      </Card>

      <!-- Top Pages -->
      <Card title="Top Pages" description="Most visited pages">
        <div class="mt-4 space-y-3">
          <div
            v-for="(page, index) in topPages"
            :key="index"
            class="flex items-center gap-3"
          >
            <span class="text-sm font-medium text-muted-foreground w-8">
              #{{ index + 1 }}
            </span>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium truncate">{{ page.path }}</p>
              <div class="mt-1 h-2 w-full rounded-full bg-muted">
                <div
                  class="h-2 rounded-full bg-primary"
                  :style="{ width: `${(page.views / topPages[0].views) * 100}%` }"
                />
              </div>
            </div>
            <Badge variant="info">{{ page.views }}</Badge>
          </div>
        </div>
      </Card>
    </div>
  </div>
</template>
