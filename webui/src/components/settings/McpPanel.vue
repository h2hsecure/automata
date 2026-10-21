<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Copy, Check, Download, Eye, EyeOff } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import { fetchMcpSkill } from '@/api/client'

const endpoint = computed(() => `${window.location.origin}/api/mcp`)

const connectionJson = computed(
  () => JSON.stringify(
    { url: endpoint.value, headers: { Authorization: 'Bearer <YOUR_API_KEY>' } },
    null,
    2,
  ),
)

const skillMd = ref('')
const skillLoaded = ref(false)
const skillVisible = ref(true)
const copied = ref<'' | 'endpoint' | 'json' | 'skill'>('')

function copy(text: string, which: 'endpoint' | 'json' | 'skill') {
  navigator.clipboard.writeText(text)
  copied.value = which
  setTimeout(() => {
    if (copied.value === which) copied.value = ''
  }, 2000)
}

function downloadSkill() {
  if (!skillMd.value) return
  const url = URL.createObjectURL(new Blob([skillMd.value], { type: 'text/markdown' }))
  const a = document.createElement('a')
  a.href = url
  a.download = 'automata-mcp.md'
  a.click()
  URL.revokeObjectURL(url)
}

onMounted(async () => {
  try {
    skillMd.value = await fetchMcpSkill()
    skillLoaded.value = true
  } catch {
    // 401 is handled by the client interceptor (redirects to login)
  }
})
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold">MCP</h1>
      <p class="text-muted-foreground">Connect MCP clients to this tenant's automation tools</p>
    </div>

    <Card>
      <div class="space-y-4">
        <div>
          <h2 class="font-semibold">Endpoint</h2>
          <p class="text-sm text-muted-foreground">
            Stateless streamable-HTTP MCP endpoint. Authenticate with any API key of this tenant
            (see the API Keys tab).
          </p>
        </div>
        <div class="flex items-center gap-2">
          <code class="flex-1 truncate rounded-md bg-muted px-3 py-2 text-sm">{{ endpoint }}</code>
          <Button variant="outline" size="sm" @click="copy(endpoint, 'endpoint')">
            <Check v-if="copied === 'endpoint'" class="mr-2 h-4 w-4" />
            <Copy v-else class="mr-2 h-4 w-4" />
            {{ copied === 'endpoint' ? 'Copied!' : 'Copy' }}
          </Button>
        </div>
        <div>
          <h3 class="mb-1 text-sm font-medium">Connection config</h3>
          <pre class="rounded-md bg-muted p-3 text-xs overflow-x-auto">{{ connectionJson }}</pre>
          <div class="mt-2">
            <Button variant="outline" size="sm" @click="copy(connectionJson, 'json')">
              <Check v-if="copied === 'json'" class="mr-2 h-4 w-4" />
              <Copy v-else class="mr-2 h-4 w-4" />
              {{ copied === 'json' ? 'Copied!' : 'Copy' }}
            </Button>
          </div>
        </div>
      </div>
    </Card>

    <Card>
      <div class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <div>
            <h2 class="font-semibold">Skill file</h2>
            <p class="text-sm text-muted-foreground">
              Full usage guide for the MCP tools — add it to your agent's instructions.
            </p>
          </div>
          <div class="flex gap-2">
            <Button variant="outline" size="sm" @click="skillVisible = !skillVisible">
              <Eye v-if="skillVisible" class="mr-2 h-4 w-4" />
              <EyeOff v-else class="mr-2 h-4 w-4" />
              {{ skillVisible ? 'Hide' : 'View' }}
            </Button>
            <Button variant="outline" size="sm" @click="copy(skillMd, 'skill')">
              <Check v-if="copied === 'skill'" class="mr-2 h-4 w-4" />
              <Copy v-else class="mr-2 h-4 w-4" />
              {{ copied === 'skill' ? 'Copied!' : 'Copy' }}
            </Button>
            <Button variant="outline" size="sm" @click="downloadSkill">
              <Download class="mr-2 h-4 w-4" />
              Download
            </Button>
          </div>
        </div>
        <pre
          v-if="skillLoaded && skillVisible"
          class="max-h-96 overflow-auto rounded-md bg-muted p-3 text-xs whitespace-pre-wrap"
        >{{ skillMd }}</pre>
        <p v-else-if="!skillLoaded" class="text-sm text-muted-foreground">Loading…</p>
      </div>
    </Card>
  </div>
</template>