CREATE TABLE IF NOT EXISTS page_pages (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL,
    slug        VARCHAR(128) NOT NULL,
    title       VARCHAR(255) NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pages_context_slug ON page_pages(context_id, slug);
CREATE INDEX IF NOT EXISTS idx_pages_context_created ON page_pages(context_id, created_at DESC);

ALTER TABLE page_pages ADD COLUMN IF NOT EXISTS position VARCHAR(16) DEFAULT '';
ALTER TABLE page_pages ADD COLUMN IF NOT EXISTS buttons JSONB DEFAULT '[]';

CREATE TABLE IF NOT EXISTS page_decisions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL,
    page_id     UUID NOT NULL REFERENCES page_pages(id) ON DELETE CASCADE,
    visitor_id  VARCHAR(128),
    action      VARCHAR(128),
    ip_hash     VARCHAR(64),
    user_agent  VARCHAR(512),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS page_impressions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL,
    page_id     UUID NOT NULL REFERENCES page_pages(id) ON DELETE CASCADE,
    visitor_id  VARCHAR(128),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_page_decisions ON page_decisions(page_id, created_at);
CREATE INDEX IF NOT EXISTS idx_page_impressions ON page_impressions(page_id, created_at);

-- Retire banner subsystem (banner_banners references banner_campaigns and is
-- referenced by the three event tables, so drop in this order).
DROP TABLE IF EXISTS banner_impressions;
DROP TABLE IF EXISTS banner_clicks;
DROP TABLE IF EXISTS banner_decisions;
DROP TABLE IF EXISTS banner_banners;
DROP TABLE IF EXISTS banner_campaigns;
DROP TABLE IF EXISTS banner_placements;