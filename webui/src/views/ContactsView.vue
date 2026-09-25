<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Pencil, Trash2, Plus, Users, Search } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Input from '@/components/Input.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import SearchBar from '@/components/SearchBar.vue'
import Badge from '@/components/Badge.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { usePagination } from '@/composables/usePagination'
import { listContacts, deleteContact, createContact, updateContact } from '@/api/client'
import type { Contact } from '@/types/api'

const { confirmStore, show, confirm } = useConfirm()
const { page, limit, total, goToPage, setTotal } = usePagination()
const contacts = ref<Contact[]>([])
const loading = ref(false)
const searchQuery = ref('')
const searchDebounce = ref<ReturnType<typeof setTimeout> | null>(null)

// Modal state
const modalOpen = ref(false)
const isEdit = ref(false)
const editingId = ref('')
const form = ref({
  email: '',
  first_name: '',
  last_name: '',
  phone: '',
  tags: [] as Array<{ id: string; name: string; color: string }>,
  custom_fields: {} as Record<string, string>,
})
const formError = ref('')

// Column definitions
const columns = [
  { key: 'name', label: 'Name' },
  { key: 'email', label: 'Email' },
  { key: 'phone', label: 'Phone' },
  { key: 'tags', label: 'Tags' },
  { key: 'created_at', label: 'Created' },
  { key: 'actions', label: 'Actions', width: '120px' },
]

async function loadContacts() {
  loading.value = true
  try {
    const response = await listContacts(page.value, limit.value, searchQuery.value)
    contacts.value = response.data || []
    setTotal(response.total || 0)
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  page.value = 1
  loadContacts()
}

function openCreate() {
  isEdit.value = false
  editingId.value = ''
  form.value = {
    email: '',
    first_name: '',
    last_name: '',
    phone: '',
    tags: [],
    custom_fields: {},
  }
  formError.value = ''
  modalOpen.value = true
}

function openEdit(contact: Contact) {
  isEdit.value = true
  editingId.value = contact.id
  form.value = {
    email: contact.email,
    first_name: contact.first_name,
    last_name: contact.last_name,
    phone: contact.phone || '',
    tags: contact.tags || [],
    custom_fields: { ...contact.custom_fields },
  }
  formError.value = ''
  modalOpen.value = true
}

async function handleSave() {
  if (!form.value.email) {
    formError.value = 'Email is required.'
    return
  }
  loading.value = true
  try {
    if (isEdit.value) {
      await updateContact(editingId.value, form.value)
    } else {
      await createContact(form.value)
    }
    modalOpen.value = false
    loadContacts()
  } catch (e: any) {
    formError.value = e.response?.data?.error || 'Failed to save contact.'
  } finally {
    loading.value = false
  }
}

async function handleDelete(contact: Contact) {
  const confirmed = await show({
    title: 'Delete Contact',
    description: `Are you sure you want to delete ${contact.first_name} ${contact.last_name}? This action cannot be undone.`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteContact(contact.id)
      loadContacts()
    } catch {
      // handled
    }
  }
}

onMounted(loadContacts)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold">Contacts</h1>
        <p class="text-muted-foreground">Manage your contact list</p>
      </div>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Add Contact
      </Button>
    </div>

    <!-- Search -->
    <div class="flex gap-4">
      <div class="w-full max-w-md">
        <SearchBar
          v-model="searchQuery"
          placeholder="Search contacts..."
          @search="handleSearch"
        />
      </div>
    </div>

    <!-- Table -->
    <Card>
      <DataTable
        :columns="columns"
        :data="contacts"
        :loading="loading"
        empty-text="No contacts found."
      >
        <template #name="{ row }: { row: Contact }">
          <div class="flex items-center gap-2">
            <div class="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center text-primary text-xs font-medium">
              {{ row.first_name?.[0] || row.email?.[0] || '?' }}
            </div>
            <div>
              <p class="font-medium">{{ row.first_name }} {{ row.last_name }}</p>
            </div>
          </div>
        </template>
        <template #email="{ row }: { row: Contact }">
          <span class="text-sm">{{ row.email }}</span>
        </template>
        <template #phone="{ row }: { row: Contact }">
          {{ row.phone || '-' }}
        </template>
        <template #tags="{ row }: { row: Contact }">
          <div class="flex gap-1 flex-wrap">
            <Badge
              v-for="tag in row.tags"
              :key="tag.id"
              :variant="'default'"
            >
              {{ tag.name }}
            </Badge>
          </div>
        </template>
        <template #created_at="{ row }: { row: Contact }">
          {{ new Date(row.created_at).toLocaleDateString() }}
        </template>
        <template #actions="{ row }: { row: Contact }">
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

      <!-- Pagination -->
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

    <!-- Create/Edit Modal -->
    <Modal :open="modalOpen" :title="isEdit ? 'Edit Contact' : 'Add Contact'" @close="modalOpen = false">
      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <Input
            v-model="form.first_name"
            label="First Name"
            placeholder="John"
          />
          <Input
            v-model="form.last_name"
            label="Last Name"
            placeholder="Doe"
          />
        </div>
        <Input
          v-model="form.email"
          type="email"
          label="Email"
          placeholder="john@example.com"
          :error="formError && !form.email ? 'Email is required' : ''"
        />
        <Input
          v-model="form.phone"
          type="tel"
          label="Phone"
          placeholder="+1 (555) 000-0000"
        />
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
