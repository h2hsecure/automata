<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Pencil, Trash2, Plus, ExternalLink, Code2, Copy, Check, Eye } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Modal from '@/components/Modal.vue'
import Input from '@/components/Input.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { useAuthStore } from '@/stores/auth'
import { listForms, deleteForm, createForm, updateForm } from '@/api/client'
import type { Form, FormInput } from '@/types/api'

const router = useRouter()
const authStore = useAuthStore()
const { confirmStore, show, confirm } = useConfirm()
const forms = ref<Form[]>([])
const loading = ref(false)
const modalOpen = ref(false)
const isEdit = ref(false)
const editingId = ref('')
const formName = ref('')
const formSlug = ref('')
const formFields = ref<Array<{ name: string; type: string; required: boolean; label: string; accept: string }>>([])
const formError = ref('')

// Submission notification settings (stored under Form.Settings.notifications).
const notifAdminEnabled = ref(true)
const notifAdminTo = ref('')
const notifSubmitterEnabled = ref(true)
// Other Settings keys, preserved across save.
const editingSettings = ref<Record<string, any>>({})

const embedOpen = ref(false)
const embedTag = ref('')
const embedSlug = ref('')
const copied = ref(false)

const columns = [
  { key: 'name', label: 'Name' },
  { key: 'slug', label: 'Slug' },
  { key: 'fields_count', label: 'Fields' },
  { key: 'created_at', label: 'Created' },
  { key: 'actions', label: 'Actions', width: '140px' },
]

async function loadForms() {
  loading.value = true
  try {
    forms.value = await listForms(authStore.currentContextId)
  } finally {
    loading.value = false
  }
}

watch(
  () => authStore.currentContextId,
  () => loadForms()
)

function openCreate() {
  isEdit.value = false
  editingId.value = ''
  formName.value = ''
  formSlug.value = ''
  formFields.value = [
    { name: 'email', type: 'email', required: true, label: 'Email', accept: '' },
    { name: 'name', type: 'text', required: true, label: 'Full Name', accept: '' },
  ]
  notifAdminEnabled.value = true
  notifAdminTo.value = ''
  notifSubmitterEnabled.value = true
  editingSettings.value = {}
  formError.value = ''
  modalOpen.value = true
}

function openEdit(f: Form) {
  isEdit.value = true
  editingId.value = f.ID
  formName.value = f.Name
  formSlug.value = f.Slug
  // Map backend FormField (slug) to the local field model (name).
  formFields.value = (f.Fields ?? []).map(fl => ({
    name: fl.slug,
    type: fl.type,
    required: fl.required,
    label: fl.label,
    accept: Array.isArray(fl.validation?.accept) ? (fl.validation.accept as string[]).join(', ') : '',
  }))
  // Notification config lives in Settings.notifications; both audiences are
  // enabled by default, and admin emails only send when `to` is non-empty.
  const n = (f.Settings?.notifications ?? {}) as any
  notifAdminEnabled.value = n.admin?.enabled !== false
  notifAdminTo.value = Array.isArray(n.admin?.to) ? n.admin.to.join(', ') : ''
  notifSubmitterEnabled.value = n.submitter?.enabled !== false
  editingSettings.value = { ...(f.Settings ?? {}) }
  formError.value = ''
  modalOpen.value = true
}

function parseEmailList(raw: string): string[] {
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

function openSubmissions(f: Form) {
  router.push({ name: 'FormSubmissions', params: { id: f.ID } })
}

function addField() {
  formFields.value.push({ name: '', type: 'text', required: false, label: 'New Field', accept: '' })
}

function removeField(index: number) {
  formFields.value.splice(index, 1)
}

async function handleSave() {
  if (!formName.value.trim()) {
    formError.value = 'Form name is required.'
    return
  }
  if (formFields.value.length === 0) {
    formError.value = 'Add at least one field.'
    return
  }
  loading.value = true
  try {
    const data: FormInput = {
      name: formName.value,
      slug: formSlug.value || formName.value.toLowerCase().replace(/\s+/g, '-'),
      // Map the local field model (name) to the backend FormField (slug).
      fields: formFields.value.map((f, i) => ({
        slug: f.name,
        type: f.type,
        label: f.label,
        required: f.required,
        validation:
          f.type === 'file'
            ? { accept: f.accept.split(',').map((s) => s.trim()).filter(Boolean) }
            : null,
        order: i,
      })),
      // Keep unrelated Settings keys and replace the notifications block.
      settings: {
        ...editingSettings.value,
        notifications: {
          admin: {
            enabled: notifAdminEnabled.value,
            to: parseEmailList(notifAdminTo.value),
          },
          submitter: { enabled: notifSubmitterEnabled.value },
        },
      },
    }
    if (isEdit.value) {
      await updateForm(editingId.value, data)
    } else {
      await createForm(data)
    }
    modalOpen.value = false
    loadForms()
  } catch (e: any) {
    formError.value = e.response?.data?.error || 'Failed to save form.'
  } finally {
    loading.value = false
  }
}

function openEmbed(f: Form) {
  embedSlug.value = f.Slug
  // Note: the closing tag is escaped below because a literal closing
  // script tag inside this block would terminate it.
  embedTag.value = `<script src="/form/snippet/${authStore.currentContextId}/${f.Slug}"><\/script>`
  copied.value = false
  embedOpen.value = true
}

async function copyEmbed() {
  const text = embedTag.value
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      // Non-secure context (plain HTTP): fall back to execCommand.
      const ta = document.createElement('textarea')
      ta.value = text
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } catch {
    // Ignore — the user can select the text manually.
  }
}

async function handleDelete(f: Form) {
  const confirmed = await show({
    title: 'Delete Form',
    description: `Are you sure you want to delete "${f.Name}"? This action cannot be undone.`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteForm(f.ID)
      loadForms()
    } catch {
      // handled
    }
  }
}

onMounted(loadForms)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">Forms</h1>
        <p class="text-muted-foreground">Create and manage contact forms</p>
      </div>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Create Form
      </Button>
    </div>

    <Card>
      <DataTable
        :columns="columns"
        :data="forms"
        :loading="loading"
        empty-text="No forms found."
      >
        <template #name="{ row }: { row: Form }">
          <span class="font-medium">{{ row.Name }}</span>
        </template>
        <template #slug="{ row }: { row: Form }">
          <code class="text-sm bg-muted px-1.5 py-0.5 rounded">{{ row.Slug }}</code>
        </template>
        <template #fields_count="{ row }: { row: Form }">
          {{ row.Fields?.length || 0 }}
        </template>
        <template #created_at="{ row }: { row: Form }">
          {{ new Date(row.CreatedAt).toLocaleDateString() }}
        </template>
        <template #actions="{ row }: { row: Form }">
          <div class="flex gap-1">
            <button
              class="p-1.5 rounded hover:bg-muted"
              title="Submissions"
              @click="openSubmissions(row)"
            >
              <Eye class="h-4 w-4" />
            </button>
            <button
              class="p-1.5 rounded hover:bg-muted"
              title="Embed"
              @click="openEmbed(row)"
            >
              <Code2 class="h-4 w-4" />
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

    <!-- Create/Edit Modal -->
    <Modal :open="modalOpen" :title="isEdit ? 'Edit Form' : 'Create Form'" @close="modalOpen = false">
      <div class="space-y-4">
        <Input v-model="formName" label="Form Name" placeholder="Contact Form" />
        <Input v-model="formSlug" label="Slug" placeholder="contact-form" />

        <div>
          <div class="flex items-center justify-between mb-2">
            <label class="text-sm font-medium">Fields</label>
            <Button variant="outline" size="sm" @click="addField">
              <Plus class="mr-1 h-3 w-3" /> Add Field
            </Button>
          </div>

          <div class="space-y-3">
            <div
              v-for="(field, index) in formFields"
              :key="index"
              class="flex items-start gap-2 rounded-md border p-3"
            >
              <div class="flex-1 grid grid-cols-2 gap-2">
                <Input
                  v-model="field.label"
                  label="Label"
                  placeholder="Field Label"
                />
                <Input
                  v-model="field.name"
                  label="Name"
                  placeholder="field_name"
                />
                <select
                  v-model="field.type"
                  class="h-9 rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                >
                  <option value="text">Text</option>
                  <option value="email">Email</option>
                  <option value="tel">Phone</option>
                  <option value="textarea">Textarea</option>
                  <option value="number">Number</option>
                  <option value="url">URL</option>
                  <option value="select">Select</option>
                  <option value="file">File</option>
                </select>
                <Input
                  v-if="field.type === 'file'"
                  v-model="field.accept"
                  label="Accepted types"
                  placeholder="image/png, application/pdf"
                />
                <label class="flex items-center gap-2 text-sm pt-4">
                  <input
                    v-model="field.required"
                    type="checkbox"
                    class="rounded border-gray-300"
                  />
                  Required
                </label>
              </div>
              <button
                class="mt-6 p-1 text-destructive hover:bg-muted rounded"
                @click="removeField(index)"
              >
                <Trash2 class="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>

        <div class="rounded-md border p-3 space-y-3">
          <label class="text-sm font-medium">Submission notifications</label>
          <label class="flex items-center gap-2 text-sm">
            <input
              v-model="notifSubmitterEnabled"
              type="checkbox"
              class="rounded border-gray-300"
            />
            Send a copy to the submitter
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input
              v-model="notifAdminEnabled"
              type="checkbox"
              class="rounded border-gray-300"
            />
            Notify admins
          </label>
          <Input
            v-if="notifAdminEnabled"
            v-model="notifAdminTo"
            label="Admin recipients"
            placeholder="admin@example.com, ops@example.com"
          />
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

    <!-- Embed Modal -->
    <Modal :open="embedOpen" title="Embed Form" @close="embedOpen = false">
      <div class="space-y-3">
        <p class="text-sm text-muted-foreground">
          Add this script tag to your page's HTML. The form renders inside an
          element with
          <code class="text-xs bg-muted px-1.5 py-0.5 rounded">[data-automata-form="{{ embedSlug }}"]</code>
          if present, otherwise at the end of the body.
        </p>
        <pre class="rounded-md bg-muted p-3 text-xs overflow-x-auto">{{ embedTag }}</pre>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <Button variant="outline" @click="embedOpen = false">Close</Button>
          <Button @click="copyEmbed">
            <Check v-if="copied" class="mr-2 h-4 w-4" />
            <Copy v-else class="mr-2 h-4 w-4" />
            {{ copied ? 'Copied!' : 'Copy' }}
          </Button>
        </div>
      </template>
    </Modal>

    <ConfirmDialog />
  </div>
</template>
