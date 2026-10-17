<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { Pencil, Trash2, Plus, UserPlus } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Modal from '@/components/Modal.vue'
import Input from '@/components/Input.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import {
  listUsers,
  createUser,
  updateUser,
  deleteUser,
  listContexts,
  listContextUsers,
  assignContextUser,
  removeContextUser,
} from '@/api/client'
import type { User, Context, ContextUser } from '@/types/api'

const { show } = useConfirm()

// Users
const users = ref<User[]>([])
const loading = ref(false)
const saving = ref(false)
const modalOpen = ref(false)
const isEdit = ref(false)
const editingId = ref('')
const email = ref('')
const password = ref('')
const isAdmin = ref(false)
const formError = ref('')

const userColumns = [
  { key: 'email', label: 'Email' },
  { key: 'is_admin', label: 'Role' },
  { key: 'created_at', label: 'Created' },
  { key: 'actions', label: 'Actions', width: '100px' },
]

async function loadUsers() {
  loading.value = true
  try {
    users.value = await listUsers()
  } finally {
    loading.value = false
  }
}

function openCreate() {
  isEdit.value = false
  editingId.value = ''
  email.value = ''
  password.value = ''
  isAdmin.value = false
  formError.value = ''
  modalOpen.value = true
}

function openEdit(u: User) {
  isEdit.value = true
  editingId.value = u.id
  email.value = u.email
  password.value = ''
  isAdmin.value = u.is_admin
  formError.value = ''
  modalOpen.value = true
}

async function handleSave() {
  if (!email.value.trim()) {
    formError.value = 'Email is required.'
    return
  }
  if (!isEdit.value && !password.value) {
    formError.value = 'Password is required.'
    return
  }
  saving.value = true
  try {
    if (isEdit.value) {
      const data: { email?: string; password?: string; is_admin?: boolean } = {
        email: email.value.trim(),
        is_admin: isAdmin.value,
      }
      if (password.value) data.password = password.value
      await updateUser(editingId.value, data)
    } else {
      await createUser({ email: email.value.trim(), password: password.value, is_admin: isAdmin.value })
    }
    modalOpen.value = false
    await loadUsers()
  } catch (e: any) {
    formError.value = e.response?.data?.error || 'Failed to save user.'
  } finally {
    saving.value = false
  }
}

async function handleDelete(u: User) {
  const confirmed = await show({
    title: 'Delete User',
    description: `Are you sure you want to delete "${u.email}"?`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteUser(u.id)
      await loadUsers()
    } catch {
      // handled
    }
  }
}

// Website memberships
const contexts = ref<Context[]>([])
const selectedContextId = ref('')
const members = ref<ContextUser[]>([])
const memberLoading = ref(false)
const addingMember = ref(false)
const newMemberUserId = ref('')
const newMemberRole = ref('member')
const memberError = ref('')

const userEmailMap = computed(() => {
  const map = new Map<string, string>()
  for (const u of users.value) map.set(u.id, u.email)
  return map
})

function memberEmail(m: ContextUser) {
  return userEmailMap.value.get(m.UserID) ?? m.UserID
}

function roleVariant(role: string) {
  if (role === 'owner') return 'warning'
  if (role === 'admin') return 'info'
  return 'default'
}

const memberColumns = [
  { key: 'email', label: 'Email' },
  { key: 'Role', label: 'Role' },
  { key: 'actions', label: 'Actions', width: '80px' },
]

async function loadContexts() {
  try {
    contexts.value = await listContexts(0, 1000)
  } catch {
    // handled: select stays empty
  }
}

async function loadMembers() {
  if (!selectedContextId.value) {
    members.value = []
    return
  }
  memberLoading.value = true
  try {
    members.value = await listContextUsers(selectedContextId.value)
  } finally {
    memberLoading.value = false
  }
}

watch(selectedContextId, loadMembers)

async function handleAddMember() {
  if (!newMemberUserId.value) {
    memberError.value = 'Select a user.'
    return
  }
  addingMember.value = true
  try {
    await assignContextUser(selectedContextId.value, {
      user_id: newMemberUserId.value,
      role: newMemberRole.value,
    })
    newMemberUserId.value = ''
    newMemberRole.value = 'member'
    memberError.value = ''
    await loadMembers()
  } catch (e: any) {
    memberError.value = e.response?.data?.error || 'Failed to add member.'
  } finally {
    addingMember.value = false
  }
}

async function handleRemoveMember(m: ContextUser) {
  const confirmed = await show({
    title: 'Remove Member',
    description: `Are you sure you want to remove "${memberEmail(m)}" from this website?`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await removeContextUser(selectedContextId.value, m.UserID)
      await loadMembers()
    } catch {
      // handled
    }
  }
}

onMounted(() => {
  loadUsers()
  loadContexts()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">Users</h1>
        <p class="text-muted-foreground">Manage platform users and website access</p>
      </div>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Add User
      </Button>
    </div>

    <Card>
      <DataTable
        :columns="userColumns"
        :data="users"
        :loading="loading"
        empty-text="No users found."
      >
        <template #email="{ row }: { row: User }">
          <span class="font-medium">{{ row.email }}</span>
        </template>
        <template #is_admin="{ row }: { row: User }">
          <Badge :text="row.is_admin ? 'Admin' : 'User'" :variant="row.is_admin ? 'info' : 'default'" />
        </template>
        <template #created_at="{ row }: { row: User }">
          {{ new Date(row.created_at).toLocaleDateString() }}
        </template>
        <template #actions="{ row }: { row: User }">
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

    <Card>
      <div class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <p class="text-sm text-muted-foreground">
            Manage which users can access each website
          </p>
          <select
            v-model="selectedContextId"
            class="max-w-[220px] truncate rounded-md border bg-background px-2 py-1.5 text-sm"
          >
            <option value="">Select a website…</option>
            <option v-for="ctx in contexts" :key="ctx.ID" :value="ctx.ID">
              {{ ctx.Name }}
            </option>
          </select>
        </div>

        <template v-if="selectedContextId">
          <DataTable
            :columns="memberColumns"
            :data="members"
            :loading="memberLoading"
            empty-text="No members for this website."
          >
            <template #email="{ row }: { row: ContextUser }">
              <span class="font-medium">{{ memberEmail(row) }}</span>
            </template>
            <template #Role="{ row }: { row: ContextUser }">
              <Badge :text="row.Role" :variant="roleVariant(row.Role)" />
            </template>
            <template #actions="{ row }: { row: ContextUser }">
              <button
                class="p-1.5 rounded hover:bg-muted text-destructive"
                title="Remove"
                @click="handleRemoveMember(row)"
              >
                <Trash2 class="h-4 w-4" />
              </button>
            </template>
          </DataTable>

          <div class="flex flex-wrap items-center gap-2 border-t pt-4">
            <select
              v-model="newMemberUserId"
              class="max-w-[220px] truncate rounded-md border bg-background px-2 py-1.5 text-sm"
            >
              <option value="">Select a user…</option>
              <option v-for="u in users" :key="u.id" :value="u.id">
                {{ u.email }}
              </option>
            </select>
            <select
              v-model="newMemberRole"
              class="rounded-md border bg-background px-2 py-1.5 text-sm"
            >
              <option value="owner">Owner</option>
              <option value="admin">Admin</option>
              <option value="member">Member</option>
            </select>
            <Button size="sm" :loading="addingMember" @click="handleAddMember">
              <UserPlus class="mr-2 h-4 w-4" />
              Add Member
            </Button>
            <p v-if="memberError" class="text-sm text-destructive">{{ memberError }}</p>
          </div>
        </template>
      </div>
    </Card>

    <!-- User Modal -->
    <Modal :open="modalOpen" :title="isEdit ? 'Edit User' : 'Add User'" @close="modalOpen = false">
      <div class="space-y-4">
        <Input v-model="email" label="Email" placeholder="user@example.com" />
        <Input
          v-model="password"
          type="password"
          :label="isEdit ? 'New Password (optional)' : 'Password'"
          :placeholder="isEdit ? 'Leave blank to keep current password' : 'Choose a password'"
        />
        <div class="flex items-center gap-2">
          <input
            v-model="isAdmin"
            type="checkbox"
            class="rounded border-gray-300"
          />
          <label class="text-sm font-medium">Admin</label>
        </div>
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
