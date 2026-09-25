<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Pencil, Trash2, Plus, Key, Copy } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Modal from '@/components/Modal.vue'
import Input from '@/components/Input.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { listApiKeys, createApiKey, revokeApiKey } from '@/api/client'
import type { ApiKey } from '@/types/api'

const { confirmStore, show, confirm } = useConfirm()
const keys = ref<ApiKey[]>([])
const loading = ref(false)
const modalOpen = ref(false)
const keyName = ref('')
const keyExpiry = ref('')
const newKey = ref('')
const formError = ref('')
const justCreated = ref(false)

const columns = [
  { key: 'name', label: 'Name' },
  { key: 'key', label: 'Key' },
  { key: 'status', label: 'Status' },
  { key: 'expires_at', label: 'Expires' },
  { key: 'created_at', label: 'Created' },
  { key: 'actions', label: 'Actions', width: '80px' },
]

async function loadKeys() {
  loading.value = true
  try {
    keys.value = await listApiKeys()
  } finally {
    loading.value = false
  }
}

function openCreate() {
  keyName.value = ''
  keyExpiry.value = ''
  newKey.value = ''
  formError.value = ''
  justCreated.value = false
  modalOpen.value = true
}

async function handleSave() {
  if (!keyName.value.trim()) {
    formError.value = 'Key name is required.'
    return
  }
  loading.value = true
  try {
    const data: { name: string; expires_at?: string } = { name: keyName.value }
    if (keyExpiry.value) data.expires_at = keyExpiry.value
    const result = await createApiKey(data)
    newKey.value = result.key || ''
    justCreated.value = true
    loadKeys()
  } catch (e: any) {
    formError.value = e.response?.data?.error || 'Failed to create API key.'
  } finally {
    loading.value = false
  }
}

function copyKey() {
  if (newKey.value) {
    navigator.clipboard.writeText(newKey.value)
    newKey.value = ''
    justCreated.value = false
    modalOpen.value = false
  }
}

function dismissKey() {
  newKey.value = ''
  justCreated.value = false
  modalOpen.value = false
}

async function handleRevoke(key: ApiKey) {
  const confirmed = await show({
    title: 'Revoke API Key',
    description: `Are you sure you want to revoke "${key.name}"?`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await revokeApiKey(key.id)
      loadKeys()
    } catch {
      // handled
    }
  }
}

function maskKey(key: string): string {
  if (key.length <= 8) return key
  return key.substring(0, 8) + '...'
}

onMounted(loadKeys)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">API Keys</h1>
        <p class="text-muted-foreground">Manage API access keys</p>
      </div>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Create Key
      </Button>
    </div>

    <Card>
      <DataTable
        :columns="columns"
        :data="keys"
        :loading="loading"
        empty-text="No API keys found."
      >
        <template #name="{ row }: { row: ApiKey }">
          <div class="flex items-center gap-2">
            <Key class="h-4 w-4 text-muted-foreground" />
            <span class="font-medium">{{ row.name }}</span>
          </div>
        </template>
        <template #key="{ row }: { row: ApiKey }">
          <code class="text-sm bg-muted px-1.5 py-0.5 rounded">{{ row.key_prefix || '••••••' }}</code>
        </template>
        <template #status="{ row }: { row: ApiKey }">
          <Badge :variant="row.active ? 'success' : 'destructive'">
            {{ row.active ? 'Active' : 'Revoked' }}
          </Badge>
        </template>
        <template #expires_at="{ row }: { row: ApiKey }">
          {{ row.expires_at ? new Date(row.expires_at).toLocaleDateString() : 'Never' }}
        </template>
        <template #created_at="{ row }: { row: ApiKey }">
          {{ new Date(row.created_at).toLocaleDateString() }}
        </template>
        <template #actions="{ row }: { row: ApiKey }">
          <button
            v-if="row.active"
            class="p-1.5 rounded hover:bg-muted text-destructive"
            @click="handleRevoke(row)"
          >
            <Trash2 class="h-4 w-4" />
          </button>
        </template>
      </DataTable>
    </Card>

    <!-- Create Key Modal -->
    <Modal :open="modalOpen" :title="justCreated ? 'API Key Created' : 'Create API Key'" @close="justCreated ? dismissKey() : (modalOpen = false)">
      <div v-if="justCreated && newKey" class="space-y-4">
        <div class="rounded-md bg-muted p-4">
          <p class="text-sm font-medium mb-2">Save this key — it won't be shown again:</p>
          <code class="break-all text-sm select-all">{{ newKey }}</code>
        </div>
        <div class="flex gap-2">
          <Button @click="copyKey">
            <Copy class="mr-2 h-4 w-4" />
            Copy Key
          </Button>
        </div>
      </div>
      <div v-else class="space-y-4">
        <Input v-model="keyName" label="Key Name" placeholder="Production API Key" />
        <Input v-model="keyExpiry" type="date" label="Expiry Date (optional)" />
        <p v-if="formError" class="text-sm text-destructive">{{ formError }}</p>
      </div>
      <template #footer>
        <div v-if="justCreated" class="flex justify-end gap-2">
          <Button @click="dismissKey">Done</Button>
        </div>
        <div v-else class="flex justify-end gap-2">
          <Button variant="outline" @click="modalOpen = false">Cancel</Button>
          <Button :loading="loading" @click="handleSave">Create</Button>
        </div>
      </template>
    </Modal>

    <ConfirmDialog />
  </div>
</template>
