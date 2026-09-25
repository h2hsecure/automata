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
  const response = await client.get('/context/contacts', { params })
  return response.data
}

export async function getContact(id: string): Promise<Contact> {
  const response = await client.get(`/context/contacts/${id}`)
  return response.data
}

export async function createContact(data: Partial<Contact>): Promise<Contact> {
  const response = await client.post('/context/contacts', data)
  return response.data
}

export async function updateContact(id: string, data: Partial<Contact>): Promise<Contact> {
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
  return [] // TODO: implement forms endpoint
}

export async function getForm(id: string): Promise<Form> {
  return {} as Form
}

export async function createForm(data: Partial<Form>): Promise<Form> {
  return {} as Form
}

export async function updateForm(id: string, data: Partial<Form>): Promise<Form> {
  return {} as Form
}

export async function deleteForm(id: string): Promise<void> {}

export async function listSubmissions(formId: string, page = 1, limit = 20) {
  return { data: [], total: 0, page, limit }
}

export async function submitForm(formId: string, data: Record<string, string>): Promise<{ id: string }> {
  return { id: '' }
}

// Banner endpoints
export async function listBanners(contextId = ''): Promise<Banner[]> {
  return [] // TODO: implement banners endpoint
}

export async function createBanner(data: Partial<Banner>): Promise<Banner> {
  return {} as Banner
}

export async function updateBanner(id: string, data: Partial<Banner>): Promise<Banner> {
  return {} as Banner
}

export async function deleteBanner(id: string): Promise<void> {}

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
  return { total_visitors: 0, active_visitors: 0, total_page_views: 0, total_contacts: 0, total_forms: 0, total_banners: 0 }
}

export async function getTrackingEvents(page = 1, limit = 50, type = '') {
  return { data: [], total: 0, page, limit }
}

// Tenant endpoints (admin only)
export async function listTenants(page = 1, limit = 20): Promise<PaginatedResponse<Tenant>> {
  return { data: [], total: 0, page, limit }
}

export async function createTenant(data: Partial<Tenant>): Promise<Tenant> {
  return {} as Tenant
}

export async function updateTenant(id: string, data: Partial<Tenant>): Promise<Tenant> {
  return {} as Tenant
}

export async function deleteTenant(id: string): Promise<void> {}

// API Key endpoints (admin only)
export async function listApiKeys(): Promise<ApiKey[]> {
  return [] // TODO: implement API keys endpoint
}

export async function createApiKey(data: { name: string; expires_at?: string }): Promise<ApiKey> {
  return {} as ApiKey
}

export async function revokeApiKey(id: string): Promise<void> {}

export default client
