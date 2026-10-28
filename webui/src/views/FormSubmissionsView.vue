<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Paperclip, Trash2 } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { usePagination } from '@/composables/usePagination'
import { getForm, listSubmissions, deleteSubmission, downloadSubmissionFile } from '@/api/client'
import type { Form, Submission } from '@/types/api'

const route = useRoute()
const router = useRouter()
const { confirmStore, show } = useConfirm()

const formId = computed(() => String(route.params.id || ''))
const form = ref<Form | null>(null)
const rows = ref<Array<Record<string, any>>>([])
const loading = ref(true)

const { page, limit, total, goToPage, setTotal } = usePagination(1, 20)

const columns = computed(() => [
  ...(form.value?.Fields ?? []).map((f) => ({ key: f.slug, label: f.label })),
  { key: 'files', label: 'Files' },
  { key: 'created', label: 'Created' },
])

// Flatten Data into slug-keyed cells so the default DataTable rendering works.
function flattenSubmission(sub: Submission): Record<string, any> {
  const row: Record<string, any> = {
    ID: sub.ID,
    Files: sub.Files ?? [],
    created: sub.CreatedAt,
  }
  for (const f of form.value?.Fields ?? []) {
    row[f.slug] = sub.Data?.[f.slug] ?? ''
  }
  return row
}

async function loadSubmissions() {
  loading.value = true
  try {
    const res = await listSubmissions(formId.value, page.value, limit.value)
    rows.value = res.data.map(flattenSubmission)
    setTotal(res.total)
  } finally {
    loading.value = false
  }
}

watch(page, loadSubmissions)

async function loadForm() {
  form.value = await getForm(formId.value)
}

// The file route is Bearer-authenticated, so downloads go through the axios
// client and trigger a browser save via an object URL.
async function downloadFile(row: Record<string, any>, index: number) {
  try {
    const blob = await downloadSubmissionFile(formId.value, row.ID, index)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = row.Files[index]?.name || 'download'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  } catch {
    // handled
  }
}

async function handleDelete(row: Record<string, any>) {
  const confirmed = await show({
    title: 'Delete Submission',
    description: 'Are you sure you want to delete this submission? This action cannot be undone.',
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteSubmission(formId.value, row.ID)
      loadSubmissions()
    } catch {
      // handled
    }
  }
}

function goBack() {
  router.push('/forms')
}

onMounted(async () => {
  await loadForm()
  await loadSubmissions()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <Button variant="outline" size="sm" class="mb-2" @click="goBack">
          <ArrowLeft class="mr-1 h-3 w-3" />
          Back
        </Button>
        <h1 class="text-2xl font-bold">
          Submissions{{ form ? ` — ${form.Name}` : '' }}
        </h1>
        <p class="text-muted-foreground">Submissions received via this form</p>
      </div>
    </div>

    <Card>
      <DataTable
        :columns="columns"
        :data="rows"
        :loading="loading"
        empty-text="No submissions yet."
      >
        <template #files="{ row }: { row: any }">
          <div v-if="row.Files?.length" class="flex flex-wrap gap-1">
            <button
              v-for="(file, i) in row.Files"
              :key="i"
              class="flex items-center gap-1 rounded border px-2 py-1 text-xs hover:bg-muted"
              @click="downloadFile(row, i)"
            >
              <Paperclip class="h-3 w-3" />
              {{ file.name }}
            </button>
          </div>
          <span v-else class="text-muted-foreground">-</span>
        </template>
        <template #created="{ row }: { row: any }">
          <div class="flex items-center gap-2">
            <span>{{ new Date(row.created).toLocaleDateString() }}</span>
            <button
              class="p-1 rounded hover:bg-muted text-destructive"
              title="Delete submission"
              @click="handleDelete(row)"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </div>
        </template>
      </DataTable>
      <div v-if="total > 0" class="flex items-center justify-between">
        <span class="text-sm text-muted-foreground">
          Showing {{ (page - 1) * limit + 1 }}-{{ Math.min(page * limit, total) }} of {{ total }}
        </span>
        <div class="flex gap-2">
          <Button
            variant="outline"
            size="sm"
            :disabled="page <= 1"
            @click="goToPage(page - 1)"
          >
            Prev
          </Button>
          <Button
            variant="outline"
            size="sm"
            :disabled="page >= Math.ceil(total / limit)"
            @click="goToPage(page + 1)"
          >
            Next
          </Button>
        </div>
      </div>
    </Card>

    <ConfirmDialog />
  </div>
</template>
