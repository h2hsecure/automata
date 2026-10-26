import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import AppLayout from './AppLayout.vue'

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
    ],
  })
}

async function mountLayout() {
  const router = makeRouter()
  router.push('/')
  await router.isReady()
  const wrapper = mount(AppLayout, {
    global: {
      plugins: [createPinia(), router],
    },
  })
  await flushPromises()
  return wrapper
}

describe('AppLayout', () => {
  const originalWidth = window.innerWidth

  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
  })

  afterEach(() => {
    window.innerWidth = originalWidth
  })

  it('starts with the sidebar closed (off-canvas) on viewports narrower than 1024px', async () => {
    window.innerWidth = 768
    const wrapper = await mountLayout()

    const aside = wrapper.find('aside')
    expect(aside.exists()).toBe(true)
    // Closed on mobile means translated fully off-screen — no overlap with page content.
    expect(aside.classes()).toContain('translate-x-[-100%]')
  })

  it('starts with the sidebar open on viewports 1024px and wider', async () => {
    window.innerWidth = 1280
    const wrapper = await mountLayout()

    const aside = wrapper.find('aside')
    expect(aside.exists()).toBe(true)
    expect(aside.classes()).not.toContain('translate-x-[-100%]')
    expect(aside.attributes('style')).toContain('width: 16rem')
  })
})