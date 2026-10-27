<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Trash2, Upload } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Input from '@/components/Input.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { useAuthStore } from '@/stores/auth'
import { listStaticFiles, uploadStaticFile, deleteStaticFile } from '@/api/client'
import type { StaticFile } from '@/types/api'

const authStore = useAuthStore()
const { confirmStore, show, confirm } = useConfirm()
const files = ref<StaticFile[]>([])
const loading = ref(false)
const uploadFile = ref<File | null>(null)
const uploadName = ref('')
const uploading = ref(false)
const uploadError = ref('')

const columns = [
  { key: 'name', label: 'Name' },
  { key: 'mime', label: 'Type' },
  { key: 'size', label: 'Size' },
  { key: 'url', label: 'Public URL' },
  { key: 'created_at', label: 'Created' },
  { key: 'actions', label: 'Actions', width: '80px' },
]

async function loadFiles() {
  loading.value = true
  try {
    files.value = await listStaticFiles()
  } finally {
    loading.value = false
  }
}

watch(
  () => authStore.currentContextId,
  () => loadFiles()
)

function publicURL(f: StaticFile) {
  return `/static/${authStore.currentContextId}/${f.Name}`
}

function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function onFileChosen(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  uploadFile.value = file
  if (file && !uploadName.value) uploadName.value = file.name
}

async function handleUpload() {
  if (!uploadFile.value) {
    uploadError.value = 'Choose a file first.'
    return
  }
  if (!uploadName.value.trim()) {
    uploadError.value = 'A URL name is required.'
    return
  }
  uploading.value = true
  uploadError.value = ''
  try {
    await uploadStaticFile(uploadFile.value, uploadName.value.trim())
    uploadFile.value = null
    uploadName.value = ''
    loadFiles()
  } catch (e: any) {
    uploadError.value = e.response?.data?.error || 'Failed to upload file.'
  } finally {
    uploading.value = false
  }
}

async function handleDelete(f: StaticFile) {
  const confirmed = await show({
    title: 'Delete File',
    description: `Are you sure you want to delete "${f.Name}"?`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteStaticFile(f.Name)
      loadFiles()
    } catch {
      // handled
    }
  }
}

onMounted(loadFiles)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">Static Files</h1>
        <p class="text-muted-foreground">Files linked to this website, served publicly</p>
      </div>
    </div>

    <Card>
      <div class="mb-4 flex flex-wrap items-end gap-3">
        <div class="space-y-1">
          <label class="text-sm font-medium leading-none">File</label>
          <input
            type="file"
            class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm file:border-0 file:bg-transparent file:text-sm file:font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
            @change="onFileChosen"
          />
        </div>
        <Input
          v-model="uploadName"
          label="URL name"
          placeholder="logo.png"
          class="w-56"
        />
        <Button :loading="uploading" @click="handleUpload">
          <Upload class="mr-2 h-4 w-4" />
          Upload
        </Button>
        <p v-if="uploadError" class="text-sm text-destructive">{{ uploadError }}</p>
      </div>

      <DataTable
        :columns="columns"
        :data="files"
        :loading="loading"
        empty-text="No static files uploaded."
      >
        <template #name="{ row }: { row: StaticFile }">
          <span class="font-medium">{{ row.Name }}</span>
        </template>
        <template #mime="{ row }: { row: StaticFile }">
          <Badge variant="info">{{ row.Mime || '-' }}</Badge>
        </template>
        <template #size="{ row }: { row: StaticFile }">
          {{ formatSize(row.Size) }}
        </template>
        <template #url="{ row }: { row: StaticFile }">
          <code class="text-xs bg-muted px-1.5 py-0.5 rounded">{{ publicURL(row) }}</code>
        </template>
        <template #created_at="{ row }: { row: StaticFile }">
          {{ new Date(row.CreatedAt).toLocaleDateString() }}
        </template>
        <template #actions="{ row }: { row: StaticFile }">
          <button
            class="p-1.5 rounded hover:bg-muted text-destructive"
            title="Delete"
            @click="handleDelete(row)"
          >
            <Trash2 class="h-4 w-4" />
          </button>
        </template>
      </DataTable>
    </Card>

    <ConfirmDialog />
  </div>
</template>