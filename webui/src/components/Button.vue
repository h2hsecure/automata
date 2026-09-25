<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'secondary' | 'destructive' | 'outline'
    size?: 'default' | 'sm' | 'lg' | 'icon'
    disabled?: boolean
    loading?: boolean
    type?: 'button' | 'submit' | 'reset',
  }>(),
  {
    variant: 'default',
    size: 'default',
    disabled: false,
    loading: false,
    type: 'button',
  }
)

const variantClasses: Record<string, string> = {
  default: 'bg-primary text-primary-foreground hover:bg-primary/90',
  secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/80',
  destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90',
  outline: 'border border-input bg-transparent hover:bg-accent hover:text-accent-foreground',
}

const sizeClasses: Record<string, string> = {
  default: 'h-9 px-4 py-2 text-sm',
  sm: 'h-8 rounded-md px-3 text-xs',
  lg: 'h-10 rounded-md px-8 text-base',
  icon: 'h-9 w-9',
}

const className = computed(
  () =>
    `inline-flex items-center justify-center rounded-md font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 disabled:pointer-events-none disabled:opacity-50 ${variantClasses[props.variant]} ${sizeClasses[props.size]}`
)
</script>

<template>
  <button
    :class="className"
    :disabled="disabled || loading"
    :type="type"
  >
    <slot v-if="!loading" />
    <span v-else class="flex items-center gap-2">
      <svg class="h-4 w-4 animate-spin" viewBox="0 0 24 24">
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
          fill="none"
        />
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
        />
      </svg>
      <slot />
    </span>
  </button>
</template>
