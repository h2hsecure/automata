<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import {
  Pencil,
  Trash2,
  Plus,
  Send,
  RefreshCw,
  Mail,
  ChevronDown,
} from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Modal from '@/components/Modal.vue'
import Input from '@/components/Input.vue'
import Textarea from '@/components/Textarea.vue'
import EmptyState from '@/components/EmptyState.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { useAuthStore } from '@/stores/auth'
import {
  listEmailTemplates,
  createEmailTemplate,
  updateEmailTemplate,
  deleteEmailTemplate,
  sendTemplateToContact,
  listInbox,
  markInboxRead,
  listForms,
  listContacts,
} from '@/api/client'
import type { EmailTemplate, InboxMessage, Form, Contact } from '@/types/api'

const authStore = useAuthStore()
const { confirmStore, show } = useConfirm()

const tab = ref<'templates' | 'inbox'>('templates')

// --- Templates tab ---
const templates = ref<EmailTemplate[]>([])
const forms = ref<Form[]>([])
const loading = ref(false)

const modalOpen = ref(false)
const isEdit = ref(false)
const editingId = ref('')
const tplKey = ref('')
const tplKind = ref<'standalone' | 'form'>('standalone')
const tplFormId = ref('')
const tplAudience = ref<'admin' | 'submitter'>('submitter')
const tplSubject = ref('')
const tplBody = ref('')
const tplError = ref('')

const placeholderHint = computed(() =>
  tplKind.value === 'form'
    ? '{{ .form.<field> }}  {{ .form_name }}  {{ .submit_date }}  (plus contact variables when a contact matches)'
    : '{{ .first_name }}  {{ .last_name }}  {{ .email }}  {{ .company }}  {{ .phone }}  {{ .custom.<field> }}'
)

async function loadTemplates() {
  loading.value = true
  try {
    const ctx = authStore.currentContextId
    templates.value = await listEmailTemplates(ctx)
    forms.value = await listForms(ctx)
  } finally {
    loading.value = false
  }
}

function formName(id: string | null): string {
  if (!id) return '-'
  return forms.value.find((f) => f.ID === id)?.Name || '-'
}

function openCreate() {
  isEdit.value = false
  editingId.value = ''
  tplKey.value = ''
  tplKind.value = 'standalone'
  tplFormId.value = ''
  tplAudience.value = 'submitter'
  tplSubject.value = ''
  tplBody.value = ''
  tplError.value = ''
  modalOpen.value = true
}

function openEdit(t: EmailTemplate) {
  isEdit.value = true
  editingId.value = t.id
  tplKey.value = t.key
  tplKind.value = t.kind === 'form' ? 'form' : 'standalone'
  tplFormId.value = t.form_id || ''
  tplAudience.value = t.audience === 'admin' ? 'admin' : 'submitter'
  tplSubject.value = t.subject
  tplBody.value = t.body_text
  tplError.value = ''
  modalOpen.value = true
}

async function handleSave() {
  if (!tplKey.value.trim() || !tplSubject.value.trim()) {
    tplError.value = 'Key and subject are required.'
    return
  }
  if (tplKind.value === 'form' && !tplFormId.value) {
    tplError.value = 'Select a form for form-integrated templates.'
    return
  }
  loading.value = true
  try {
    const payload: Record<string, unknown> = {
      key: tplKey.value.trim(),
      kind: tplKind.value,
      form_id: tplKind.value === 'form' ? tplFormId.value : null,
      subject: tplSubject.value,
      body_text: tplBody.value,
    }
    if (tplKind.value === 'form') {
      payload.audience = tplAudience.value
    }
    if (isEdit.value) {
      await updateEmailTemplate(editingId.value, payload)
    } else {
      await createEmailTemplate({ context_id: authStore.currentContextId, ...payload })
    }
    modalOpen.value = false
    loadTemplates()
  } catch (e: any) {
    tplError.value = e.response?.data?.error || 'Failed to save template.'
  } finally {
    loading.value = false
  }
}

async function handleDelete(t: EmailTemplate) {
  const confirmed = await show({
    title: 'Delete Template',
    description: `Are you sure you want to delete "${t.key}"? This action cannot be undone.`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteEmailTemplate(t.id)
      loadTemplates()
    } catch {
      // handled
    }
  }
}

// --- Standalone send ---
const sendOpen = ref(false)
const sendingTemplate = ref<EmailTemplate | null>(null)
const contactSearch = ref('')
const contacts = ref<Contact[]>([])
const contactsLoading = ref(false)
const selectedContactId = ref('')
const sending = ref(false)
const sendResult = ref('')

function openSend(t: EmailTemplate) {
  sendingTemplate.value = t
  contactSearch.value = ''
  selectedContactId.value = ''
  sendResult.value = ''
  sendOpen.value = true
  searchContacts()
}

async function searchContacts() {
  contactsLoading.value = true
  try {
    const res = await listContacts(1, 20, contactSearch.value, authStore.currentContextId)
    contacts.value = res.data
  } finally {
    contactsLoading.value = false
  }
}

async function confirmSend() {
  if (!selectedContactId.value || !sendingTemplate.value) return
  sending.value = true
  sendResult.value = ''
  try {
    const res = await sendTemplateToContact(
      authStore.currentContextId,
      sendingTemplate.value.id,
      selectedContactId.value
    )
    sendResult.value = `Queued for sending (job ${res.job_id.slice(0, 8)}).`
    selectedContactId.value = ''
  } catch (e: any) {
    sendResult.value = e.response?.data?.error || 'Failed to send.'
  } finally {
    sending.value = false
  }
}

// --- Inbox tab ---
const inboxMessages = ref<InboxMessage[]>([])
const inboxLoading = ref(false)
const expandedId = ref('')

async function loadInbox() {
  inboxLoading.value = true
  try {
    const res = await listInbox(authStore.currentContextId, 50)
    inboxMessages.value = res.messages
  } finally {
    inboxLoading.value = false
  }
}

async function toggleMessage(m: InboxMessage) {
  if (expandedId.value === m.id) {
    expandedId.value = ''
    return
  }
  expandedId.value = m.id
  if (!m.is_read) {
    expandedId.value = m.id
    try {
      await markInboxRead(authStore.currentContextId, m.id)
      m.is_read = true
    } catch {
      // handled
    }
  }
}

const columns = [
  { key: 'key', label: 'Key' },
  { key: 'kind', label: 'Kind' },
  { key: 'form', label: 'Form' },
  { key: 'subject', label: 'Subject' },
  { key: 'updated_at', label: 'Updated' },
  { key: 'actions', label: 'Actions', width: '130px' },
]

function loadActiveTab() {
  if (tab.value === 'templates') loadTemplates()
  else loadInbox()
}

watch(tab, loadActiveTab)

watch(
  () => authStore.currentContextId,
  () => loadActiveTab()
)

onMounted(loadActiveTab)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">Email</h1>
        <p class="text-muted-foreground">Email templates, sends, and inbox</p>
      </div>
      <div v-if="tab === 'templates'" class="flex gap-1 rounded-lg bg-muted p-1">
        <button
          v-for="t in [
            { id: 'templates', label: 'Templates' },
            { id: 'inbox', label: 'Inbox' },
          ]"
          :key="t.id"
          :class="[
            'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
            tab === t.id
              ? 'bg-card text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground',
          ]"
          @click="tab = t.id"
        >
          {{ t.label }}
        </button>
      </div>
    </div>

    <!-- Templates tab -->
    <template v-if="tab === 'templates'">
      <div class="flex justify-end">
        <Button @click="openCreate">
          <Plus class="mr-2 h-4 w-4" />
          New Template
        </Button>
      </div>

      <Card>
        <DataTable
          :columns="columns"
          :data="templates"
          :loading="loading"
          empty-text="No email templates found."
        >
          <template #key="{ row }: { row: EmailTemplate }">
            <code class="text-sm bg-muted px-1.5 py-0.5 rounded">{{ row.key }}</code>
          </template>
          <template #kind="{ row }: { row: EmailTemplate }">
            <Badge :variant="row.kind === 'form' ? 'warning' : 'info'" :text="row.kind" />
          </template>
          <template #form="{ row }: { row: EmailTemplate }">
            {{ formName(row.form_id) }}
          </template>
          <template #subject="{ row }: { row: EmailTemplate }">
            <span class="text-sm truncate block max-w-[240px]" :title="row.subject">
              {{ row.subject }}
            </span>
          </template>
          <template #updated_at="{ row }: { row: EmailTemplate }">
            {{ new Date(row.updated_at).toLocaleDateString() }}
          </template>
          <template #actions="{ row }: { row: EmailTemplate }">
            <div class="flex gap-1">
              <button
                v-if="row.kind !== 'form'"
                class="p-1.5 rounded hover:bg-muted"
                title="Send to contact"
                @click="openSend(row)"
              >
                <Send class="h-4 w-4" />
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
    </template>

    <!-- Inbox tab -->
    <template v-else>
      <div class="flex items-center justify-between">
        <p class="text-sm text-muted-foreground">
          Messages polled from the website's mailbox (refreshes every 60s server-side).
        </p>
        <Button variant="outline" size="sm" @click="loadInbox">
          <RefreshCw class="mr-2 h-4 w-4" />
          Refresh
        </Button>
      </div>

      <Card>
        <EmptyState
          v-if="!inboxLoading && inboxMessages.length === 0"
          title="Inbox is empty"
          description="New messages will appear here after the next poll."
        />
        <p
          v-else-if="inboxLoading"
          class="py-8 text-center text-sm text-muted-foreground"
        >
          Loading inbox...
        </p>
        <div v-else class="divide-y">
          <div v-for="m in inboxMessages" :key="m.id" class="py-3 first:pt-0 last:pb-0">
            <button
              class="flex w-full items-center gap-3 text-left"
              @click="toggleMessage(m)"
            >
              <ChevronDown
                :class="[
                  'h-4 w-4 flex-shrink-0 text-muted-foreground transition-transform',
                  { 'rotate-180': expandedId === m.id },
                ]"
              />
              <div class="flex-1 min-w-0">
                <div class="flex items-center gap-2">
                  <Badge v-if="!m.is_read" variant="info" text="New" />
                  <span
                    :class="[
                      'truncate',
                      { 'font-semibold': !m.is_read, 'font-medium': m.is_read },
                    ]"
                  >
                    {{ m.from }}
                  </span>
                </div>
                <p class="truncate text-sm text-muted-foreground">{{ m.subject || '(no subject)' }}</p>
              </div>
              <span class="flex-shrink-0 text-xs text-muted-foreground">
                {{ new Date(m.message_date).toLocaleString() }}
              </span>
            </button>
            <div
              v-if="expandedId === m.id"
              class="ml-7 mt-2 max-h-96 overflow-y-auto whitespace-pre-wrap rounded-md bg-muted p-3 text-sm"
            >
              {{ m.body_text || '(no text body)' }}
            </div>
          </div>
        </div>
      </Card>
    </template>

    <!-- Create/Edit Modal -->
    <Modal :open="modalOpen" :title="isEdit ? 'Edit Template' : 'New Template'" @close="modalOpen = false">
      <div class="space-y-4">
        <Input v-model="tplKey" label="Key" placeholder="welcome" :disabled="isEdit" />
        <div>
          <label class="text-sm font-medium">Kind</label>
          <select
            v-model="tplKind"
            class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
          >
            <option value="standalone">Standalone (sent to a contact)</option>
            <option value="form">Form-integrated (sent with form submission)</option>
          </select>
        </div>
        <div v-if="tplKind === 'form'">
          <label class="text-sm font-medium">Form</label>
          <select
            v-model="tplFormId"
            class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
          >
            <option value="" disabled>Select a form</option>
            <option v-for="f in forms" :key="f.ID" :value="f.ID">{{ f.Name }}</option>
          </select>
        </div>
        <div v-if="tplKind === 'form'">
          <label class="text-sm font-medium">Audience</label>
          <select
            v-model="tplAudience"
            class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
          >
            <option value="submitter">Submitter (the person who submitted the form)</option>
            <option value="admin">Admin (sent to the form's notification recipients)</option>
          </select>
        </div>
        <Input v-model="tplSubject" label="Subject" placeholder="Welcome, {{ .first_name }}!" />
        <div>
          <Textarea v-model="tplBody" label="Body" placeholder="Hi {{ .first_name }}, ..." :rows="6" />
          <p class="mt-1 text-xs text-muted-foreground">
            Placeholders:
            <code class="bg-muted px-1 py-0.5 rounded">{{ placeholderHint }}</code>
          </p>
        </div>
        <p v-if="tplError" class="text-sm text-destructive">{{ tplError }}</p>
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

    <!-- Send Modal -->
    <Modal :open="sendOpen" title="Send to Contact" @close="sendOpen = false">
      <div class="space-y-4">
        <p v-if="sendingTemplate" class="text-sm text-muted-foreground">
          Template
          <code class="bg-muted px-1 py-0.5 rounded">{{ sendingTemplate.key }}</code>
          — "{{ sendingTemplate.subject }}"
        </p>
        <Input
          v-model="contactSearch"
          label="Search contacts"
          placeholder="Name or email"
          @input="searchContacts"
        />
        <div class="max-h-60 space-y-1 overflow-y-auto">
          <p
            v-if="contactsLoading"
            class="py-2 text-center text-sm text-muted-foreground"
          >
            Searching...
          </p>
          <p
            v-else-if="contacts.length === 0"
            class="py-2 text-center text-sm text-muted-foreground"
          >
            No contacts found.
          </p>
          <label
            v-for="c in contacts"
            :key="c.ID"
            :class="[
              'flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm',
              selectedContactId === c.ID ? 'border-primary bg-primary/5' : 'hover:bg-muted',
            ]"
          >
            <input
              type="radio"
              :value="c.ID"
              v-model="selectedContactId"
              class="rounded"
            />
            <span class="flex-1 truncate font-medium">{{ c.FirstName }} {{ c.LastName }}</span>
            <span class="truncate text-muted-foreground">{{ c.Email || '(no email)' }}</span>
          </label>
        </div>
        <p v-if="sendResult" class="text-sm text-muted-foreground">{{ sendResult }}</p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <Button variant="outline" @click="sendOpen = false">Close</Button>
          <Button
            :disabled="!selectedContactId"
            :loading="sending"
            @click="confirmSend"
          >
            <Mail class="mr-2 h-4 w-4" />
            Send
          </Button>
        </div>
      </template>
    </Modal>

    <ConfirmDialog />
  </div>
</template>