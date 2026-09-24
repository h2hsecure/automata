CREATE TABLE IF NOT EXISTS banner_banners (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id     UUID NOT NULL,
    name          VARCHAR(255) NOT NULL,
    type          VARCHAR(32) NOT NULL,
    content       TEXT,
    link_url      VARCHAR(2048),
    image_url     VARCHAR(2048),
    alt_text      VARCHAR(512),
    campaign_id   UUID,
    placements    JSONB DEFAULT '[]',
    priority      INTEGER DEFAULT 0,
    start_date    TIMESTAMPTZ,
    end_date      TIMESTAMPTZ,
    is_active     BOOLEAN DEFAULT TRUE,
    ab_test       BOOLEAN DEFAULT FALSE,
    ab_variants   JSONB DEFAULT '[]',
    impressions   BIGINT DEFAULT 0,
    clicks        BIGINT DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS banner_placements (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id     UUID NOT NULL,
    name          VARCHAR(255) NOT NULL,
    location      VARCHAR(128),
    css_selector  VARCHAR(512),
    max_banners   INTEGER DEFAULT 1,
    priority      INTEGER DEFAULT 0,
    is_active     BOOLEAN DEFAULT TRUE,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS banner_campaigns (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id       UUID NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    start_date      TIMESTAMPTZ,
    end_date        TIMESTAMPTZ,
    is_active       BOOLEAN DEFAULT TRUE,
    target_url      VARCHAR(2048),
    tracking_code   VARCHAR(512),
    impressions     BIGINT DEFAULT 0,
    clicks          BIGINT DEFAULT 0,
    conversion_rate FLOAT8 DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS banner_impressions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id     UUID NOT NULL REFERENCES banner_banners(id) ON DELETE CASCADE,
    context_id     UUID NOT NULL,
    visitor_id    VARCHAR(128),
    placement_id  UUID,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS banner_clicks (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    banner_id     UUID NOT NULL REFERENCES banner_banners(id) ON DELETE CASCADE,
    context_id     UUID NOT NULL,
    visitor_id    VARCHAR(128),
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_banners_context_active ON banner_banners(context_id, is_active, priority DESC);
CREATE INDEX IF NOT EXISTS idx_banners_campaign ON banner_banners(context_id, campaign_id);
CREATE INDEX IF NOT EXISTS idx_banners_dates ON banner_banners(context_id, start_date, end_date);
CREATE INDEX IF NOT EXISTS idx_impressions_banner ON banner_impressions(banner_id, created_at);
CREATE INDEX IF NOT EXISTS idx_clicks_banner ON banner_clicks(banner_id, created_at);
