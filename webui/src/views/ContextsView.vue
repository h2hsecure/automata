<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Pencil, Trash2, Plus, Globe, Eye } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Modal from '@/components/Modal.vue'
import Input from '@/components/Input.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { useAuthStore } from '@/stores/auth'
import { listContexts, createContext, updateContext, deleteContext } from '@/api/client'
import type { Context } from '@/types/api'

const router = useRouter()
const authStore = useAuthStore()
const { confirmStore, show, confirm } = useConfirm()
const contexts = ref<Context[]>([])
const loading = ref(false)
const modalOpen = ref(false)
const isEdit = ref(false)
const editingId = ref('')
const contextName = ref('')
const contextSlug = ref('')
const contextDomain = ref('')
const contextActive = ref(true)
const formError = ref('')

const columns = [
  { key: 'Name', label: 'Name' },
  { key: 'Slug', label: 'Slug' },
  { key: 'Domain', label: 'Domain' },
  { key: 'IsActive', label: 'Status' },
  { key: 'CreatedAt', label: 'Created' },
  { key: 'actions', label: 'Actions', width: '140px' },
]

async function loadContexts() {
  loading.value = true
  try {
    contexts.value = await listContexts(0, 100)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  editingId.value = ''
  contextName.value = ''
  contextSlug.value = ''
  contextDomain.value = ''
  contextActive.value = true
  formError.value = ''
  modalOpen.value = true
}

function openEdit(c: Context) {
  isEdit.value = true
  editingId.value = c.ID
  contextName.value = c.Name
  contextSlug.value = c.Slug
  contextDomain.value = c.Domain || ''
  contextActive.value = c.IsActive
  formError.value = ''
  modalOpen.value = true
}

async function handleSave() {
  if (!contextName.value.trim()) {
    formError.value = 'Name is required.'
    return
  }
  if (!contextSlug.value.trim()) {
    formError.value = 'Slug is required.'
    return
  }
  loading.value = true
  try {
    const data = {
      name: contextName.value,
      slug: contextSlug.value,
      domain: contextDomain.value,
      is_active: contextActive.value,
    }
    if (isEdit.value) {
      await updateContext(editingId.value, data)
    } else {
      await createContext(data)
    }
    modalOpen.value = false
    await authStore.loadContexts()
    loadContexts()
  } catch (e: any) {
    formError.value = e.response?.data?.error || 'Failed to save website.'
  } finally {
    loading.value = false
  }
}

async function handleDelete(c: Context) {
  const confirmed = await show({
    title: 'Delete Website',
    description: `Are you sure you want to delete "${c.Name}"? All associated data will be lost.`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteContext(c.ID)
      await authStore.loadContexts()
      loadContexts()
    } catch {
      // handled
    }
  }
}

function viewContext(c: Context) {
  router.push(`/contexts/${c.ID}`)
}

onMounted(loadContexts)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <div class="flex items-center gap-2">
          <h1 class="text-2xl font-bold">Websites</h1>
          <Globe class="h-5 w-5 text-muted-foreground" />
        </div>
        <p class="text-muted-foreground">Manage your websites and their tracking settings</p>
      </div>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Add Website
      </Button>
    </div>

    <Card>
      <DataTable
        :columns="columns"
        :data="contexts"
        :loading="loading"
        empty-text="No websites found."
      >
        <template #Name="{ row }: { row: Context }">
          <span class="font-medium">{{ row.Name }}</span>
        </template>
        <template #Slug="{ row }: { row: Context }">
          <code class="text-sm bg-muted px-1.5 py-0.5 rounded">{{ row.Slug }}</code>
        </template>
        <template #Domain="{ row }: { row: Context }">
          {{ row.Domain || '-' }}
        </template>
        <template #IsActive="{ row }: { row: Context }">
          <Badge :variant="row.IsActive ? 'success' : 'destructive'">
            {{ row.IsActive ? 'Active' : 'Inactive' }}
          </Badge>
        </template>
        <template #CreatedAt="{ row }: { row: Context }">
          {{ new Date(row.CreatedAt).toLocaleDateString() }}
        </template>
        <template #actions="{ row }: { row: Context }">
          <div class="flex gap-1">
            <button
              class="p-1.5 rounded hover:bg-muted"
              title="View"
              @click="viewContext(row)"
            >
              <Eye class="h-4 w-4" />
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
    <Modal :open="modalOpen" :title="isEdit ? 'Edit Website' : 'Add Website'" @close="modalOpen = false">
      <div class="space-y-4">
        <Input v-model="contextName" label="Name" placeholder="My Website" />
        <Input v-model="contextSlug" label="Slug" placeholder="my-website" />
        <Input v-model="contextDomain" label="Domain" placeholder="example.com" />
        <div class="flex items-center gap-2">
          <input
            v-model="contextActive"
            type="checkbox"
            class="rounded border-gray-300"
          />
          <label class="text-sm font-medium">Active</label>
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