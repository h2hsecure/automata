<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { Eye, MousePointerClick, Globe, Monitor, Smartphone, BarChart3, Users } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import { usePagination } from '@/composables/usePagination'
import { getDashboardStats, getTrackingEvents } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import type { TrackingEvent } from '@/types/api'

const authStore = useAuthStore()
const { page, limit, total, goToPage, setTotal } = usePagination()
const stats = ref({
  totalVisitors: 0,
  activeVisitors: 0,
  totalPageViews: 0,
})
const events = ref<TrackingEvent[]>([])
const loading = ref(false)
const chartLoading = ref(true)

const pageViewsChart = ref<Array<{ label: string; count: number }>>([])
const deviceBreakdown = ref<Array<{ label: string; count: number; pct: number }>>([])
const referrers = ref<Array<{ domain: string; count: number }>>([])

const maxPageViews = computed(() =>
  Math.max(1, ...pageViewsChart.value.map((item) => item.count))
)

const columns = [
  { key: 'Type', label: 'Type' },
  { key: 'URL', label: 'URL' },
  { key: 'EventName', label: 'Event' },
  { key: 'Referrer', label: 'Referrer' },
  { key: 'CreatedAt', label: 'Time' },
]

async function loadStats() {
  chartLoading.value = true
  try {
    const dashboard = await getDashboardStats()
    stats.value.totalVisitors = dashboard.total_visitors
    stats.value.activeVisitors = dashboard.active_visitors
    stats.value.totalPageViews = dashboard.page_views
    pageViewsChart.value = (dashboard.top_pages ?? []).map((p) => ({
      label: p.URL || '(unknown)',
      count: p.Count,
    }))
    const devices = dashboard.device_breakdown ?? {}
    const deviceTotal = Object.values(devices).reduce((a, b) => a + b, 0) || 1
    deviceBreakdown.value = Object.entries(devices).map(([label, count]) => ({
      label: label.charAt(0).toUpperCase() + label.slice(1),
      count,
      pct: Math.round((count / deviceTotal) * 100),
    }))
    referrers.value = (dashboard.top_referrers ?? []).map((r) => ({
      domain: r.Referrer || 'direct',
      count: r.Count,
    }))
  } finally {
    chartLoading.value = false
  }
}

async function loadEvents() {
  loading.value = true
  try {
    const response = await getTrackingEvents(page.value, limit.value)
    events.value = response.data || []
    setTotal(response.total || 0)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadStats()
  loadEvents()
})

watch(
  () => authStore.currentContextId,
  () => {
    page.value = 1
    loadStats()
    loadEvents()
  }
)
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold">Tracking</h1>
      <p class="text-muted-foreground">Visitor analytics and event tracking</p>
    </div>

    <!-- Stats -->
    <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
      </Card>
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
      </Card>
      <Card>
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-medium text-muted-foreground">Active Now</p>
            <p class="text-2xl font-bold">{{ stats.activeVisitors }}</p>
          </div>
          <div class="rounded-lg bg-amber-100 dark:bg-amber-900/30 p-3">
            <MousePointerClick class="h-6 w-6 text-amber-600 dark:text-amber-400" />
          </div>
        </div>
      </Card>
    </div>

    <!-- Charts Row -->
    <div class="grid gap-4 lg:grid-cols-3">
      <!-- Page Views Chart -->
      <Card class="lg:col-span-2" title="Page Views" description="Top pages by view count">
        <p v-if="!chartLoading && pageViewsChart.length === 0" class="mt-4 py-8 text-center text-sm text-muted-foreground">
          No page view data yet.
        </p>
        <div v-else-if="!chartLoading" class="mt-4 flex items-end gap-2 h-48">
          <div
            v-for="item in pageViewsChart"
            :key="item.label"
            class="flex-1 flex flex-col items-center gap-1 group"
          >
            <span class="text-xs font-medium opacity-0 group-hover:opacity-100 transition-opacity">
              {{ item.count }}
            </span>
            <div
              class="w-full rounded-t bg-primary opacity-80 hover:opacity-100 transition-all cursor-pointer"
              :style="{ height: `${(item.count / maxPageViews) * 100}%` }"
              :title="`${item.label}: ${item.count}`"
            />
            <span class="text-xs text-muted-foreground truncate w-full text-center">{{ item.label }}</span>
          </div>
        </div>
      </Card>

      <!-- Device Breakdown -->
      <Card title="Devices" description="Visitor device breakdown">
        <div class="mt-4 space-y-3">
          <p v-if="deviceBreakdown.length === 0" class="py-4 text-center text-sm text-muted-foreground">
            No device data yet.
          </p>
          <div
            v-for="device in deviceBreakdown"
            :key="device.label"
            class="space-y-1"
          >
            <div class="flex items-center justify-between text-sm">
              <span class="flex items-center gap-1.5">
                <Monitor v-if="device.label === 'Desktop'" class="h-3.5 w-3.5" />
                <Smartphone v-if="device.label === 'Mobile'" class="h-3.5 w-3.5" />
                <BarChart3 v-if="device.label === 'Tablet'" class="h-3.5 w-3.5" />
                {{ device.label }}
              </span>
              <span class="text-muted-foreground">{{ device.count }}</span>
            </div>
            <div class="h-2 w-full rounded-full bg-muted">
              <div
                class="h-2 rounded-full bg-primary"
                :style="{ width: `${device.pct}%` }"
              />
            </div>
          </div>
        </div>
      </Card>
    </div>

    <!-- Top Referrers -->
    <Card title="Top Referrers" description="Where your visitors come from">
      <div class="mt-4 space-y-3">
        <p v-if="referrers.length === 0" class="py-4 text-center text-sm text-muted-foreground">
          No referrer data yet.
        </p>
        <div
          v-for="(ref, index) in referrers"
          :key="ref.domain"
          class="flex items-center gap-3"
        >
          <span class="text-sm font-medium text-muted-foreground w-6">#{{ index + 1 }}</span>
          <Globe class="h-4 w-4 text-muted-foreground" />
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium truncate">{{ ref.domain }}</p>
          </div>
          <Badge variant="info">{{ ref.count }}</Badge>
        </div>
      </div>
    </Card>

    <!-- Events Log -->
    <Card title="Events Log" description="Recent tracking events">
      <DataTable
        :columns="columns"
        :data="events"
        :loading="loading"
        empty-text="No events found."
      >
        <template #Type="{ row }: { row: TrackingEvent }">
          <Badge :variant="row.Type === 'pageview' ? 'info' : 'success'">
            {{ row.Type }}
          </Badge>
        </template>
        <template #URL="{ row }: { row: TrackingEvent }">
          <span class="text-sm truncate block max-w-[200px]">{{ row.URL }}</span>
        </template>
        <template #EventName="{ row }: { row: TrackingEvent }">
          {{ row.EventName || '-' }}
        </template>
        <template #Referrer="{ row }: { row: TrackingEvent }">
          {{ row.Referrer || 'direct' }}
        </template>
        <template #CreatedAt="{ row }: { row: TrackingEvent }">
          {{ new Date(row.CreatedAt).toLocaleString() }}
        </template>
      </DataTable>

      <div
        v-if="total > 0"
        class="flex items-center justify-between border-t px-4 py-3"
      >
        <p class="text-sm text-muted-foreground">
          Showing {{ (page - 1) * limit + 1 }}-{{ Math.min(page * limit, total) }} of {{ total }}
        </p>
        <div class="flex gap-1">
          <Button variant="outline" size="sm" :disabled="page <= 1" @click="goToPage(page - 1)">
            Previous
          </Button>
          <Button variant="outline" size="sm" :disabled="page >= Math.ceil(total / limit)" @click="goToPage(page + 1)">
            Next
          </Button>
        </div>
      </div>
    </Card>
  </div>
</template>
