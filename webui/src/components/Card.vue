<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  title?: string
  description?: string
}>()

const className = computed(() =>
  'rounded-lg border bg-card text-card-foreground shadow-sm'
)
</script>

<template>
  <div :class="className">
    <div v-if="title || description" class="flex flex-col space-y-1.5 p-6">
      <h3 v-if="title" class="text-2xl font-semibold leading-none tracking-tight">
        {{ title }}
      </h3>
      <p v-if="description" class="text-sm text-muted-foreground">
        {{ description }}
      </p>
    </div>
    <div class="p-6 pt-0">
      <slot />
    </div>
    <div v-if="$slots.footer" class="flex items-center p-6 pt-0">
      <slot name="footer" />
    </div>
  </div>
</template>
