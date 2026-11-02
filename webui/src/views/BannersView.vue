<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { Pencil, Trash2, Plus, Eye, MousePointerClick, BarChart3 } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import DataTable from '@/components/DataTable.vue'
import Badge from '@/components/Badge.vue'
import Modal from '@/components/Modal.vue'
import Input from '@/components/Input.vue'
import Textarea from '@/components/Textarea.vue'
import ConfirmDialog from '@/components/ConfirmDialog.vue'
import { useConfirm } from '@/composables/useConfirm'
import { useAuthStore } from '@/stores/auth'
import { listBanners, createBanner, updateBanner, deleteBanner, listBannerDecisions } from '@/api/client'
import type { Banner, BannerInput, BannerButton, BannerDecision } from '@/types/api'

const authStore = useAuthStore()
const { confirmStore, show, confirm } = useConfirm()
const banners = ref<Banner[]>([])
const loading = ref(true)
const modalOpen = ref(false)
const isEdit = ref(false)
const editingId = ref('')
const bannerName = ref('')
const bannerPlacement = ref('')
const bannerContentType = ref('html')
const bannerContent = ref('')
const bannerLinkUrl = ref('')
const bannerStatus = ref('active')
const bannerPosition = ref('')
const bannerButtons = ref<BannerButton[]>([])
const formError = ref('')

// Decisions modal state
const decisionsModalOpen = ref(false)
const decisionsBanner = ref<Banner | null>(null)
const decisions = ref<BannerDecision[]>([])
const decisionCounts = ref<Record<string, number>>({})
const decisionsLoading = ref(false)

const decisionColumns = [
  { key: 'visitor_id', label: 'Visitor' },
  { key: 'action', label: 'Action' },
  { key: 'ip_hash', label: 'IP Hash' },
  { key: 'created_at', label: 'Decided At' },
]

const columns = [
  { key: 'name', label: 'Name' },
  { key: 'type', label: 'Type' },
  { key: 'placement', label: 'Placement' },
  { key: 'status', label: 'Status' },
  { key: 'created_at', label: 'Created' },
  { key: 'actions', label: 'Actions', width: '100px' },
]

async function loadBanners() {
  loading.value = true
  try {
    banners.value = await listBanners(authStore.currentContextId)
  } finally {
    loading.value = false
  }
}

function addButtonRow() {
  bannerButtons.value.push({ label: '', action: '', link_url: '', dismisses: false })
}

function removeButtonRow(index: number) {
  bannerButtons.value.splice(index, 1)
}

async function openDecisions(b: Banner) {
  decisionsBanner.value = b
  decisionsModalOpen.value = true
  decisionsLoading.value = true
  decisions.value = []
  decisionCounts.value = {}
  try {
    const data = await listBannerDecisions(b.ID)
    decisions.value = data.decisions ?? []
    decisionCounts.value = data.counts ?? {}
  } finally {
    decisionsLoading.value = false
  }
}

watch(
  () => authStore.currentContextId,
  () => loadBanners()
)

function openCreate() {
  isEdit.value = false
  editingId.value = ''
  bannerName.value = ''
  bannerPlacement.value = ''
  bannerContentType.value = 'html'
  bannerContent.value = ''
  bannerLinkUrl.value = ''
  bannerStatus.value = 'active'
  bannerPosition.value = ''
  bannerButtons.value = []
  formError.value = ''
  modalOpen.value = true
}

function openEdit(b: Banner) {
  isEdit.value = true
  editingId.value = b.ID
  bannerName.value = b.Name
  bannerPlacement.value = b.Placements?.[0] ?? ''
  bannerContentType.value = b.Type
  bannerContent.value = b.Content
  bannerLinkUrl.value = b.LinkURL
  bannerStatus.value = b.IsActive ? 'active' : 'inactive'
  bannerPosition.value = b.Position ?? ''
  bannerButtons.value = (b.Buttons ?? []).map((btn) => ({ ...btn }))
  formError.value = ''
  modalOpen.value = true
}

async function handleSave() {
  if (!bannerName.value.trim()) {
    formError.value = 'Name is required.'
    return
  }
  loading.value = true
  try {
    const data: BannerInput = {
      name: bannerName.value,
      type: bannerContentType.value,
      content: bannerContent.value,
      link_url: bannerLinkUrl.value,
      placements: bannerPlacement.value ? [bannerPlacement.value] : [],
      position: bannerPosition.value,
      buttons: bannerButtons.value.filter(
        (b) => b.label.trim() || b.action.trim()
      ),
    }
    if (isEdit.value) {
      data.is_active = bannerStatus.value === 'active'
    }
    if (isEdit.value) {
      await updateBanner(editingId.value, data)
    } else {
      await createBanner(data)
    }
    modalOpen.value = false
    loadBanners()
  } catch (e: any) {
    formError.value = e.response?.data?.error || 'Failed to save banner.'
  } finally {
    loading.value = false
  }
}

async function handleDelete(b: Banner) {
  const confirmed = await show({
    title: 'Delete Banner',
    description: `Are you sure you want to delete "${b.Name}"?`,
    variant: 'destructive',
  })
  if (confirmed) {
    try {
      await deleteBanner(b.ID)
      loadBanners()
    } catch {
      // handled
    }
  }
}

onMounted(loadBanners)
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between">
      <div>
        <h1 class="text-2xl font-bold">Banners</h1>
        <p class="text-muted-foreground">Manage promotional banners and campaigns</p>
      </div>
      <Button @click="openCreate">
        <Plus class="mr-2 h-4 w-4" />
        Create Banner
      </Button>
    </div>

    <Card>
      <DataTable
        :columns="columns"
        :data="banners"
        :loading="loading"
        empty-text="No banners found."
      >
        <template #name="{ row }: { row: Banner }">
          <span class="font-medium">{{ row.Name }}</span>
        </template>
        <template #type="{ row }: { row: Banner }">
          <Badge variant="info">{{ row.Type }}</Badge>
        </template>
        <template #placement="{ row }: { row: Banner }">
          {{ row.Placements?.[0] || '-' }}
        </template>
        <template #status="{ row }: { row: Banner }">
          <Badge :variant="row.IsActive ? 'success' : 'default'">
            {{ row.IsActive ? 'active' : 'inactive' }}
          </Badge>
        </template>
        <template #created_at="{ row }: { row: Banner }">
          {{ new Date(row.CreatedAt).toLocaleDateString() }}
        </template>
        <template #actions="{ row }: { row: Banner }">
          <div class="flex gap-1">
            <button
              class="p-1.5 rounded hover:bg-muted"
              @click="openDecisions(row)"
              aria-label="View decisions"
            >
              <BarChart3 class="h-4 w-4" />
            </button>
            <button
              class="p-1.5 rounded hover:bg-muted"
              @click="openEdit(row)"
            >
              <Pencil class="h-4 w-4" />
            </button>
            <button
              class="p-1.5 rounded hover:bg-muted text-destructive"
              @click="handleDelete(row)"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </div>
        </template>
      </DataTable>
    </Card>

    <!-- Modal -->
    <Modal :open="modalOpen" :title="isEdit ? 'Edit Banner' : 'Create Banner'" @close="modalOpen = false">
      <div class="space-y-4">
        <Input v-model="bannerName" label="Banner Name" placeholder="Summer Sale Banner" />
        <Input v-model="bannerPlacement" label="Placement" placeholder="hero-top" />

        <div>
          <label class="text-sm font-medium mb-1 block">Content Type</label>
          <select
            v-model="bannerContentType"
            class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
          >
            <option value="html">HTML</option>
            <option value="image">Image</option>
            <option value="video">Video</option>
            <option value="popup">Popup</option>
            <option value="sticky">Sticky</option>
          </select>
        </div>

        <div>
          <label class="text-sm font-medium mb-1 block">Position</label>
          <select
            v-model="bannerPosition"
            class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
          >
            <option value="">Legacy (placement injection)</option>
            <option value="top">Top bar</option>
            <option value="bottom">Bottom bar</option>
            <option value="fullpage">Full page</option>
          </select>
        </div>

        <div>
          <label class="text-sm font-medium mb-1 block">
            Buttons
            <span class="text-slate-400 font-normal">(optional — used by the banner snippet)</span>
          </label>
          <div class="space-y-2">
            <div v-for="(btn, i) in bannerButtons" :key="i" class="grid grid-cols-12 gap-2 items-end">
              <div class="col-span-4"><Input v-model="btn.label" placeholder="Accept" :label="i === 0 ? 'Label' : ''" /></div>
              <div class="col-span-3"><Input v-model="btn.action" placeholder="accept" :label="i === 0 ? 'Action' : ''" /></div>
              <div class="col-span-3"><Input v-model="btn.link_url" placeholder="/cookie-policy" :label="i === 0 ? 'Link URL' : ''" /></div>
              <div class="col-span-2 flex items-center gap-3">
                <label class="flex items-center gap-2 text-sm text-slate-700">
                  <input type="checkbox" v-model="btn.dismisses" class="rounded border-slate-300" />
                  <span v-if="i === 0">Dismisses</span>
                </label>
                <button class="p-1.5 rounded hover:bg-muted text-destructive" @click="removeButtonRow(i)" :aria-label="'Remove button ' + (i + 1)">
                  <Trash2 class="h-4 w-4" />
                </button>
              </div>
            </div>
            <Button variant="ghost" size="sm" @click="addButtonRow">
              <Plus class="mr-1 h-4 w-4" /> Add button
            </Button>
          </div>
        </div>

        <Textarea
          v-model="bannerContent"
          label="Content"
          :rows="6"
          placeholder="Enter HTML or content..."
        />

        <Input v-model="bannerLinkUrl" label="Link URL" placeholder="https://example.com" />

        <div>
          <label class="text-sm font-medium mb-1 block">Status</label>
          <select
            v-model="bannerStatus"
            class="h-9 w-full rounded-md border bg-transparent px-3 py-1 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
          >
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
          </select>
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

    <!-- Decisions modal -->
    <Modal :open="decisionsModalOpen" :title="'Decisions — ' + (decisionsBanner?.Name ?? '')" @close="decisionsModalOpen = false">
      <div class="space-y-4">
        <div v-if="decisionsLoading" class="text-sm text-muted-foreground py-4 text-center">Loading decisions…</div>
        <template v-else>
          <div class="flex flex-wrap gap-2">
            <Badge v-for="(count, action) in decisionCounts" :key="action" variant="info">
              {{ action }}: {{ count }}
            </Badge>
            <span v-if="Object.keys(decisionCounts).length === 0" class="text-sm text-muted-foreground">
              No decisions recorded yet.
            </span>
          </div>
          <DataTable
            :columns="decisionColumns"
            :data="decisions"
            empty-text="No decisions yet."
          >
            <template #action="{ row }: { row: BannerDecision }">
              <Badge variant="default">{{ row.Action }}</Badge>
            </template>
            <template #created_at="{ row }: { row: BannerDecision }">
              {{ new Date(row.CreatedAt).toLocaleString() }}
            </template>
          </DataTable>
        </template>
      </div>
    </Modal>

    <ConfirmDialog />
  </div>
</template>
