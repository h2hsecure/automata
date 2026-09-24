CREATE TABLE IF NOT EXISTS tracking_visitors (
    id              VARCHAR(128) PRIMARY KEY,
    context_id       UUID NOT NULL,
    cookie_value    VARCHAR(128) NOT NULL,
    fingerprint     VARCHAR(64),
    first_seen      TIMESTAMPTZ DEFAULT NOW(),
    last_seen       TIMESTAMPTZ DEFAULT NOW(),
    page_views      INT DEFAULT 0,
    UNIQUE(context_id, cookie_value)
);

CREATE INDEX IF NOT EXISTS idx_tracking_visitors_context ON tracking_visitors(context_id);

CREATE TABLE IF NOT EXISTS tracking_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id       UUID NOT NULL,
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

CREATE INDEX IF NOT EXISTS idx_tracking_events_context_created ON tracking_events(context_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tracking_events_visitor ON tracking_events(context_id, visitor_id);
CREATE INDEX IF NOT EXISTS idx_tracking_events_type ON tracking_events(context_id, type);
CREATE INDEX IF NOT EXISTS idx_tracking_events_url ON tracking_events(context_id, url) WHERE type = 'pageview';
