<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { Zap, Mail, Lock, AlertCircle } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Input from '@/components/Input.vue'

const router = useRouter()
const email = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  if (!email.value || !password.value) {
    error.value = 'Please enter both email and password.'
    return
  }

  loading.value = true
  error.value = ''

  try {
    const { useAuthStore } = await import('@/stores/auth')
    const authStore = useAuthStore()
    await authStore.login(email.value, password.value)
    const redirect = router.currentRoute.value.query.redirect as string || '/dashboard'
    router.push(redirect)
  } catch (e: any) {
    error.value = e.response?.data?.error || 'Login failed. Please check your credentials.'
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
        <h1 class="text-2xl font-bold text-center">Automata</h1>
        <p class="text-sm text-muted-foreground">Sign in to your account</p>
      </div>

      <!-- Error -->
      <div v-if="error" class="flex items-center gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
        <AlertCircle class="h-4 w-4 flex-shrink-0" />
        <span>{{ error }}</span>
      </div>

      <!-- Form -->
      <form @submit.prevent="handleLogin" class="space-y-4">
        <Input
          v-model="email"
          type="email"
          label="Email"
          placeholder="admin@automata.local"
          :disabled="loading"
        />
        <Input
          v-model="password"
          type="password"
          label="Password"
          placeholder="••••••••"
          :disabled="loading"
        />
        <Button
          type="submit"
          class="w-full"
          :loading="loading"
        >
          Sign in
        </Button>
      </form>

      </div>
  </div>
</template>
