<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { RouterLink } from 'vue-router'
import {
  Users,
  FileText,
  Image,
  Eye,
  MousePointerClick,
  Clock,
  Globe,
  Monitor,
  Smartphone,
  BarChart3,
  Mail,
  Send,
} from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import {
  listContacts,
  listForms,
  listBanners,
  getDashboardStats,
  getTrackingEvents,
  listVisitors,
  getEmailStats,
} from '@/api/client'
import { usePagination } from '@/composables/usePagination'
import { useAuthStore } from '@/stores/auth'
import type { TrackingEvent, Visitor, EmailStats } from '@/types/api'

const authStore = useAuthStore()
const tab = ref<'overview' | 'events' | 'visitors'>('overview')
const tabs = [
  { id: 'overview', label: 'Overview' },
  { id: 'events', label: 'Events' },
  { id: 'visitors', label: 'Visitors' },
] as const

const loading = ref(true)
const stats = ref({
  totalVisitors: 0,
  activeVisitors: 0,
  totalPageViews: 0,
  totalContacts: 0,
  totalForms: 0,
  totalBanners: 0,
})

// Email stats (queue status + reach) — cards hide if the call fails
const emailStats = ref<EmailStats | null>(null)
const emailStatsLoaded = ref(false)

// Real page view data (top pages by view count)
const pageViewsChart = ref<Array<{ label: string; count: number }>>([])
const deviceBreakdown = ref<Array<{ label: string; count: number; pct: number }>>([])
const referrers = ref<Array<{ domain: string; count: number }>>([])

const maxPageViews = computed(() =>
  Math.max(1, ...pageViewsChart.value.map((item) => item.count))
)

// Events tab
const events = ref<TrackingEvent[]>([])
const eventsLoading = ref(false)
const eventsPagination = usePagination()
const {
  page: eventsPage,
  limit: eventsLimit,
  total: eventsTotal,
  goToPage: eventsGoToPage,
  setTotal: setEventsTotal,
} = eventsPagination

const eventColumns = [
  { key: 'Type', label: 'Type' },
  { key: 'URL', label: 'URL' },
  { key: 'EventName', label: 'Event' },
  { key: 'Referrer', label: 'Referrer' },
  { key: 'CreatedAt', label: 'Time' },
]

// Visitors tab
const visitors = ref<Visitor[]>([])
const visitorsLoading = ref(false)
const visitorsPagination = usePagination()
const {
  page: visitorsPage,
  limit: visitorsLimit,
  total: visitorsTotal,
  goToPage: visitorsGoToPage,
  setTotal: setVisitorsTotal,
} = visitorsPagination

const visitorColumns = [
  { key: 'ID', label: 'ID' },
  { key: 'FirstSeen', label: 'First Seen' },
  { key: 'LastSeen', label: 'Last Seen' },
  { key: 'PageViews', label: 'Page Views' },
  { key: 'LastIP', label: 'Last IP' },
  { key: 'ContactID', label: 'Identified' },
]

async function loadOverview() {
  loading.value = true
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

    const contacts = await listContacts(1, 1, '', authStore.currentContextId)
    stats.value.totalContacts = contacts.total || 0

    const forms = await listForms(authStore.currentContextId)
    stats.value.totalForms = forms.length

    const banners = await listBanners(authStore.currentContextId)
    stats.value.totalBanners = banners.length

    // Non-blocking: hide email cards on failure
    getEmailStats()
      .then((s) => {
        emailStats.value = s
        emailStatsLoaded.value = true
      })
      .catch(() => {
        emailStatsLoaded.value = false
      })
  } finally {
    loading.value = false
  }
}

async function loadEvents() {
  eventsLoading.value = true
  try {
    const response = await getTrackingEvents(eventsPage.value, eventsLimit.value)
    events.value = response.data || []
    setEventsTotal(response.total || 0)
  } finally {
    eventsLoading.value = false
  }
}

async function loadVisitors() {
  visitorsLoading.value = true
  try {
    const response = await listVisitors(visitorsPage.value, visitorsLimit.value)
    visitors.value = response.data || []
    setVisitorsTotal(response.total || 0)
  } finally {
    visitorsLoading.value = false
  }
}

function loadActiveTab() {
  if (tab.value === 'overview') loadOverview()
  else if (tab.value === 'events') loadEvents()
  else loadVisitors()
}

onMounted(loadActiveTab)

watch(tab, loadActiveTab)

watch(
  () => authStore.currentContextId,
  () => {
    eventsPagination.reset()
    visitorsPagination.reset()
    loadActiveTab()
  }
)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold">Dashboard</h1>
        <p class="text-muted-foreground">Overview of your marketing automation platform</p>
      </div>
      <div class="flex gap-1 rounded-lg bg-muted p-1">
        <button
          v-for="t in tabs"
          :key="t.id"
          :class="[
            'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
            tab === t.id
              ? 'bg-card text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground',
          ]"
          @click="tab = t.id"
        >
          {{ t.label }}
        </button>
      </div>
    </div>

    <!-- Overview tab -->
    <template v-if="tab === 'overview'">
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

        <!-- Email Queue -->
        <Card v-if="emailStatsLoaded && emailStats">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm font-medium text-muted-foreground">Email Queue</p>
              <p class="text-2xl font-bold">{{ emailStats.pending }}</p>
            </div>
            <div class="rounded-lg bg-rose-100 dark:bg-rose-900/30 p-3">
              <Mail class="h-6 w-6 text-rose-600 dark:text-rose-400" />
            </div>
          </div>
          <div class="mt-2 text-sm text-muted-foreground">
            pending · {{ emailStats.sent_24h }} sent, {{ emailStats.failed_24h }} failed (24h)
          </div>
        </Card>

        <!-- Email Reach -->
        <Card v-if="emailStatsLoaded && emailStats">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm font-medium text-muted-foreground">Email Reach</p>
              <p class="text-2xl font-bold">{{ emailStats.contacts_reached }}</p>
            </div>
            <div class="rounded-lg bg-teal-100 dark:bg-teal-900/30 p-3">
              <Send class="h-6 w-6 text-teal-600 dark:text-teal-400" />
            </div>
          </div>
          <div class="mt-2 text-sm text-muted-foreground">
            contacts reached · {{ emailStats.reached_7d }} in 7d · {{ emailStats.emails_sent_total }} emails total
          </div>
        </Card>
      </div>

      <!-- Charts Row -->
      <div class="grid gap-4 lg:grid-cols-3">
        <!-- Page Views Chart -->
        <Card class="lg:col-span-2" title="Page Views" description="Top pages by view count">
          <p v-if="!loading && pageViewsChart.length === 0" class="mt-4 py-8 text-center text-sm text-muted-foreground">
            No page view data yet.
          </p>
          <div v-else-if="!loading" class="mt-4 flex items-end gap-2 h-48">
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

      <!-- Top Pages -->
      <Card title="Top Pages" description="Most visited pages">
        <div class="mt-4 space-y-3">
          <p v-if="pageViewsChart.length === 0" class="py-4 text-center text-sm text-muted-foreground">
            No page view data yet.
          </p>
          <div
            v-for="(page, index) in pageViewsChart"
            :key="page.label"
            class="flex items-center gap-3"
          >
            <span class="text-sm font-medium text-muted-foreground w-8">
              #{{ index + 1 }}
            </span>
            <div class="flex-1 min-w-0">
              <p class="text-sm font-medium truncate">{{ page.label }}</p>
              <div class="mt-1 h-2 w-full rounded-full bg-muted">
                <div
                  class="h-2 rounded-full bg-primary"
                  :style="{ width: `${(page.count / maxPageViews) * 100}%` }"
                />
              </div>
            </div>
            <Badge variant="info">{{ page.count }}</Badge>
          </div>
        </div>
      </Card>

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
    </template>

    <!-- Events tab -->
    <Card v-else-if="tab === 'events'" title="Events Log" description="Recent tracking events">
      <DataTable
        :columns="eventColumns"
        :data="events"
        :loading="eventsLoading"
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
        v-if="eventsTotal > 0"
        class="flex items-center justify-between border-t px-4 py-3"
      >
        <p class="text-sm text-muted-foreground">
          Showing {{ (eventsPage - 1) * eventsLimit + 1 }}-{{ Math.min(eventsPage * eventsLimit, eventsTotal) }} of {{ eventsTotal }}
        </p>
        <div class="flex gap-1">
          <Button variant="outline" size="sm" :disabled="eventsPage <= 1" @click="eventsGoToPage(eventsPage - 1)">
            Previous
          </Button>
          <Button variant="outline" size="sm" :disabled="eventsPage >= Math.ceil(eventsTotal / eventsLimit)" @click="eventsGoToPage(eventsPage + 1)">
            Next
          </Button>
        </div>
      </div>
    </Card>

    <!-- Visitors tab -->
    <Card v-else title="Visitors" description="Tracked visitors">
      <DataTable
        :columns="visitorColumns"
        :data="visitors"
        :loading="visitorsLoading"
        empty-text="No visitors found."
      >
        <template #ID="{ row }: { row: Visitor }">
          <span class="font-mono text-xs" :title="row.ID">{{ row.ID.slice(0, 8) }}</span>
        </template>
        <template #FirstSeen="{ row }: { row: Visitor }">
          {{ new Date(row.FirstSeen).toLocaleString() }}
        </template>
        <template #LastSeen="{ row }: { row: Visitor }">
          {{ new Date(row.LastSeen).toLocaleString() }}
        </template>
        <template #PageViews="{ row }: { row: Visitor }">
          {{ row.PageViews }}
        </template>
        <template #LastIP="{ row }: { row: Visitor }">
          {{ row.LastIP || '-' }}
        </template>
        <template #ContactID="{ row }: { row: Visitor }">
          <RouterLink v-if="row.ContactID" to="/contacts" class="hover:opacity-80 transition-opacity">
            <Badge variant="success">Identified</Badge>
          </RouterLink>
          <Badge v-else variant="default">Anonymous</Badge>
        </template>
      </DataTable>

      <div
        v-if="visitorsTotal > 0"
        class="flex items-center justify-between border-t px-4 py-3"
      >
        <p class="text-sm text-muted-foreground">
          Showing {{ (visitorsPage - 1) * visitorsLimit + 1 }}-{{ Math.min(visitorsPage * visitorsLimit, visitorsTotal) }} of {{ visitorsTotal }}
        </p>
        <div class="flex gap-1">
          <Button variant="outline" size="sm" :disabled="visitorsPage <= 1" @click="visitorsGoToPage(visitorsPage - 1)">
            Previous
          </Button>
          <Button variant="outline" size="sm" :disabled="visitorsPage >= Math.ceil(visitorsTotal / visitorsLimit)" @click="visitorsGoToPage(visitorsPage + 1)">
            Next
          </Button>
        </div>
      </div>
    </Card>
  </div>
</template>