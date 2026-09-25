<script setup lang="ts">
import { Bell, LogOut, Menu, Moon, Sun, User, ChevronDown } from 'lucide-vue-next'
import { ref } from 'vue'

const emit = defineEmits<{ 'toggle-sidebar': [] }>()

const { useAuthStore } = await import('@/stores/auth')
const authStore = useAuthStore()
const dropdownOpen = ref(false)

async function handleLogout() {
  await authStore.logout()
  window.location.href = '/login'
}
</script>

<template>
  <header class="flex h-16 items-center border-b bg-card px-6">
    <!-- Mobile menu button -->
    <button
      class="mr-4 p-2 hover:bg-muted rounded lg:hidden"
      @click="emit('toggle-sidebar')"
    >
      <Menu class="h-5 w-5" />
    </button>

    <!-- Title -->
    <h1 class="text-lg font-semibold hidden sm:block">
      {{ $route.meta.title || 'Automata' }}
    </h1>

    <!-- Spacer -->
    <div class="flex-1" />

    <!-- Actions -->
    <div class="flex items-center gap-2">
      <button class="p-2 hover:bg-muted rounded relative">
        <Bell class="h-5 w-5" />
        <span class="absolute top-1 right-1 h-2 w-2 rounded-full bg-primary" />
      </button>

      <button
        class="p-2 hover:bg-muted rounded"
        @click="authStore.theme === 'dark' ? (authStore.theme = 'light') : (authStore.theme = 'dark')"
      >
        <Moon v-if="authStore.theme === 'light'" class="h-5 w-5" />
        <Sun v-else class="h-5 w-5" />
      </button>

      <!-- User dropdown -->
      <div class="relative">
        <button
          class="flex items-center gap-2 rounded-md p-2 hover:bg-muted"
          @click="dropdownOpen = !dropdownOpen"
        >
          <div
            class="h-8 w-8 rounded-full bg-primary flex items-center justify-center text-primary-foreground text-sm font-medium"
          >
            {{ authStore.user?.email?.charAt(0).toUpperCase() || 'U' }}
          </div>
          <span class="hidden md:block text-sm font-medium">
            {{ authStore.user?.email || 'User' }}
          </span>
          <ChevronDown class="h-4 w-4 text-muted-foreground" />
        </button>

        <div
          v-if="dropdownOpen"
          class="absolute right-0 mt-2 w-48 rounded-md border bg-popover shadow-lg py-1 z-50"
        >
          <div class="px-3 py-2 text-sm text-muted-foreground border-b">
            {{ authStore.user?.email }}
          </div>
          <RouterLink
            to="/settings"
            class="flex w-full items-center gap-2 px-3 py-2 text-sm hover:bg-muted"
            @click="dropdownOpen = false"
          >
            <User class="h-4 w-4" />
            Profile
          </RouterLink>
          <button
            class="flex w-full items-center gap-2 px-3 py-2 text-sm text-destructive hover:bg-muted"
            @click="handleLogout"
          >
            <LogOut class="h-4 w-4" />
            Log out
          </button>
        </div>
      </div>
    </div>
  </header>
</template>
