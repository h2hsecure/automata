<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
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
  ChevronRight,
} from 'lucide-vue-next'
import Card from '@/components/Card.vue'
import Badge from '@/components/Badge.vue'
import WorldMapChart from '@/components/WorldMapChart.vue'
import {
  listContacts,
  listForms,
  listBanners,
  getDashboardStats,
  getEmailStats,
} from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import { aggregateContacts, type CountryStat } from '@/lib/worldMap'
import type { Contact, EmailStats } from '@/types/api'

const authStore = useAuthStore()

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

// World map: contact locations from the last 90 days (includes hidden)
const MAP_WINDOW_DAYS = 90
const mapCountries = ref<CountryStat[]>([])
const mapLoading = ref(true)

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

    const contacts = await listContacts(1, 1, '', authStore.currentContextId, false)
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

// Fetches all contacts updated in the last 90 days (paginated) and
// aggregates them by country for the world map. Non-critical: an empty map
// is fine if the call fails.
async function loadMapData() {
  mapLoading.value = true
  const since = new Date(Date.now() - MAP_WINDOW_DAYS * 24 * 60 * 60 * 1000).toISOString()
  const pageSize = 100
  let page = 1
  let total = 0
  const collected: Contact[] = []
  try {
    do {
      const res = await listContacts(page, pageSize, '', authStore.currentContextId, undefined, since)
      collected.push(...res.data)
      total = res.total || 0
      page += 1
    } while (collected.length < total && page <= 50)
  } catch {
    return
  } finally {
    mapLoading.value = false
  }
  mapCountries.value = aggregateContacts(collected)
}

onMounted(() => {
  loadOverview()
  loadMapData()
})

watch(
  () => authStore.currentContextId,
  () => {
    loadOverview()
    loadMapData()
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
    </div>

    <!-- Stats Grid -->
      <div v-if="loading" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <div
          v-for="i in 5"
          :key="i"
          class="rounded-lg border border-border bg-card p-6"
        >
          <div class="animate-pulse space-y-3">
            <div class="h-3 w-24 rounded bg-muted" />
            <div class="h-7 w-16 rounded bg-muted" />
            <div class="h-3 w-32 rounded bg-muted" />
          </div>
        </div>
      </div>
      <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
          <router-link
            to="/forms"
            class="group -mx-1 -my-1 block rounded-lg p-1 transition hover:bg-primary/5 hover:ring-1 hover:ring-primary/30"
          >
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm font-medium text-muted-foreground">Forms</p>
                <p class="text-2xl font-bold">{{ stats.totalForms }}</p>
              </div>
              <div class="rounded-lg bg-amber-100 dark:bg-amber-900/30 p-3">
                <FileText class="h-6 w-6 text-amber-600 dark:text-amber-400" />
              </div>
            </div>
            <div class="mt-2 flex items-center justify-between text-sm text-muted-foreground">
              Active forms
              <ChevronRight
                class="h-4 w-4 opacity-40 transition group-hover:translate-x-0.5 group-hover:opacity-100"
              />
            </div>
          </router-link>
        </Card>

        <!-- Banners -->
        <Card>
          <router-link
            to="/banners"
            class="group -mx-1 -my-1 block rounded-lg p-1 transition hover:bg-primary/5 hover:ring-1 hover:ring-primary/30"
          >
            <div class="flex items-center justify-between">
              <div>
                <p class="text-sm font-medium text-muted-foreground">Banners</p>
                <p class="text-2xl font-bold">{{ stats.totalBanners }}</p>
              </div>
              <div class="rounded-lg bg-purple-100 dark:bg-purple-900/30 p-3">
                <Image class="h-6 w-6 text-purple-600 dark:text-purple-400" />
              </div>
            </div>
            <div class="mt-2 flex items-center justify-between text-sm text-muted-foreground">
              Active banners
              <ChevronRight
                class="h-4 w-4 opacity-40 transition group-hover:translate-x-0.5 group-hover:opacity-100"
              />
            </div>
          </router-link>
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

      <!-- Contact Locations Map -->
      <Card title="Contact Locations" description="Countries with contact activity in the last 90 days">
        <p
          v-if="mapLoading"
          class="py-8 text-center text-sm text-muted-foreground"
        >
          Loading…
        </p>
        <p
          v-else-if="mapCountries.length === 0"
          class="py-8 text-center text-sm text-muted-foreground"
        >
          No geo data for the last 90 days.
        </p>
        <WorldMapChart v-else :countries="mapCountries" />
      </Card>

      <!-- Charts Row -->
      <div class="grid gap-4 lg:grid-cols-3">
        <!-- Top Pages -->
        <Card class="lg:col-span-2" title="Top Pages" description="Most visited pages by view count">
          <div class="mt-4 space-y-3">
            <p v-if="loading" class="py-4 text-center text-sm text-muted-foreground">
              Loading…
            </p>
            <p v-else-if="pageViewsChart.length === 0" class="py-4 text-center text-sm text-muted-foreground">
              No page view data yet.
            </p>
            <div
              v-for="(page, index) in pageViewsChart"
              :key="page.label"
              class="flex items-center gap-3"
            >
              <span class="w-8 text-sm font-medium text-muted-foreground">
                #{{ index + 1 }}
              </span>
              <div class="flex-1 min-w-0">
                <p class="text-sm font-medium truncate" :title="page.label">{{ page.label }}</p>
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

        <!-- Device Breakdown -->
        <Card title="Devices" description="Visitor device breakdown">
          <div class="mt-4 space-y-3">
            <p v-if="loading" class="py-4 text-center text-sm text-muted-foreground">
              Loading…
            </p>
            <p v-else-if="deviceBreakdown.length === 0" class="py-4 text-center text-sm text-muted-foreground">
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
          <p v-if="loading" class="py-4 text-center text-sm text-muted-foreground">
            Loading…
          </p>
          <p v-else-if="referrers.length === 0" class="py-4 text-center text-sm text-muted-foreground">
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
  </div>
</template>