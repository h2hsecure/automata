import axios, { type AxiosInstance, type InternalAxiosRequestConfig } from 'axios'
import type {
  Contact,
  Form,
  Banner,
  Tenant,
  ApiKey,
  PaginatedResponse,
  DashboardStats,
  TrackingEvent,
  User,
  Campaign,
  Placement,
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
export async function login(email: string, password: string, tenant?: string) {
  const response = await client.post('/auth/login', { email, password, tenant })
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
  const response = await client.get('/tenant/contacts', { params })
  return response.data
}

export async function getContact(id: string): Promise<Contact> {
  const response = await client.get(`/tenant/contacts/${id}`)
  return response.data
}

export async function createContact(data: Partial<Contact>): Promise<Contact> {
  const response = await client.post('/tenant/contacts', data)
  return response.data
}

export async function updateContact(id: string, data: Partial<Contact>): Promise<Contact> {
  const response = await client.put(`/tenant/contacts/${id}`, data)
  return response.data
}

export async function deleteContact(id: string): Promise<void> {
  await client.delete(`/tenant/contacts/${id}`)
}

export async function mergeContacts(id: string, mergeWith: string): Promise<{ merged_id: string }> {
  const response = await client.post(`/tenant/contacts/${id}/merge`, { merge_with: mergeWith })
  return response.data
}

export async function getContactActivity(id: string, page = 1, limit = 20) {
  const response = await client.get(`/tenant/contacts/${id}/activity`, { params: { page, limit } })
  return response.data
}

// Form endpoints
export async function listForms(contextId = ''): Promise<Form[]> {
  const params: Record<string, string> = {}
  if (contextId) params.context_id = contextId
  const response = await client.get('/tenant/forms', { params })
  return response.data
}

export async function getForm(id: string): Promise<Form> {
  const response = await client.get(`/tenant/forms/${id}`)
  return response.data
}

export async function createForm(data: Partial<Form>): Promise<Form> {
  const response = await client.post('/tenant/forms', data)
  return response.data
}

export async function updateForm(id: string, data: Partial<Form>): Promise<Form> {
  const response = await client.put(`/tenant/forms/${id}`, data)
  return response.data
}

export async function deleteForm(id: string): Promise<void> {
  await client.delete(`/tenant/forms/${id}`)
}

export async function listSubmissions(formId: string, page = 1, limit = 20) {
  const response = await client.get(`/tenant/forms/${formId}/submissions`, { params: { page, limit } })
  return response.data
}

export async function submitForm(formId: string, data: Record<string, string>): Promise<{ id: string }> {
  const response = await client.post(`/tenant/forms/${formId}/submit`, data)
  return response.data
}

// Banner endpoints
export async function listBanners(contextId = ''): Promise<Banner[]> {
  const params: Record<string, string> = {}
  if (contextId) params.context_id = contextId
  const response = await client.get('/tenant/banners', { params })
  return response.data
}

export async function createBanner(data: Partial<Banner>): Promise<Banner> {
  const response = await client.post('/tenant/banners', data)
  return response.data
}

export async function updateBanner(id: string, data: Partial<Banner>): Promise<Banner> {
  const response = await client.put(`/tenant/banners/${id}`, data)
  return response.data
}

export async function deleteBanner(id: string): Promise<void> {
  await client.delete(`/tenant/banners/${id}`)
}

// Placement endpoints
export async function listPlacements(contextId = ''): Promise<Placement[]> {
  const params: Record<string, string> = {}
  if (contextId) params.context_id = contextId
  const response = await client.get('/tenant/placements', { params })
  return response.data
}

// Campaign endpoints
export async function listCampaigns(contextId = ''): Promise<Campaign[]> {
  const params: Record<string, string> = {}
  if (contextId) params.context_id = contextId
  const response = await client.get('/tenant/campaigns', { params })
  return response.data
}

// Tracking endpoints
export async function getDashboardStats(): Promise<DashboardStats> {
  const response = await client.get('/tracking/dashboard')
  return response.data
}

export async function getTrackingEvents(page = 1, limit = 50, type = '') {
  const params: Record<string, string> = { page: String(page), limit: String(limit) }
  if (type) params.type = type
  const response = await client.get('/tracking/events', { params })
  return response.data
}

// Tenant endpoints (admin only)
export async function listTenants(page = 1, limit = 20): Promise<PaginatedResponse<Tenant>> {
  const response = await client.get('/admin/tenants', { params: { page, limit } })
  return response.data
}

export async function createTenant(data: Partial<Tenant>): Promise<Tenant> {
  const response = await client.post('/admin/tenants', data)
  return response.data
}

export async function updateTenant(id: string, data: Partial<Tenant>): Promise<Tenant> {
  const response = await client.put(`/admin/tenants/${id}`, data)
  return response.data
}

export async function deleteTenant(id: string): Promise<void> {
  await client.delete(`/admin/tenants/${id}`)
}

// API Key endpoints (admin only)
export async function listApiKeys(): Promise<ApiKey[]> {
  const response = await client.get('/admin/api-keys')
  return response.data
}

export async function createApiKey(data: { name: string; expires_at?: string }): Promise<ApiKey> {
  const response = await client.post('/admin/api-keys', data)
  return response.data
}

export async function revokeApiKey(id: string): Promise<void> {
  await client.delete(`/admin/api-keys/${id}`)
}

export default client
