<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Pencil, Trash2, Plus, Shield } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Modal from '@/components/Modal.vue'
import Input from '@/components/Input.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { usePagination } from '@/composables/usePagination'
import { listTenants, createTenant, updateTenant, deleteTenant } from '@/api/client'
import type { Tenant } from '@/types/api'

const { confirmStore, show, confirm } = useConfirm()
const { page, limit, total, goToPage, setTotal } = usePagination()
const tenants = ref<Tenant[]>([])
const loading = ref(false)
const modalOpen = ref(false)
const isEdit = ref(false)
const editingId = ref('')
const tenantName = ref('')
const tenantSlug = ref('')
const tenantDomain = ref('')
const tenantActive = ref(true)
const formError = ref('')

const columns = [
  { key: 'name', label: 'Name' },
  { key: 'slug', label: 'Slug' },
  { key: 'domain', label: 'Domain' },
  { key: 'status', label: 'Status' },
  { key: 'created_at', label: 'Created' },
  { key: 'actions', label: 'Actions', width: '100px' },
]

async function loadTenants() {
  loading.value = true
  try {
    const response = await listTenants(page.value, limit.value)
    tenants.value = response.data || []
    setTotal(response.total || 0)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  editingId.value = ''
  tenantName.value = ''
  tenantSlug.value = ''
  tenantDomain.value = ''
  tenantActive.value = true
  formError.value = ''
  modalOpen.value = true
}

function openEdit(t: Tenant) {
  isEdit.value = true
  editingId.value = t.id
  tenantName.value = t.name
  tenantSlug.value = t.slug
  tenantDomain.value = t.domain || ''
  tenantActive.value = t.active
  formError.value = ''
  modalOpen.value = true
}

async function handleSave() {
  if (!tenantName.value.trim()) {
    formError.value = 'Name is required.'
    return
  }
  if (!tenantSlug.value.trim()) {
    formError.value = 'Slug is required.'
    return
  }
  loading.value = true
  try {
    const data = {
      name: tenantName.value,
      slug: tenantSlug.value,
      domain: tenantDomain.value,
      active: tenantActive.value,
    }
    if (isEdit.value) {
      await updateTenant(editingId.value, data)
    } else {
      await createTenant(data)
    }
    modalOpen.value = false
    loadTenants()
  } catch (e: any) {
    formError.value = e.response?.data?.error || 'Failed to save tenant.'
  } finally {
    loading.value = false
  }
}

async function handleDelete(t: Tenant) {
  const confirmed = await show({
    title: 'Delete Tenant',
    description: `Are you sure you want to delete "${t.name}"? All associated data will be lost.`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteTenant(t.id)
      loadTenants()
    } catch {
      // handled
    }
  }
}

onMounted(loadTenants)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <div class="flex items-center gap-2">
          <h1 class="text-2xl font-bold">Tenants</h1>
          <Shield class="h-5 w-5 text-muted-foreground" />
        </div>
        <p class="text-muted-foreground">Manage multi-tenant organizations</p>
      </div>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Add Tenant
      </Button>
    </div>

    <Card>
      <DataTable
        :columns="columns"
        :data="tenants"
        :loading="loading"
        empty-text="No tenants found."
      >
        <template #name="{ row }: { row: Tenant }">
          <span class="font-medium">{{ row.name }}</span>
        </template>
        <template #slug="{ row }: { row: Tenant }">
          <code class="text-sm bg-muted px-1.5 py-0.5 rounded">{{ row.slug }}</code>
        </template>
        <template #domain="{ row }: { row: Tenant }">
          {{ row.domain || '-' }}
        </template>
        <template #status="{ row }: { row: Tenant }">
          <Badge :variant="row.active ? 'success' : 'destructive'">
            {{ row.active ? 'Active' : 'Inactive' }}
          </Badge>
        </template>
        <template #created_at="{ row }: { row: Tenant }">
          {{ new Date(row.created_at).toLocaleDateString() }}
        </template>
        <template #actions="{ row }: { row: Tenant }">
          <div class="flex gap-1">
            <button
              class="p-1.5 rounded hover:bg-muted"
              @click="openEdit(row)"
            >
              <Pencil class="h-4 w-4" />
            </button>
            <button
              class="p-1.5 rounded hover:bg-muted text-destructive"
              @click="handleDelete(row)"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </div>
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

    <!-- Modal -->
    <Modal :open="modalOpen" :title="isEdit ? 'Edit Tenant' : 'Add Tenant'" @close="modalOpen = false">
      <div class="space-y-4">
        <Input v-model="tenantName" label="Name" placeholder="Acme Corp" />
        <Input v-model="tenantSlug" label="Slug" placeholder="acme-corp" />
        <Input v-model="tenantDomain" label="Domain" placeholder="acme.com" />
        <div class="flex items-center gap-2">
          <input
            v-model="tenantActive"
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
