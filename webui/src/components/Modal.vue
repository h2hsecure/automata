<script setup lang="ts">
import { X } from 'lucide-vue-next'

defineProps<{
  open: boolean
  title?: string
}>()

const emit = defineEmits<{ close: [] }>()
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="emit('close')"
    >
      <div class="w-full max-w-lg rounded-lg border bg-card shadow-lg">
        <div class="flex items-center justify-between border-b p-4">
          <h3 class="text-lg font-semibold">{{ title }}</h3>
          <button
            class="rounded p-1 hover:bg-muted"
            @click="emit('close')"
          >
            <X class="h-4 w-4" />
          </button>
        </div>
        <div class="p-4">
          <slot />
        </div>
        <div v-if="$slots.footer" class="border-t p-4">
          <slot name="footer" />
        </div>
      </div>
    </div>
  </Teleport>
</template>
