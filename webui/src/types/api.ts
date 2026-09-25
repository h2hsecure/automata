export interface LoginResponse {
  token: string;
  user_id: string;
  email: string;
  is_admin: boolean;
  contexts: Array<{ id: string; slug: string; name: string }>;
}

export interface Contact {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  phone: string | null;
  tags: Tag[];
  custom_fields: Record<string, string>;
  context_id: string;
  created_at: string;
  updated_at: string;
}

export interface Tag {
  id: string;
  name: string;
  color: string;
}

export interface Form {
  id: string;
  name: string;
  slug: string;
  context_id: string;
  fields: Array<{ name: string; type: string; required: boolean; label: string }>;
  created_at: string;
}

export interface Banner {
  id: string;
  name: string;
  placement: string;
  status: string;
  content_type: string;
  content: string;
  link_url: string;
  created_at: string;
  updated_at: string;
}

export interface Placement {
  id: string;
  name: string;
  selector: string;
  status: string;
  created_at: string;
}

export interface Campaign {
  id: string;
  name: string;
  description: string;
  start_date: string;
  end_date: string;
  status: string;
  created_at: string;
}

export interface Tenant {
  id: string;
  slug: string;
  name: string;
  domain: string;
  active: boolean;
  created_at: string;
}

export interface ApiKey {
  id: string;
  name: string;
  key_prefix: string;
  expires_at: string | null;
  created_at: string;
  active: boolean;
}

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
}

export interface DashboardStats {
  total_visitors: number;
  active_visitors: number;
  total_page_views: number;
  total_contacts: number;
  total_forms: number;
  total_banners: number;
}

export interface TrackingEvent {
  id: string;
  type: string;
  url: string;
  event_name: string;
  referrer: string;
  visitor_id: string;
  created_at: string;
}

export interface User {
  id: string;
  email: string;
  is_admin: boolean;
  tenant_id: string;
}
