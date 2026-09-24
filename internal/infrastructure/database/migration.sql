-- Core platform migration: contexts, context_users, api_keys

CREATE TABLE IF NOT EXISTS contexts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    domain          VARCHAR(256),
    is_active       BOOLEAN DEFAULT true,
    settings        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS context_users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    email       VARCHAR(254) NOT NULL,
    password_hash TEXT NOT NULL,
    is_owner    BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_context_users_context_email ON context_users(context_id, email);

CREATE TABLE IF NOT EXISTS api_keys (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES context_users(id) ON DELETE CASCADE,
    key_hash    VARCHAR(64) NOT NULL,
    name        TEXT NOT NULL,
    expires_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_context_name ON api_keys(context_id, name);

-- Admin config: per-context key-value settings
CREATE TABLE IF NOT EXISTS admin_configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    key         VARCHAR(128) NOT NULL,
    value       JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(context_id, key)
);

CREATE INDEX IF NOT EXISTS idx_admin_configs_context ON admin_configs(context_id);
