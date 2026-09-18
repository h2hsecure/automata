CREATE TABLE IF NOT EXISTS visitors (
    id              VARCHAR(128) PRIMARY KEY,
    tenant_id       UUID NOT NULL,
    cookie_value    VARCHAR(128) NOT NULL,
    fingerprint     VARCHAR(64),
    first_seen      TIMESTAMPTZ DEFAULT NOW(),
    last_seen       TIMESTAMPTZ DEFAULT NOW(),
    page_views      INT DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_visitors_tenant ON visitors(tenant_id);

CREATE TABLE IF NOT EXISTS events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    visitor_id      VARCHAR(128),
    type            VARCHAR(32) NOT NULL,
    url             VARCHAR(2048),
    title           VARCHAR(512),
    referrer        VARCHAR(2048),
    event_name      VARCHAR(128),
    properties      JSONB DEFAULT '{}',
    user_agent      VARCHAR(512),
    ip_hash         VARCHAR(64),
    utm_source      VARCHAR(128),
    utm_medium      VARCHAR(128),
    utm_campaign    VARCHAR(128),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_events_tenant_created ON events(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_events_visitor ON events(tenant_id, visitor_id);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(tenant_id, type);
CREATE INDEX IF NOT EXISTS idx_events_url ON events(tenant_id, url) WHERE type = 'pageview';
