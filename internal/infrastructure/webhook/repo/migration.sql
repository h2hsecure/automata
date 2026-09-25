-- Webhook endpoints table
CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id      UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    name            VARCHAR(128) NOT NULL,
    url             TEXT NOT NULL,
    events          TEXT NOT NULL DEFAULT '',
    secret_hash     TEXT,
    signing_secret  TEXT,
    active          BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_context ON webhook_endpoints(context_id);
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_active ON webhook_endpoints(context_id, active) WHERE active = true;
