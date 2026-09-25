import { ref, computed } from 'vue'

export function useSearch(debounceMs = 300) {
  const query = ref('')
  const results = ref<any[]>([])
  const loading = ref(false)
  let timeout: ReturnType<typeof setTimeout> | null = null

  const hasQuery = computed(() => query.value.trim().length > 0)

  function search(fn: (q: string) => Promise<any[]>) {
    if (timeout) clearTimeout(timeout)
    timeout = setTimeout(async () => {
      loading.value = true
      try {
        results.value = await fn(query.value.trim())
      } finally {
        loading.value = false
      }
    }, debounceMs)
  }

  function clear() {
    query.value = ''
    results.value = []
  }

  return {
    query,
    results,
    loading,
    hasQuery,
    search,
    clear,
  }
}
