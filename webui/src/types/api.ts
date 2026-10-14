export interface LoginResponse {
  token: string;
  user_id: string;
  email: string;
  is_admin: boolean;
  contexts: Array<{ id: string; slug: string; name: string }>;
}

// NOTE: backend domain structs have no json tags, so list/get responses
// use Go's PascalCase field names. Request payloads use the snake_case
// tags defined on the handler input structs (see *Input types below).
export interface Contact {
  ID: string;
  ContextID: string;
  Email: string | null;
  FirstName: string;
  LastName: string;
  Phone: string;
  Company: string;
  CustomFields: Record<string, string>;
  Source: string;
  SourceID: string;
  Tags: Tag[];
  CreatedAt: string;
  UpdatedAt: string;
}

export interface ContactInput {
  email?: string;
  first_name?: string;
  last_name?: string;
  phone?: string;
  company?: string;
  custom_fields?: Record<string, string>;
  source?: string;
  tags?: string[];
}

export interface Tag {
  ID: string;
  ContextID: string;
  Name: string;
  Color: string;
  CreatedAt: string;
}

export interface FormField {
  slug: string;
  type: string;
  label: string;
  required: boolean;
  validation: Record<string, unknown> | null;
  order: number;
}

export interface Form {
  ID: string;
  ContextID: string;
  Slug: string;
  Name: string;
  Description: string;
  Fields: FormField[];
  Settings: Record<string, unknown> | null;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface FormInput {
  slug?: string;
  name?: string;
  description?: string;
  fields?: FormField[];
  settings?: Record<string, unknown>;
}

export interface Banner {
  ID: string;
  ContextID: string;
  Name: string;
  Type: string;
  Content: string;
  LinkURL: string;
  ImageURL: string | null;
  AltText: string | null;
  CampaignID: string | null;
  Placements: string[];
  Priority: number;
  StartDate: string | null;
  EndDate: string | null;
  IsActive: boolean;
  ABTest: boolean;
  ABVariants: unknown[];
  Impressions: number;
  Clicks: number;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface BannerInput {
  name?: string;
  type?: string;
  content?: string;
  link_url?: string;
  image_url?: string;
  alt_text?: string;
  campaign_id?: string;
  placements?: string[];
  priority?: number;
  start_date?: string;
  end_date?: string;
  is_active?: boolean;
  ab_test?: boolean;
  ab_variants?: unknown[];
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
