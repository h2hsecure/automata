<script setup lang="ts">
import { FileSearch } from 'lucide-vue-next'

defineProps<{
  title?: string
  description?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center py-12 text-center">
    <FileSearch class="mb-4 h-12 w-12 text-muted-foreground/50" />
    <h3 class="mb-1 text-lg font-medium">{{ title || 'No results' }}</h3>
    <p class="text-sm text-muted-foreground">{{ description || 'Try adjusting your search or filters.' }}</p>
  </div>
</template>
