<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Pencil, Trash2, Plus, BarChart3 } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Modal from '@/components/Modal.vue'
import Input from '@/components/Input.vue'
import Textarea from '@/components/Textarea.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { useAuthStore } from '@/stores/auth'
import { listPages, createPage, updatePage, deletePage, listPageDecisions } from '@/api/client'
import type { Page, PageInput, PageButton, PageDecision } from '@/types/api'

const authStore = useAuthStore()
const { confirmStore, show, confirm } = useConfirm()
const pages = ref<Page[]>([])
const loading = ref(true)
const modalOpen = ref(false)
const isEdit = ref(false)
const editingId = ref('')
const pageSlug = ref('')
const pageTitle = ref('')
const pageContent = ref('')
const pageStatus = ref('active')
const pagePosition = ref('')
const pageButtons = ref<PageButton[]>([])
const formError = ref('')

// Decisions modal state
const decisionsModalOpen = ref(false)
const decisionsPage = ref<Page | null>(null)
const decisions = ref<PageDecision[]>([])
const decisionCounts = ref<Record<string, number>>({})
const decisionsLoading = ref(false)

const decisionColumns = [
  { key: 'visitor_id', label: 'Visitor' },
  { key: 'action', label: 'Action' },
  { key: 'ip_hash', label: 'IP Hash' },
  { key: 'created_at', label: 'Decided At' },
]

const columns = [
  { key: 'slug', label: 'Slug' },
  { key: 'title', label: 'Title' },
  { key: 'url', label: 'Public URL' },
  { key: 'position', label: 'Position' },
  { key: 'buttons', label: 'Buttons' },
  { key: 'status', label: 'Status' },
  { key: 'updated_at', label: 'Updated' },
  { key: 'actions', label: 'Actions', width: '130px' },
]

async function loadPages() {
  loading.value = true
  try {
    pages.value = await listPages()
  } finally {
    loading.value = false
  }
}

watch(
  () => authStore.currentContextId,
  () => loadPages()
)

function publicURL(p: Page) {
  return `/page/${authStore.currentContextId}/${p.Slug}`
}

function addButtonRow() {
  pageButtons.value.push({ label: '', action: 'link', href: '', variant: 'primary' })
}

function removeButtonRow(index: number) {
  pageButtons.value.splice(index, 1)
}

async function openDecisions(p: Page) {
  decisionsPage.value = p
  decisionsModalOpen.value = true
  decisionsLoading.value = true
  decisions.value = []
  decisionCounts.value = {}
  try {
    const data = await listPageDecisions(p.ID)
    decisions.value = data.decisions ?? []
    decisionCounts.value = data.counts ?? {}
  } finally {
    decisionsLoading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  editingId.value = ''
  pageSlug.value = ''
  pageTitle.value = ''
  pageContent.value = ''
  pageStatus.value = 'active'
  pagePosition.value = ''
  pageButtons.value = []
  formError.value = ''
  modalOpen.value = true
}

function openEdit(p: Page) {
  isEdit.value = true
  editingId.value = p.ID
  pageSlug.value = p.Slug
  pageTitle.value = p.Title
  pageContent.value = p.Content
  pageStatus.value = p.IsActive ? 'active' : 'inactive'
  pagePosition.value = p.Position ?? ''
  pageButtons.value = (p.Buttons ?? []).map((btn) => ({ ...btn }))
  formError.value = ''
  modalOpen.value = true
}

async function handleSave() {
  if (!pageSlug.value.trim()) {
    formError.value = 'Slug is required.'
    return
  }
  if (!pageTitle.value.trim()) {
    formError.value = 'Title is required.'
    return
  }
  loading.value = true
  try {
    const data: PageInput = {
      slug: pageSlug.value,
      title: pageTitle.value,
      content: pageContent.value,
      is_active: pageStatus.value === 'active',
      position: pagePosition.value,
      buttons: pageButtons.value.filter((b) => b.label.trim() || b.action),
    }
    if (isEdit.value) {
      await updatePage(editingId.value, data)
    } else {
      await createPage(data)
    }
    modalOpen.value = false
    loadPages()
  } catch (e: any) {
    formError.value = e.response?.data?.error || 'Failed to save page.'
  } finally {
    loading.value = false
  }
}

async function handleDelete(p: Page) {
  const confirmed = await show({
    title: 'Delete Page',
    description: `Are you sure you want to delete "${p.Title}"?`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deletePage(p.ID)
      loadPages()
    } catch {
      // handled
    }
  }
}

onMounted(loadPages)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">Pages</h1>
        <p class="text-muted-foreground">Editable HTML pages — set a position to render as a site overlay</p>
      </div>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Create Page
      </Button>
    </div>

    <Card>
      <DataTable
        :columns="columns"
        :data="pages"
        :loading="loading"
        empty-text="No pages found."
      >
        <template #slug="{ row }: { row: Page }">
          <span class="font-medium">{{ row.Slug }}</span>
        </template>
        <template #title="{ row }: { row: Page }">
          {{ row.Title }}
        </template>
        <template #url="{ row }: { row: Page }">
          <code class="text-xs bg-muted px-1.5 py-0.5 rounded">{{ publicURL(row) }}</code>
        </template>
        <template #position="{ row }: { row: Page }">
          <Badge v-if="row.Position" variant="info">{{ row.Position }}</Badge>
          <span v-else class="text-muted-foreground">—</span>
        </template>
        <template #buttons="{ row }: { row: Page }">
          {{ row.Buttons?.length || 0 }} buttons
        </template>
        <template #status="{ row }: { row: Page }">
          <Badge :variant="row.IsActive ? 'success' : 'default'">
            {{ row.IsActive ? 'active' : 'inactive' }}
          </Badge>
        </template>
        <template #updated_at="{ row }: { row: Page }">
          {{ new Date(row.UpdatedAt).toLocaleDateString() }}
        </template>
        <template #actions="{ row }: { row: Page }">
          <div class="flex gap-1">
            <button
              class="p-1.5 rounded hover:bg-muted"
              title="Decisions"
              aria-label="View decisions"
              @click="openDecisions(row)"
            >
              <BarChart3 class="h-4 w-4" />
            </button>
            <button
              class="p-1.5 rounded hover:bg-muted"
              title="Edit"
              @click="openEdit(row)"
            >
              <Pencil class="h-4 w-4" />
            </button>
            <button
              class="p-1.5 rounded hover:bg-muted text-destructive"
              title="Delete"
              @click="handleDelete(row)"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </div>
        </template>
      </DataTable>
    </Card>

    <!-- Modal -->
    <Modal :open="modalOpen" :title="isEdit ? 'Edit Page' : 'Create Page'" @close="modalOpen = false">
      <div class="space-y-4">
        <Input v-model="pageSlug" label="Slug" placeholder="cookie-policy" />
        <Input v-model="pageTitle" label="Title" placeholder="Cookie Policy" />

        <div>
          <label class="text-sm font-medium mb-1 block">Position</label>
          <select
            v-model="pagePosition"
            class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
          >
            <option value="">None (regular page)</option>
            <option value="top">Top bar</option>
            <option value="bottom">Bottom bar</option>
            <option value="left">Left strip</option>
            <option value="right">Right strip</option>
            <option value="full">Full overlay</option>
          </select>
        </div>

        <div>
          <label class="text-sm font-medium mb-1 block">
            Buttons
            <span class="text-slate-400 font-normal">(optional — rendered with positioned pages)</span>
          </label>
          <div class="space-y-2">
            <div v-for="(btn, i) in pageButtons" :key="i" class="grid grid-cols-12 gap-2 items-end">
              <div class="col-span-4"><Input v-model="btn.label" placeholder="Accept" :label="i === 0 ? 'Label' : ''" /></div>
              <div class="col-span-3">
                <select
                  v-model="btn.action"
                  class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                >
                  <option v-if="i === 0" hidden>Action</option>
                  <option value="link">Link</option>
                  <option value="dismiss_accept">Dismiss (accept)</option>
                  <option value="dismiss_decline">Dismiss (decline)</option>
                </select>
              </div>
              <div class="col-span-3"><Input v-model="btn.href" placeholder="/privacy" :label="i === 0 ? 'Href (link action)' : ''" /></div>
              <div class="col-span-2 flex items-center gap-2">
                <select
                  v-model="btn.variant"
                  class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                >
                  <option v-if="i === 0" hidden>Style</option>
                  <option value="primary">Primary</option>
                  <option value="secondary">Secondary</option>
                </select>
                <button class="p-1.5 rounded hover:bg-muted text-destructive" @click="removeButtonRow(i)" :aria-label="'Remove button ' + (i + 1)">
                  <Trash2 class="h-4 w-4" />
                </button>
              </div>
            </div>
            <Button variant="ghost" size="sm" @click="addButtonRow">
              <Plus class="mr-1 h-4 w-4" /> Add button
            </Button>
          </div>
        </div>

        <Textarea
          v-model="pageContent"
          label="Content"
          :rows="12"
          placeholder="HTML content..."
        />

        <div>
          <label class="text-sm font-medium mb-1 block">Status</label>
          <select
            v-model="pageStatus"
            class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
          >
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
          </select>
        </div>

        <p v-if="formError" class="text-sm text-destructive">{{ formError }}</p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <Button variant="outline" @click="modalOpen = false">Cancel</Button>
          <Button :loading="loading" @click="handleSave">
            {{ isEdit ? 'Save' : 'Create' }}
          </Button>
        </div>
      </template>
    </Modal>

    <!-- Decisions modal -->
    <Modal :open="decisionsModalOpen" :title="'Decisions — ' + (decisionsPage?.Title ?? '')" @close="decisionsModalOpen = false">
      <div class="space-y-4">
        <div v-if="decisionsLoading" class="text-sm text-muted-foreground py-4 text-center">Loading decisions…</div>
        <template v-else>
          <div class="flex flex-wrap gap-2">
            <Badge v-for="(count, action) in decisionCounts" :key="action" variant="info">
              {{ action }}: {{ count }}
            </Badge>
            <span v-if="Object.keys(decisionCounts).length === 0" class="text-sm text-muted-foreground">
              No decisions recorded yet.
            </span>
          </div>
          <DataTable
            :columns="decisionColumns"
            :data="decisions"
            empty-text="No decisions yet."
          >
            <template #visitor_id="{ row }: { row: PageDecision }">
              <code class="text-xs">{{ row.VisitorID }}</code>
            </template>
            <template #action="{ row }: { row: PageDecision }">
              <Badge variant="default">{{ row.Action }}</Badge>
            </template>
            <template #ip_hash="{ row }: { row: PageDecision }">
              <code class="text-xs text-muted-foreground">{{ row.IPHash }}</code>
            </template>
            <template #created_at="{ row }: { row: PageDecision }">
              {{ new Date(row.CreatedAt).toLocaleString() }}
            </template>
          </DataTable>
        </template>
      </div>
    </Modal>

    <ConfirmDialog />
  </div>
</template>