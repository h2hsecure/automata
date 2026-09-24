-- Migration: rename tenant references to context
-- Renames tables, columns, constraints, and indexes from tenant_* to context_*
-- Note: This migration assumes base migration.sql creates tenants table
-- If contexts table already exists (from updated base migration), skip rename steps

BEGIN;

-- 1. Rename tenants -> contexts (only if tenants exists and contexts doesn't)
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'tenants') AND
       NOT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'contexts') THEN
        ALTER TABLE tenants RENAME TO contexts;
    END IF;
END $$;

-- 2. Rename tenant_users -> context_users (only if tenant_users exists and context_users doesn't)
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'tenant_users') AND
       NOT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'context_users') THEN
        ALTER TABLE tenant_users RENAME TO context_users;
    END IF;
END $$;

-- 3. Rename api_keys.tenant_id -> context_id (only if column exists)
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'api_keys' AND column_name = 'tenant_id') THEN
        ALTER TABLE api_keys RENAME COLUMN tenant_id TO context_id;
    END IF;
END $$;

-- 4. Rename admin_configs.tenant_id -> context_id (only if column exists)
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'admin_configs' AND column_name = 'tenant_id') THEN
        ALTER TABLE admin_configs RENAME COLUMN tenant_id TO context_id;
    END IF;
END $$;

-- 5. Rename context_users.tenant_id -> context_id (only if column exists)
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'context_users' AND column_name = 'tenant_id') THEN
        ALTER TABLE context_users RENAME COLUMN tenant_id TO context_id;
    END IF;
END $$;

-- 6. Drop old indexes (ignore if they don't exist)
DROP INDEX IF EXISTS idx_tenant_users_tenant_email;
DROP INDEX IF EXISTS idx_api_keys_tenant_name;
DROP INDEX IF EXISTS idx_admin_configs_tenant;
DROP INDEX IF EXISTS idx_contacts_tenant_email;
DROP INDEX IF EXISTS idx_contacts_tenant_created;
DROP INDEX IF EXISTS idx_contacts_tenant_source;
DROP INDEX IF EXISTS idx_tags_tenant_name;
DROP INDEX IF EXISTS idx_field_defs_tenant_key;
DROP INDEX IF EXISTS idx_forms_tenant_slug;
DROP INDEX IF EXISTS idx_forms_tenant_created;
DROP INDEX IF EXISTS idx_submissions_form;
DROP INDEX IF EXISTS idx_submissions_tenant_created;
DROP INDEX IF EXISTS idx_banners_tenant_active;
DROP INDEX IF EXISTS idx_banners_campaign;
DROP INDEX IF EXISTS idx_banners_dates;
DROP INDEX IF EXISTS idx_impressions_banner;
DROP INDEX IF EXISTS idx_clicks_banner;
DROP INDEX IF EXISTS idx_tracking_visitors_tenant;
DROP INDEX IF EXISTS idx_tracking_events_tenant_created;
DROP INDEX IF EXISTS idx_tracking_events_visitor;
DROP INDEX IF EXISTS idx_tracking_events_type;
DROP INDEX IF EXISTS idx_tracking_events_url;
DROP INDEX IF EXISTS idx_email_jobs_status;
DROP INDEX IF EXISTS idx_webhooks_status;
DROP INDEX IF EXISTS idx_webhooks_next_retry;

-- 7. Create new indexes with context_ prefix (only if tables exist)
CREATE UNIQUE INDEX IF NOT EXISTS idx_context_users_context_email ON context_users(context_id, email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_context_name ON api_keys(context_id, name);
CREATE INDEX IF NOT EXISTS idx_admin_configs_context ON admin_configs(context_id);

-- Conditional index creation for tables that may not exist in base migration
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'contacts') THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_context_email ON contacts(context_id, email) WHERE email IS NOT NULL;
        CREATE INDEX IF NOT EXISTS idx_contacts_context_created ON contacts(context_id, created_at DESC);
        CREATE INDEX IF NOT EXISTS idx_contacts_context_source ON contacts(context_id, source);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'contact_tags') THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_context_name ON contact_tags(context_id, name);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'contact_field_definitions') THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_field_defs_context_key ON contact_field_definitions(context_id, key);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'forms') THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_forms_context_slug ON forms(context_id, slug);
        CREATE INDEX IF NOT EXISTS idx_forms_context_created ON forms(context_id, created_at DESC);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'form_submissions') THEN
        CREATE INDEX IF NOT EXISTS idx_submissions_form ON form_submissions(form_id);
        CREATE INDEX IF NOT EXISTS idx_submissions_context_created ON form_submissions(context_id, created_at DESC);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'banner_banners') THEN
        CREATE INDEX IF NOT EXISTS idx_banners_context_active ON banner_banners(context_id, is_active, priority DESC);
        CREATE INDEX IF NOT EXISTS idx_banners_campaign ON banner_banners(context_id, campaign_id);
        CREATE INDEX IF NOT EXISTS idx_banners_dates ON banner_banners(context_id, start_date, end_date);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'banner_impressions') THEN
        CREATE INDEX IF NOT EXISTS idx_impressions_banner ON banner_impressions(banner_id, created_at);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'banner_clicks') THEN
        CREATE INDEX IF NOT EXISTS idx_clicks_banner ON banner_clicks(banner_id, created_at);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'tracking_visitors') THEN
        CREATE INDEX IF NOT EXISTS idx_tracking_visitors_context ON tracking_visitors(context_id);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'tracking_events') THEN
        CREATE INDEX IF NOT EXISTS idx_tracking_events_context_created ON tracking_events(context_id, created_at DESC);
        CREATE INDEX IF NOT EXISTS idx_tracking_events_visitor ON tracking_events(context_id, visitor_id);
        CREATE INDEX IF NOT EXISTS idx_tracking_events_type ON tracking_events(context_id, type);
        CREATE INDEX IF NOT EXISTS idx_tracking_events_url ON tracking_events(context_id, url) WHERE type = 'pageview';
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'email_jobs') THEN
        CREATE INDEX IF NOT EXISTS idx_email_jobs_status ON email_jobs(next_retry);
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'webhook_deliveries') THEN
        CREATE INDEX IF NOT EXISTS idx_webhooks_status ON webhook_deliveries(status) WHERE status IN ('pending', 'retrying');
        CREATE INDEX IF NOT EXISTS idx_webhooks_next_retry ON webhook_deliveries(next_retry) WHERE status = 'pending';
    END IF;
END $$;

COMMIT;
