import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { AxiosError } from 'axios'
import client, { getMe, listContexts } from './client'

// The 401 backstop in the response interceptor navigates to /login via
// window.location.href. jsdom does not implement navigation, so we stub
// the location object and assert on the href assignment instead.
let location: { href: string }
const realLocation = window.location

beforeEach(() => {
  location = { href: '/' }
  Object.defineProperty(window, 'location', {
    value: location,
    writable: false,
    configurable: true,
  })
})

afterEach(() => {
  Object.defineProperty(window, 'location', {
    value: realLocation,
    writable: false,
    configurable: true,
  })
  vi.restoreAllMocks()
})

// rejectWith401 makes the axios adapter reject like a real 401 response,
// preserving the request config (which is how the interceptor sees
// per-request flags).
function rejectWith401() {
  client.defaults.adapter = async (config) => {
    const response = {
      status: 401,
      statusText: 'Unauthorized',
      headers: {},
      data: { error: 'unauthorized' },
      config,
    } as any
    throw new AxiosError(
      'Request failed with status code 401',
      AxiosError.ERR_BAD_REQUEST,
      config,
      null,
      response
    )
  }
}

describe('client 401 interceptor', () => {
  it('does NOT full-reload for session-verification calls (guard handles the redirect)', async () => {
    rejectWith401()
    await expect(getMe()).rejects.toMatchObject({ response: { status: 401 } })
    expect(location.href).toBe('/')
  })

  it('full-reloads to /login for a mid-session 401 on a data endpoint', async () => {
    rejectWith401()
    await expect(listContexts()).rejects.toMatchObject({ response: { status: 401 } })
    expect(location.href).toBe('/login')
  })
})