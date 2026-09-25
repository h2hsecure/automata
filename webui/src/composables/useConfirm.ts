import { ref } from 'vue'

interface ConfirmOptions {
  title: string
  description: string
  confirmText?: string
  cancelText?: string
  variant?: 'default' | 'destructive'
}

const confirmStore = ref<{
  show: boolean
  options: ConfirmOptions | null
  resolve: ((value: boolean) => void) | null
}>({
  show: false,
  options: null,
  resolve: null,
})

let pendingResolve: ((value: boolean) => void) | null = null

export function useConfirm() {
  function show(options: ConfirmOptions): Promise<boolean> {
    return new Promise((resolve) => {
      pendingResolve = resolve
      confirmStore.value = {
        show: true,
        options: {
          confirmText: 'Confirm',
          cancelText: 'Cancel',
          ...options,
        },
        resolve,
      }
    })
  }

  function confirm() {
    if (pendingResolve) {
      pendingResolve(true)
      pendingResolve = null
    }
    confirmStore.value.show = false
  }

  function cancel() {
    if (pendingResolve) {
      pendingResolve(false)
      pendingResolve = null
    }
    confirmStore.value.show = false
  }

  return {
    confirmStore,
    show,
    confirm,
    cancel,
  }
}
