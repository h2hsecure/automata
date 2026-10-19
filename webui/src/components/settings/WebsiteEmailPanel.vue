<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Button from '@/components/Button.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Input from '@/components/Input.vue'
import Modal from '@/components/Modal.vue'
import { useConfirm } from '@/composables/useConfirm'
import {
  listContextSettings,
  upsertContextSetting,
  deleteContextSetting,
  listEmailQueue,
  retryEmailJob,
  cancelEmailJob,
  sendTestEmail,
  type EmailQueueJob,
} from '@/api/client'
import { settingGroups } from './settingsCatalog'
import type { SettingField } from './settingsCatalog'
import type { Context } from '@/types/api'

const props = defineProps<{
  open: boolean
  context: Context | null
}>()

const emit = defineEmits<{ close: [] }>()

const { show } = useConfirm()

const smtpGroup = settingGroups.find((g) => g.id === 'smtp')!

// SMTP settings
const values = ref<Record<string, string>>({})
const original = ref<Record<string, string>>({})
const loading = ref(false)
const saving = ref(false)
const saveError = ref('')
const saveSuccess = ref(false)

const dirtyKeys = computed(() =>
  Object.keys(values.value).filter((k) => values.value[k] !== original.value[k]),
)

const isDirty = computed(() => dirtyKeys.value.length > 0)

function fieldChecked(field: SettingField): boolean {
  return values.value[field.key] !== 'false'
}

function setFieldChecked(field: SettingField, checked: boolean) {
  values.value[field.key] = checked ? 'true' : 'false'
}

async function loadSettings() {
  if (!props.context) return
  loading.value = true
  try {
    const data = await listContextSettings(props.context.ID)
    const next: Record<string, string> = {}
    for (const field of smtpGroup.fields) {
      next[field.key] = data[field.key] ?? ''
    }
    values.value = next
    original.value = { ...next }
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!props.context) return
  saving.value = true
  saveError.value = ''
  saveSuccess.value = false
  try {
    for (const key of dirtyKeys.value) {
      const next = values.value[key]
      if (next === '') {
        await deleteContextSetting(props.context.ID, key)
      } else {
        await upsertContextSetting(props.context.ID, key, next)
      }
    }
    original.value = { ...values.value }
    saveSuccess.value = true
  } catch (e: any) {
    saveError.value = e.response?.data?.error || 'Failed to save settings.'
  } finally {
    saving.value = false
  }
}

// Test send
const testEmail = ref('')
const testSending = ref(false)
const testResult = ref('')
const testError = ref('')

async function handleTestSend() {
  if (!props.context) return
  const to = testEmail.value.trim()
  if (!to) {
    testError.value = 'Enter an email address.'
    testResult.value = ''
    return
  }
  testSending.value = true
  testError.value = ''
  testResult.value = ''
  try {
    await sendTestEmail(props.context.ID, to)
    testResult.value = `Test email queued for ${to}.`
    await loadQueue()
  } catch (e: any) {
    testError.value = e.response?.data?.error || 'Failed to send test email.'
  } finally {
    testSending.value = false
  }
}

// Queue
type QueueStatus = 'pending' | 'sent' | 'failed'
const activeStatus = ref<QueueStatus>('pending')
const queue = ref<Record<QueueStatus, EmailQueueJob[]>>({ pending: [], sent: [], failed: [] })
const queueLoading = ref(false)
const busyId = ref('')

async function loadQueue() {
  if (!props.context) return
  queueLoading.value = true
  try {
    const [pending, sent, failed] = await Promise.all([
      listEmailQueue(props.context.ID, 'pending', 500),
      listEmailQueue(props.context.ID, 'sent', 500),
      listEmailQueue(props.context.ID, 'failed', 500),
    ])
    queue.value = { pending, sent, failed }
  } finally {
    queueLoading.value = false
  }
}

const activeJobs = computed(() => queue.value[activeStatus.value])

const counts = computed(() => ({
  pending: queue.value.pending.length,
  sent: queue.value.sent.length,
  failed: queue.value.failed.length,
}))

const queueColumns = [
  { key: 'to', label: 'To' },
  { key: 'subject', label: 'Subject' },
  { key: 'attempts', label: 'Attempts' },
  { key: 'created_at', label: 'Created' },
  { key: 'last_error', label: 'Last error' },
  { key: 'actions', label: 'Actions', width: '120px' },
]

async function handleRetry(job: EmailQueueJob) {
  if (!props.context) return
  busyId.value = job.id
  try {
    await retryEmailJob(props.context.ID, job.id)
    await loadQueue()
  } finally {
    busyId.value = ''
  }
}

async function handleCancel(job: EmailQueueJob) {
  if (!props.context) return
  const confirmed = await show({
    title: 'Cancel Email',
    description: `Cancel the pending email "${job.subject}" to ${job.to.join(', ')}?`,
    variant: 'destructive',
  })
  if (!confirmed) return
  busyId.value = job.id
  try {
    await cancelEmailJob(props.context.ID, job.id)
    await loadQueue()
  } finally {
    busyId.value = ''
  }
}

watch(
  [() => props.open, () => props.context?.ID],
  ([open]) => {
    if (open) {
      saveSuccess.value = false
      saveError.value = ''
      testResult.value = ''
      testError.value = ''
      loadSettings()
      loadQueue()
    }
  },
)
</script>

<template>
  <Modal :open="open" :title="context ? `Email — ${context.Name}` : 'Email'" @close="emit('close')">
    <div class="space-y-6">
      <!-- SMTP settings -->
      <section>
        <div class="flex items-center justify-between">
          <div>
            <h4 class="text-sm font-semibold">{{ smtpGroup.title }}</h4>
            <p class="text-xs text-muted-foreground">
              {{ smtpGroup.description }} Empty fields inherit from global settings.
            </p>
          </div>
          <Button size="sm" :disabled="!isDirty || saving" :loading="saving" @click="handleSave">
            Save Changes
          </Button>
        </div>
        <p v-if="saveError" class="mt-2 text-sm text-destructive">{{ saveError }}</p>
        <p v-if="saveSuccess" class="mt-2 text-sm text-green-600">Settings saved.</p>
        <div v-if="loading" class="py-4 text-center text-sm text-muted-foreground">
          Loading settings…
        </div>
        <div v-else class="mt-3 grid gap-4 sm:grid-cols-2">
          <template v-for="field in smtpGroup.fields" :key="field.key">
            <div v-if="field.type === 'boolean'" class="flex items-center gap-2">
              <input
                :id="`ctx-${field.key}`"
                type="checkbox"
                class="h-4 w-4 rounded border"
                :checked="fieldChecked(field)"
                @change="setFieldChecked(field, ($event.target as HTMLInputElement).checked)"
              />
              <label :for="`ctx-${field.key}`" class="text-sm">
                {{ field.label }}
                <span v-if="field.description" class="text-muted-foreground">
                  — {{ field.description }}
                </span>
              </label>
            </div>
            <Input
              v-else
              v-model="values[field.key]"
              :label="field.label"
              :type="field.type"
              placeholder="Inherit from global"
            />
          </template>
        </div>
      </section>

      <!-- Test send -->
      <section>
        <h4 class="text-sm font-semibold">Test send</h4>
        <p class="text-xs text-muted-foreground">
          Queue a test email using this website's SMTP settings.
        </p>
        <div class="mt-3 flex gap-2">
          <Input v-model="testEmail" placeholder="you@example.com" class="flex-1" />
          <Button :loading="testSending" @click="handleTestSend">Send test email</Button>
        </div>
        <p v-if="testError" class="mt-2 text-sm text-destructive">{{ testError }}</p>
        <p v-if="testResult" class="mt-2 text-sm text-green-600">{{ testResult }}</p>
      </section>

      <!-- Queue -->
      <section>
        <div class="flex items-center justify-between">
          <h4 class="text-sm font-semibold">Email queue</h4>
          <div class="flex items-center gap-2">
            <Badge :text="`Pending ${counts.pending}`" variant="warning" />
            <Badge :text="`Sent ${counts.sent}`" variant="success" />
            <Badge :text="`Failed ${counts.failed}`" variant="destructive" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-2">
          <select
            v-model="activeStatus"
            class="h-8 rounded-md border border-input bg-transparent px-2 text-sm"
          >
            <option value="pending">Pending</option>
            <option value="sent">Sent</option>
            <option value="failed">Failed</option>
          </select>
          <Button size="sm" variant="outline" @click="loadQueue">Refresh</Button>
        </div>
        <div class="mt-3">
          <DataTable
            :columns="queueColumns"
            :data="activeJobs"
            :loading="queueLoading"
            empty-text="No emails in this queue."
          >
            <template #to="{ row }: { row: EmailQueueJob }">
              {{ row.to.join(', ') }}
            </template>
            <template #attempts="{ row }: { row: EmailQueueJob }">
              {{ row.attempts }}{{ row.status === 'pending' ? ` / ${row.max_retries}` : '' }}
            </template>
            <template #created_at="{ row }: { row: EmailQueueJob }">
              {{ new Date(row.created_at).toLocaleString() }}
            </template>
            <template #last_error="{ row }: { row: EmailQueueJob }">
              <span class="text-xs text-destructive">{{ row.last_error || '-' }}</span>
            </template>
            <template #actions="{ row }: { row: EmailQueueJob }">
              <div class="flex gap-1">
                <Button
                  v-if="row.status === 'failed'"
                  size="sm"
                  variant="outline"
                  :loading="busyId === row.id"
                  @click="handleRetry(row)"
                >
                  Retry
                </Button>
                <Button
                  v-if="row.status === 'pending'"
                  size="sm"
                  variant="outline"
                  :loading="busyId === row.id"
                  @click="handleCancel(row)"
                >
                  Cancel
                </Button>
              </div>
            </template>
          </DataTable>
        </div>
      </section>
    </div>
    <template #footer>
      <div class="flex justify-end">
        <Button variant="outline" @click="emit('close')">Close</Button>
      </div>
    </template>
  </Modal>
</template>