<script setup lang="ts">
import { useConfirm } from '@/composables/useConfirm'

const { show, confirm, cancel } = useConfirm()
const emit = defineEmits<{ confirm: [] }>()

async function handleConfirm() {
  confirm()
  emit('confirm')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="show"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
    >
      <div class="w-full max-w-md rounded-lg border bg-card p-6 shadow-lg">
        <h3 class="text-lg font-semibold mb-2">
          {{ show.options?.title }}
        </h3>
        <p class="text-sm text-muted-foreground mb-6">
          {{ show.options?.description }}
        </p>
        <div class="flex justify-end gap-2">
          <Button variant="outline" @click="cancel">
            {{ show.options?.cancelText || 'Cancel' }}
          </Button>
          <Button
            :variant="show.options?.variant || 'default'"
            @click="handleConfirm"
          >
            {{ show.options?.confirmText || 'Confirm' }}
          </Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
