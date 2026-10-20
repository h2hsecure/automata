<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Zap, Globe, AlertCircle } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Input from '@/components/Input.vue'
import { createContext } from '@/api/client'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const name = ref('')
const loading = ref(false)
const error = ref('')

function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

async function handleCreate() {
  const trimmed = name.value.trim()
  if (!trimmed) {
    error.value = 'Please enter a name for your website.'
    return
  }

  loading.value = true
  error.value = ''

  try {
    const ctx = await createContext({ name: trimmed, slug: slugify(trimmed) })
    authStore.contexts.push({ id: ctx.ID, slug: ctx.Slug, name: ctx.Name })
    localStorage.setItem('contexts', JSON.stringify(authStore.contexts))
    authStore.currentContextId = ctx.ID
    localStorage.setItem('context_id', ctx.ID)
    router.push('/dashboard')
  } catch (e: any) {
    error.value = e.response?.data?.error || 'Failed to create website. Please try again.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="flex min-h-screen items-center justify-center bg-muted/30 px-4">
    <div class="w-full max-w-sm space-y-6 rounded-lg border bg-card p-8 shadow-sm">
      <!-- Logo -->
      <div class="flex flex-col items-center space-y-2">
        <div class="flex h-12 w-12 items-center justify-center rounded-lg bg-primary">
          <Zap class="h-6 w-6 text-primary-foreground" />
        </div>
        <h1 class="text-2xl font-bold text-center">Welcome to Automata</h1>
        <p class="text-sm text-muted-foreground">Set up your first website to get started</p>
      </div>

      <!-- Error -->
      <div v-if="error" class="flex items-center gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
        <AlertCircle class="h-4 w-4 flex-shrink-0" />
        <span>{{ error }}</span>
      </div>

      <!-- Form -->
      <form @submit.prevent="handleCreate" class="space-y-4">
        <Input
          v-model="name"
          label="Website name"
          placeholder="e.g. My Website"
          :disabled="loading"
        />
        <p class="flex items-center gap-1.5 text-xs text-muted-foreground">
          <Globe class="h-3.5 w-3.5" />
          URL slug: {{ slugify(name) || '—' }}
        </p>
        <Button
          type="submit"
          class="w-full"
          :loading="loading"
        >
          Create website
        </Button>
      </form>
    </div>
  </div>
</template>