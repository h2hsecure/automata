<script setup lang="ts">
import { ref, computed, onMounted, type Component } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  ArrowLeft,
  FileText,
  Globe,
  MousePointerClick,
  Eye,
  UserPlus,
  UserCog,
  GitMerge,
  Tag,
  Mail,
} from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import Badge from '@/components/Badge.vue'
import { getContact, getContactActivity } from '@/api/client'
import type { Contact, Activity } from '@/types/api'

const route = useRoute()
const router = useRouter()

const contact = ref<Contact | null>(null)
const activities = ref<Activity[]>([])
const total = ref(0)
const loading = ref(true)
const loadingMore = ref(false)
const error = ref('')
const notFound = ref(false)
const typeFilter = ref('')

const PAGE_SIZE = 50

const contactId = computed(() => String(route.params.id || ''))

interface ActivityMeta {
  label: string
  icon: Component
  classes: string
}

const ACTIVITY_META: Record<string, ActivityMeta> = {
  form_submission: { label: 'Form submission', icon: FileText, classes: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400' },
  page_visit: { label: 'Page visit', icon: Globe, classes: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-400' },
  banner_click: { label: 'Banner click', icon: MousePointerClick, classes: 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400' },
  banner_impression: { label: 'Banner impression', icon: Eye, classes: 'bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400' },
  contact_created: { label: 'Contact created', icon: UserPlus, classes: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  contact_updated: { label: 'Contact updated', icon: UserCog, classes: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400' },
  contact_merged: { label: 'Contact merged', icon: GitMerge, classes: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400' },
  tag_added: { label: 'Tag added', icon: Tag, classes: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400' },
  tag_removed: { label: 'Tag removed', icon: Tag, classes: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-400' },
  email_sent: { label: 'Email sent', icon: Mail, classes: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400' },
}

const FILTER_OPTIONS = Object.entries(ACTIVITY_META)

function metaFor(a: Activity): ActivityMeta {
  return ACTIVITY_META[a.type] ?? {
    label: a.type,
    icon: Globe,
    classes: 'bg-muted text-muted-foreground',
  }
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  return isNaN(d.getTime()) ? '' : d.toLocaleString()
}

async function loadActivities() {
  try {
    const res = await getContactActivity(
      contactId.value,
      0,
      PAGE_SIZE,
      typeFilter.value || undefined
    )
    activities.value = res.activities
    total.value = res.total
  } catch (e: any) {
    error.value = e.response?.data?.error || 'Failed to load activities.'
  }
}

async function loadMore() {
  loadingMore.value = true
  try {
    const res = await getContactActivity(
      contactId.value,
      activities.value.length,
      PAGE_SIZE,
      typeFilter.value || undefined
    )
    activities.value = [...activities.value, ...res.activities]
    total.value = res.total
  } finally {
    loadingMore.value = false
  }
}

function onFilterChange() {
  loadActivities()
}

async function load() {
  loading.value = true
  notFound.value = false
  error.value = ''
  try {
    contact.value = await getContact(contactId.value)
  } catch (e: any) {
    if (e.response?.status === 404) {
      notFound.value = true
    } else {
      error.value = e.response?.data?.error || 'Failed to load contact.'
    }
  } finally {
    loading.value = false
  }
  if (contact.value) {
    await loadActivities()
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center gap-3">
      <Button variant="outline" size="sm" @click="router.push('/contacts')">
        <ArrowLeft class="mr-2 h-4 w-4" />
        Back
      </Button>
      <h1 class="text-2xl font-bold">
        {{ contact ? `${contact.FirstName} ${contact.LastName}`.trim() || contact.Email : 'Contact' }}
      </h1>
      <Badge v-if="contact?.Hidden" variant="warning">Hidden visitor</Badge>
    </div>

    <p v-if="notFound" class="text-sm text-muted-foreground">
      Contact not found.
      <Button variant="outline" size="sm" @click="router.push('/contacts')">Back to contacts</Button>
    </p>
    <p v-else-if="error && !contact" class="text-sm text-destructive">{{ error }}</p>

    <div v-else-if="contact" class="space-y-6">
      <!-- Contact details -->
      <Card title="Details">
        <div class="mt-4 grid gap-4 sm:grid-cols-2">
          <div>
            <p class="text-sm text-muted-foreground">Email</p>
            <p class="text-sm">{{ contact.Email || '-' }}</p>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Phone</p>
            <p class="text-sm">{{ contact.Phone || '-' }}</p>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Company</p>
            <p class="text-sm">{{ contact.Company || '-' }}</p>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Source</p>
            <p class="text-sm">{{ contact.Source || '-' }}</p>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Tags</p>
            <div class="flex gap-1 flex-wrap">
              <Badge
                v-for="tag in contact.Tags"
                :key="tag.ID"
                variant="default"
              >
                {{ tag.Name }}
              </Badge>
              <span v-if="!contact.Tags?.length" class="text-sm text-muted-foreground">-</span>
            </div>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Created</p>
            <p class="text-sm">{{ formatTime(contact.CreatedAt) }}</p>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Updated</p>
            <p class="text-sm">{{ formatTime(contact.UpdatedAt) }}</p>
          </div>
        </div>
      </Card>

      <!-- Visitor client info (hidden contacts enriched from page visits) -->
      <Card v-if="contact.UserAgent" title="Visitor">
        <div class="mt-4 grid gap-4 sm:grid-cols-2">
          <div>
            <p class="text-sm text-muted-foreground">Device</p>
            <p class="text-sm">{{ contact.DeviceType || '-' }}</p>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Browser</p>
            <p class="text-sm">{{ contact.Browser || '-' }}</p>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">IP address</p>
            <p class="text-sm">{{ contact.IPAddress || '-' }}</p>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Location</p>
            <p class="text-sm">
              {{ [contact.GeoCity, contact.GeoRegion, contact.GeoCountry].filter(Boolean).join(', ') || '-' }}
            </p>
          </div>
          <div class="sm:col-span-2">
            <p class="text-sm text-muted-foreground">User agent</p>
            <p class="text-sm break-all">{{ contact.UserAgent || '-' }}</p>
          </div>
        </div>
      </Card>

      <!-- Activity feed -->
      <Card title="Activity">
        <div class="mt-4 flex items-center justify-between gap-4">
          <p class="text-sm text-muted-foreground">
            {{ total }} activity{{ total === 1 ? '' : 'ies' }}
          </p>
          <select
            v-model="typeFilter"
            class="text-sm rounded-md border border-input bg-background px-3 py-1.5"
            @change="onFilterChange"
          >
            <option value="">All activity types</option>
            <option v-for="[value, meta] in FILTER_OPTIONS" :key="value" :value="value">
              {{ meta.label }}
            </option>
          </select>
        </div>

        <div v-if="activities.length" class="mt-4">
          <div
            v-for="(a, i) in activities"
            :key="a.id"
            class="flex gap-3"
          >
            <div class="flex flex-col items-center">
              <div
                class="h-8 w-8 rounded-full flex items-center justify-center shrink-0"
                :class="metaFor(a).classes"
              >
                <component :is="metaFor(a).icon" class="h-4 w-4" />
              </div>
              <div v-if="i < activities.length - 1" class="w-px flex-1 bg-border" />
            </div>
            <div class="flex-1 pb-5">
              <div class="flex items-baseline justify-between gap-2">
                <p class="text-sm font-medium">{{ a.title }}</p>
                <p class="text-xs text-muted-foreground whitespace-nowrap">
                  {{ formatTime(a.created_at) }}
                </p>
              </div>
              <p
                v-if="a.description && a.description !== a.title"
                class="text-sm text-muted-foreground"
              >
                {{ a.description }}
              </p>
            </div>
          </div>
        </div>
        <p v-else class="mt-4 text-sm text-muted-foreground">
          No activities recorded yet.
        </p>

        <div v-if="activities.length < total" class="mt-2 flex justify-center">
          <Button
            variant="outline"
            size="sm"
            :loading="loadingMore"
            @click="loadMore"
          >
            Load more ({{ total - activities.length }} remaining)
          </Button>
        </div>
      </Card>
    </div>
  </div>
</template>