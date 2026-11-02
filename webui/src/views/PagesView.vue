<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Pencil, Trash2, Plus } from 'lucide-vue-next'
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
import { listPages, createPage, updatePage, deletePage } from '@/api/client'
import type { Page, PageInput } from '@/types/api'

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
const formError = ref('')

const columns = [
  { key: 'slug', label: 'Slug' },
  { key: 'title', label: 'Title' },
  { key: 'url', label: 'Public URL' },
  { key: 'status', label: 'Status' },
  { key: 'updated_at', label: 'Updated' },
  { key: 'actions', label: 'Actions', width: '100px' },
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

function openCreate() {
  isEdit.value = false
  editingId.value = ''
  pageSlug.value = ''
  pageTitle.value = ''
  pageContent.value = ''
  pageStatus.value = 'active'
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
        <p class="text-muted-foreground">Editable HTML pages served publicly, linkable from banner buttons</p>
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

    <ConfirmDialog />
  </div>
</template>