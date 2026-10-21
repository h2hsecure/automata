<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Card from '@/components/Card.vue'
import SettingsPanel from '@/components/settings/SettingsPanel.vue'
import ApiKeysPanel from '@/components/settings/ApiKeysPanel.vue'
import UsersPanel from '@/components/settings/UsersPanel.vue'
import WebsitesPanel from '@/components/settings/WebsitesPanel.vue'
import McpPanel from '@/components/settings/McpPanel.vue'

const route = useRoute()
const router = useRouter()

const tabs = [
  { id: 'users', label: 'Users' },
  { id: 'settings', label: 'Settings' },
  { id: 'api-keys', label: 'API Keys' },
  { id: 'mcp', label: 'MCP' },
  { id: 'websites', label: 'Websites' },
]

const activeTab = computed(() => {
  const tab = route.query.tab
  return tabs.some((t) => t.id === tab) ? (tab as string) : 'users'
})

function selectTab(id: string) {
  router.push({ query: { tab: id } })
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-4">
      <div>
        <h1 class="text-2xl font-bold">Settings</h1>
        <p class="text-muted-foreground">Manage global platform settings</p>
      </div>
      <div class="flex gap-1 rounded-lg bg-muted p-1">
        <button
          v-for="t in tabs"
          :key="t.id"
          :class="[
            'rounded-md px-3 py-1.5 text-sm font-medium transition-colors',
            activeTab === t.id
              ? 'bg-card text-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground',
          ]"
          @click="selectTab(t.id)"
        >
          {{ t.label }}
        </button>
      </div>
    </div>

    <UsersPanel v-if="activeTab === 'users'" />
    <Card v-else-if="activeTab === 'settings'">
      <SettingsPanel />
    </Card>
    <ApiKeysPanel v-else-if="activeTab === 'api-keys'" />
    <McpPanel v-else-if="activeTab === 'mcp'" />
    <WebsitesPanel v-else-if="activeTab === 'websites'" />
  </div>
</template>
