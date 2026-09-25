<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  text: string
  variant?: 'default' | 'success' | 'warning' | 'destructive' | 'info'
}>()

const variantClasses: Record<string, string> = {
  default: 'bg-muted text-muted-foreground',
  success: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/30 dark:text-emerald-400',
  warning: 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400',
  destructive: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
  info: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
}

const className = computed(() =>
  `inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${variantClasses[props.variant || 'default']}`
)
</script>

<template>
  <span :class="className">
    <slot>{{ text }}</slot>
  </span>
</template>
