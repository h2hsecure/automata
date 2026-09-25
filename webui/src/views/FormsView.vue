<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Pencil, Trash2, Plus, ExternalLink } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Modal from '@/components/Modal.vue'
import Input from '@/components/Input.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { listForms, deleteForm, createForm, updateForm } from '@/api/client'
import type { Form } from '@/types/api'

const { confirmStore, show, confirm } = useConfirm()
const forms = ref<Form[]>([])
const loading = ref(false)
const modalOpen = ref(false)
const isEdit = ref(false)
const editingId = ref('')
const formName = ref('')
const formSlug = ref('')
const formFields = ref<Array<{ name: string; type: string; required: boolean; label: string }>>([])
const formError = ref('')

const columns = [
  { key: 'name', label: 'Name' },
  { key: 'slug', label: 'Slug' },
  { key: 'fields_count', label: 'Fields' },
  { key: 'created_at', label: 'Created' },
  { key: 'actions', label: 'Actions', width: '100px' },
]

async function loadForms() {
  loading.value = true
  try {
    forms.value = await listForms()
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  editingId.value = ''
  formName.value = ''
  formSlug.value = ''
  formFields.value = [
    { name: 'email', type: 'email', required: true, label: 'Email' },
    { name: 'name', type: 'text', required: true, label: 'Full Name' },
  ]
  formError.value = ''
  modalOpen.value = true
}

function openEdit(f: Form) {
  isEdit.value = true
  editingId.value = f.id
  formName.value = f.name
  formSlug.value = f.slug
  formFields.value = [...f.fields]
  formError.value = ''
  modalOpen.value = true
}

function addField() {
  formFields.value.push({ name: '', type: 'text', required: false, label: 'New Field' })
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
    const data = {
      name: formName.value,
      slug: formSlug.value || formName.value.toLowerCase().replace(/\s+/g, '-'),
      fields: formFields.value,
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

async function handleDelete(f: Form) {
  const confirmed = await show({
    title: 'Delete Form',
    description: `Are you sure you want to delete "${f.name}"? This action cannot be undone.`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteForm(f.id)
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
          <span class="font-medium">{{ row.name }}</span>
        </template>
        <template #slug="{ row }: { row: Form }">
          <code class="text-sm bg-muted px-1.5 py-0.5 rounded">{{ row.slug }}</code>
        </template>
        <template #fields_count="{ row }: { row: Form }">
          {{ row.fields?.length || 0 }}
        </template>
        <template #created_at="{ row }: { row: Form }">
          {{ new Date(row.created_at).toLocaleDateString() }}
        </template>
        <template #actions="{ row }: { row: Form }">
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
                </select>
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
