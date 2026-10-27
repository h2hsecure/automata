CREATE TABLE IF NOT EXISTS static_files (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL,
    name        TEXT NOT NULL,
    token       TEXT NOT NULL,
    mime        TEXT,
    size        BIGINT DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_static_files_context_name ON static_files(context_id, name);
CREATE INDEX IF NOT EXISTS idx_static_files_context ON static_files(context_id);