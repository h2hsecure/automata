import axios, { type AxiosInstance, type InternalAxiosRequestConfig } from 'axios'
import type {
  Contact,
  ContactInput,
  Form,
  FormInput,
  Banner,
  BannerInput,
  ApiKey,
  Context,
  ContextInput,
  PaginatedResponse,
  DashboardStats,
  TrackingEvent,
  Visitor,
  User,
  Campaign,
  Placement,
  ContextUser,
} from '@/types/api'

const client: AxiosInstance = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
})

// Token refresh interceptor
client.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem('token')
    if (token) {
      config.headers.set('Authorization', `Bearer ${token}`)
    }
    return config
  },
  (error) => Promise.reject(error)
)

// Context interceptor: send the selected context ID with every request.
client.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const contextId = localStorage.getItem('context_id')
    if (contextId) {
      config.headers.set('X-Context-ID', contextId)
    }
    return config
  },
  (error) => Promise.reject(error)
)

client.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('user')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// Auth endpoints
export async function login(email: string, password: string) {
  const response = await client.post('/auth/login', { email, password })
  return response.data
}

export async function logout() {
  await client.post('/auth/logout')
}

export async function getMe(): Promise<User> {
  const response = await client.get('/auth/me')
  return response.data
}

// Contact endpoints
export async function listContacts(
  page = 1,
  limit = 20,
  search = '',
  contextId = ''
): Promise<PaginatedResponse<Contact>> {
  const params: Record<string, string> = { page: String(page), limit: String(limit) }
  if (search) params.search = search
  if (contextId) params.context_id = contextId
  const response = await client.get('/context/contacts', { params })
  const d = response.data
  return {
    data: d.contacts ?? [],
    total: d.total ?? 0,
    page: d.page ?? page,
    limit: d.limit ?? limit,
  }
}

export async function getContact(id: string): Promise<Contact> {
  const response = await client.get(`/context/contacts/${id}`)
  return response.data
}

export async function createContact(data: ContactInput): Promise<Contact> {
  const response = await client.post('/context/contacts', data)
  return response.data
}

export async function updateContact(id: string, data: ContactInput): Promise<Contact> {
  const response = await client.put(`/context/contacts/${id}`, data)
  return response.data
}

export async function deleteContact(id: string): Promise<void> {
  await client.delete(`/context/contacts/${id}`)
}

export async function mergeContacts(id: string, mergeWith: string): Promise<{ merged_id: string }> {
  const response = await client.post(`/context/contacts/${id}/merge`, { merge_with: mergeWith })
  return response.data
}

export async function getContactActivity(id: string, page = 1, limit = 20) {
  const response = await client.get(`/context/contacts/${id}/activity`, { params: { page, limit } })
  return response.data
}

// Form endpoints
export async function listForms(contextId = ''): Promise<Form[]> {
  const params: Record<string, string> = {}
  if (contextId) params.context_id = contextId
  const response = await client.get('/context/forms', { params })
  return response.data.forms ?? []
}

export async function getForm(id: string): Promise<Form> {
  const response = await client.get(`/context/forms/${id}`)
  return response.data
}

export async function createForm(data: FormInput): Promise<Form> {
  const response = await client.post('/context/forms', data)
  return response.data
}

export async function updateForm(id: string, data: FormInput): Promise<Form> {
  const response = await client.put(`/context/forms/${id}`, data)
  return response.data
}

export async function deleteForm(id: string): Promise<void> {
  await client.delete(`/context/forms/${id}`)
}

export async function listSubmissions(formId: string, page = 1, limit = 20) {
  return { data: [], total: 0, page, limit }
}

export async function submitForm(formId: string, data: Record<string, string>): Promise<{ id: string }> {
  return { id: '' }
}

// Banner endpoints
export async function listBanners(contextId = ''): Promise<Banner[]> {
  const params: Record<string, string> = {}
  if (contextId) params.context_id = contextId
  const response = await client.get('/context/banners', { params })
  return response.data.banners ?? []
}

export async function createBanner(data: BannerInput): Promise<Banner> {
  const response = await client.post('/context/banners', data)
  return response.data
}

export async function updateBanner(id: string, data: BannerInput): Promise<Banner> {
  const response = await client.put(`/context/banners/${id}`, data)
  return response.data
}

export async function deleteBanner(id: string): Promise<void> {
  await client.delete(`/context/banners/${id}`)
}

// Placement endpoints
export async function listPlacements(contextId = ''): Promise<Placement[]> {
  return [] // TODO: implement placements endpoint
}

// Campaign endpoints
export async function listCampaigns(contextId = ''): Promise<Campaign[]> {
  return [] // TODO: implement campaigns endpoint
}

// Tracking endpoints
export async function getDashboardStats(): Promise<DashboardStats> {
  const response = await client.get('/context/tracking/dashboard')
  return response.data
}

export async function getTrackingEvents(page = 1, limit = 50, type = ''): Promise<PaginatedResponse<TrackingEvent>> {
  const params: Record<string, string> = {
    offset: String((page - 1) * limit),
    limit: String(limit),
  }
  if (type) params.type = type
  const response = await client.get('/context/tracking/events', { params })
  const d = response.data
  return {
    data: d.events ?? [],
    total: d.total ?? 0,
    page,
    limit,
  }
}

export async function listVisitors(page = 1, limit = 20, sort = 'last_seen'): Promise<PaginatedResponse<Visitor>> {
  const params: Record<string, string> = { offset: String((page - 1) * limit), limit: String(limit), sort }
  const response = await client.get('/context/tracking/visitors', { params })
  const d = response.data
  return { data: d.visitors ?? [], total: d.total ?? 0, page, limit }
}

export async function getVisitorEvents(visitorId: string, page = 1, limit = 20): Promise<PaginatedResponse<TrackingEvent>> {
  const params: Record<string, string> = { offset: String((page - 1) * limit), limit: String(limit) }
  const response = await client.get(`/context/tracking/visitors/${visitorId}/events`, { params })
  const d = response.data
  return { data: d.events ?? [], total: d.total ?? 0, page, limit }
}

// Context endpoints (admin only)
export async function listContexts(offset = 0, limit = 100): Promise<Context[]> {
  const response = await client.get('/admin/contexts', {
    params: { offset: String(offset), limit: String(limit) },
  })
  return response.data.contexts ?? []
}

export async function getContext(id: string): Promise<Context> {
  const response = await client.get(`/admin/contexts/${id}`)
  return response.data
}

export async function createContext(data: ContextInput): Promise<Context> {
  const response = await client.post('/admin/contexts', data)
  return response.data
}

export async function updateContext(id: string, data: ContextInput): Promise<Context> {
  const response = await client.put(`/admin/contexts/${id}`, data)
  return response.data
}

export async function deleteContext(id: string): Promise<void> {
  await client.delete(`/admin/contexts/${id}`)
}

// API Key endpoints (admin only)
export async function listApiKeys(): Promise<ApiKey[]> {
  const response = await client.get('/admin/api-keys')
  return response.data.keys ?? []
}

export async function createApiKey(data: { name: string; expires_in?: number }): Promise<{ key: string }> {
  const response = await client.post('/admin/api-keys', data)
  return response.data
}

export async function revokeApiKey(id: string): Promise<void> {
  await client.delete(`/admin/api-keys/${id}`)
}

// User endpoints (admin only)
export async function listUsers(): Promise<User[]> {
  const response = await client.get('/admin/users')
  return response.data.users ?? []
}

export async function createUser(data: { email: string; password: string; is_admin?: boolean }): Promise<User> {
  const response = await client.post('/admin/users', data)
  return response.data
}

export async function updateUser(id: string, data: { email?: string; password?: string; is_admin?: boolean }): Promise<User> {
  const response = await client.put(`/admin/users/${id}`, data)
  return response.data
}

export async function deleteUser(id: string): Promise<void> {
  await client.delete(`/admin/users/${id}`)
}

// Context membership endpoints (admin only)
export async function listContextUsers(contextId: string): Promise<ContextUser[]> {
  const response = await client.get(`/admin/contexts/${contextId}/users`)
  return response.data.members ?? []
}

export async function assignContextUser(contextId: string, data: { user_id: string; role: string }): Promise<void> {
  await client.post(`/admin/contexts/${contextId}/users`, data)
}

export async function removeContextUser(contextId: string, userId: string): Promise<void> {
  await client.delete(`/admin/contexts/${contextId}/users/${userId}`)
}

// Global settings endpoints (admin only)
export async function listSettings(): Promise<Record<string, string>> {
  const { data } = await client.get('/admin/settings')
  return data.settings
}

export async function upsertSetting(key: string, value: string): Promise<void> {
  await client.put('/admin/settings', { key, value })
}

export async function deleteSetting(key: string): Promise<void> {
  await client.delete(`/admin/settings/${key}`)
}

export default client
