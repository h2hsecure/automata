<script setup lang="ts">
import { Search, X } from 'lucide-vue-next'
import { ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue?: string
    placeholder?: string
    debounce?: number
  }>(),
  {
    modelValue: '',
    placeholder: 'Search...',
    debounce: 300,
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  search: [value: string]
}>()

const localValue = ref(props.modelValue)
let timeout: ReturnType<typeof setTimeout> | null = null

watch(localValue, (val) => {
  emit('update:modelValue', val)
  if (timeout) clearTimeout(timeout)
  timeout = setTimeout(() => {
    emit('search', val)
  }, props.debounce)
})

watch(
  () => props.modelValue,
  (val) => {
    if (val !== localValue.value) localValue.value = val
  }
)

function clear() {
  localValue.value = ''
  emit('update:modelValue', '')
  emit('search', '')
}
</script>

<template>
  <div class="relative">
    <Search class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
    <input
      v-model="localValue"
      :placeholder="placeholder"
      class="h-9 w-full rounded-md border bg-transparent pl-9 pr-9 text-sm outline-none placeholder:text-muted-foreground focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-primary/20"
    />
    <button
      v-if="localValue"
      class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
      @click="clear"
    >
      <X class="h-4 w-4" />
    </button>
  </div>
</template>
