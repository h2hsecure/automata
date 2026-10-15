<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Copy, Check, Globe } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import Badge from '@/components/Badge.vue'
import { getContext, listForms, listBanners } from '@/api/client'
import type { Context, Form, Banner } from '@/types/api'

const route = useRoute()
const router = useRouter()
const context = ref<Context | null>(null)
const forms = ref<Form[]>([])
const banners = ref<Banner[]>([])
const loading = ref(true)
const error = ref('')
const copied = ref(false)

const contextId = computed(() => String(route.params.id || ''))

const snippetTag = computed(() =>
  context.value ? `<script src="/snippet/${context.value.ID}.js"><\/script>` : ''
)

async function load() {
  loading.value = true
  error.value = ''
  try {
    context.value = await getContext(contextId.value)
    const [f, b] = await Promise.all([
      listForms(contextId.value),
      listBanners(contextId.value),
    ])
    forms.value = f
    banners.value = b
  } catch (e: any) {
    error.value = e.response?.data?.error || 'Failed to load website.'
  } finally {
    loading.value = false
  }
}

async function copySnippet() {
  if (!snippetTag.value) return
  try {
    await navigator.clipboard.writeText(snippetTag.value)
    copied.value = true
    setTimeout(() => {
      copied.value = false
    }, 2000)
  } catch {
    // clipboard unavailable
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center gap-3">
      <Button variant="outline" size="sm" @click="router.push('/contexts')">
        <ArrowLeft class="mr-2 h-4 w-4" />
        Back
      </Button>
      <div class="flex items-center gap-2">
        <h1 class="text-2xl font-bold">{{ context?.Name || 'Website' }}</h1>
        <Globe class="h-5 w-5 text-muted-foreground" />
      </div>
    </div>

    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>

    <div v-else-if="context">
      <!-- Details -->
      <Card title="Details">
        <div class="mt-4 grid gap-4 sm:grid-cols-2">
          <div>
            <p class="text-sm text-muted-foreground">ID</p>
            <code class="text-sm">{{ context.ID }}</code>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Slug</p>
            <code class="text-sm">{{ context.Slug }}</code>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Domain</p>
            <p class="text-sm">{{ context.Domain || '-' }}</p>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Status</p>
            <Badge :variant="context.IsActive ? 'success' : 'destructive'">
              {{ context.IsActive ? 'Active' : 'Inactive' }}
            </Badge>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Created</p>
            <p class="text-sm">{{ new Date(context.CreatedAt).toLocaleString() }}</p>
          </div>
          <div>
            <p class="text-sm text-muted-foreground">Updated</p>
            <p class="text-sm">{{ new Date(context.UpdatedAt).toLocaleString() }}</p>
          </div>
        </div>
      </Card>

      <!-- Tracking snippet -->
      <Card title="Tracking Snippet" description="Add this script tag to every page of your website, just before the closing body tag.">
        <div class="mt-4 space-y-3">
          <div class="flex items-center gap-2">
            <code class="flex-1 overflow-x-auto rounded-md bg-muted px-3 py-2 text-sm">{{ snippetTag }}</code>
            <Button variant="outline" size="sm" @click="copySnippet">
              <Check v-if="copied" class="mr-2 h-4 w-4" />
              <Copy v-else class="mr-2 h-4 w-4" />
              {{ copied ? 'Copied' : 'Copy' }}
            </Button>
          </div>
          <p class="text-sm text-muted-foreground">
            The relative path works when the page is served through the Automata proxy
            (e.g. a per-website ingress route). For any other site, use the full URL,
            for example <code class="text-sm">http://your-host/snippet/{{ context.ID }}.js</code>.
          </p>
        </div>
      </Card>

      <!-- Forms -->
      <Card title="Forms" description="Forms associated with this website">
        <div class="mt-4 space-y-3">
          <p v-if="forms.length === 0" class="py-4 text-center text-sm text-muted-foreground">
            No forms yet.
          </p>
          <div
            v-for="form in forms"
            :key="form.ID"
            class="flex items-center justify-between rounded-md border px-3 py-2"
          >
            <div>
              <p class="text-sm font-medium">{{ form.Name }}</p>
              <code class="text-xs text-muted-foreground">{{ form.ID }}</code>
            </div>
            <code class="text-xs bg-muted px-1.5 py-0.5 rounded">{{ form.Slug }}</code>
          </div>
        </div>
      </Card>

      <!-- Banners -->
      <Card title="Banners" description="Banners associated with this website">
        <div class="mt-4 space-y-3">
          <p v-if="banners.length === 0" class="py-4 text-center text-sm text-muted-foreground">
            No banners yet.
          </p>
          <div
            v-for="banner in banners"
            :key="banner.ID"
            class="flex items-center justify-between rounded-md border px-3 py-2"
          >
            <div>
              <p class="text-sm font-medium">{{ banner.Name }}</p>
              <code class="text-xs text-muted-foreground">{{ banner.ID }}</code>
            </div>
            <Badge :variant="banner.IsActive ? 'success' : 'destructive'">
              {{ banner.IsActive ? 'Active' : 'Inactive' }}
            </Badge>
          </div>
        </div>
      </Card>
    </div>
  </div>
</template>