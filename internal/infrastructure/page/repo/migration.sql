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