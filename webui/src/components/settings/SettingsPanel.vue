<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Pencil, Trash2, Plus } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import DataTable from '@/components/DataTable.vue'
import Modal from '@/components/Modal.vue'
import Input from '@/components/Input.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { listSettings, upsertSetting, deleteSetting } from '@/api/client'

interface SettingRow {
  key: string
  value: string
}

const { show } = useConfirm()
const settings = ref<SettingRow[]>([])
const loading = ref(false)
const saving = ref(false)
const modalOpen = ref(false)
const isEdit = ref(false)
const editingKey = ref('')
const key = ref('')
const value = ref('')
const formError = ref('')

const columns = [
  { key: 'key', label: 'Key' },
  { key: 'value', label: 'Value' },
  { key: 'actions', label: 'Actions', width: '100px' },
]

async function loadSettings() {
  loading.value = true
  try {
    const data = await listSettings()
    settings.value = Object.entries(data).map(([k, v]) => ({ key: k, value: v }))
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  editingKey.value = ''
  key.value = ''
  value.value = ''
  formError.value = ''
  modalOpen.value = true
}

function openEdit(row: SettingRow) {
  isEdit.value = true
  editingKey.value = row.key
  key.value = row.key
  value.value = row.value
  formError.value = ''
  modalOpen.value = true
}

async function handleSave() {
  if (!key.value.trim()) {
    formError.value = 'Key is required.'
    return
  }
  saving.value = true
  try {
    await upsertSetting(key.value.trim(), value.value)
    modalOpen.value = false
    await loadSettings()
  } catch (e: any) {
    formError.value = e.response?.data?.error || 'Failed to save setting.'
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: SettingRow) {
  const confirmed = await show({
    title: 'Delete Setting',
    description: `Are you sure you want to delete "${row.key}"?`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteSetting(row.key)
      await loadSettings()
    } catch {
      // handled
    }
  }
}

onMounted(loadSettings)
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <p class="text-sm text-muted-foreground">
        Global settings applied across all websites
      </p>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Add Setting
      </Button>
    </div>

    <DataTable
      :columns="columns"
      :data="settings"
      :loading="loading"
      empty-text="No settings found."
    >
      <template #key="{ row }: { row: SettingRow }">
        <code class="text-sm bg-muted px-1.5 py-0.5 rounded">{{ row.key }}</code>
      </template>
      <template #value="{ row }: { row: SettingRow }">
        {{ row.value || '-' }}
      </template>
      <template #actions="{ row }: { row: SettingRow }">
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

    <!-- Modal -->
    <Modal :open="modalOpen" :title="isEdit ? 'Edit Setting' : 'Add Setting'" @close="modalOpen = false">
      <div class="space-y-4">
        <Input v-model="key" label="Key" placeholder="e.g. default_redirect" :disabled="isEdit" />
        <Input v-model="value" label="Value" placeholder="Setting value" />
        <p v-if="formError" class="text-sm text-destructive">{{ formError }}</p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <Button variant="outline" @click="modalOpen = false">Cancel</Button>
          <Button :loading="saving" @click="handleSave">
            {{ isEdit ? 'Save' : 'Create' }}
          </Button>
        </div>
      </template>
    </Modal>

    <ConfirmDialog />
  </div>
</template>