import { ref, computed } from 'vue'

export function usePagination(initialPage = 1, initialLimit = 20) {
  const page = ref(initialPage)
  const limit = ref(initialLimit)
  const total = ref(0)

  const totalPages = computed(() => Math.ceil(total.value / limit.value))

  function goToPage(p: number) {
    const clamped = Math.max(1, Math.min(p, totalPages.value))
    page.value = clamped
  }

  function setTotal(t: number) {
    total.value = t
  }

  function reset() {
    page.value = initialPage
    limit.value = initialLimit
    total.value = 0
  }

  return {
    page,
    limit,
    total,
    totalPages,
    goToPage,
    setTotal,
    reset,
  }
}
