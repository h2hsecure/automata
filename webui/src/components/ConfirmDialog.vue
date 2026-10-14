<script setup lang="ts">
import { useConfirm } from '@/composables/useConfirm'

const { confirmStore, confirm, cancel } = useConfirm()
const emit = defineEmits<{ confirm: [] }>()

async function handleConfirm() {
  confirm()
  emit('confirm')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="confirmStore.show"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    >
      <div class="w-full max-w-md rounded-lg border bg-card p-6 shadow-lg">
        <h3 class="text-lg font-semibold mb-2">
          {{ confirmStore.options?.title }}
        </h3>
        <p class="text-sm text-muted-foreground mb-6">
          {{ confirmStore.options?.description }}
        </p>
        <div class="flex justify-end gap-2">
          <Button variant="outline" @click="cancel">
            {{ confirmStore.options?.cancelText || 'Cancel' }}
          </Button>
          <Button
            :variant="confirmStore.options?.variant || 'default'"
            @click="handleConfirm"
          >
            {{ confirmStore.options?.confirmText || 'Confirm' }}
          </Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
