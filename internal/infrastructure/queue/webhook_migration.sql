CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL,
    form_id     UUID NOT NULL,
    url         TEXT NOT NULL,
    payload     JSONB NOT NULL,
    status      VARCHAR(16) DEFAULT 'pending',
    attempts    INT DEFAULT 0,
    max_retries INT DEFAULT 3,
    next_retry  TIMESTAMPTZ,
    error_msg   TEXT,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_status ON webhook_deliveries(status) WHERE status IN ('pending', 'retrying');
CREATE INDEX IF NOT EXISTS idx_webhooks_next_retry ON webhook_deliveries(next_retry) WHERE status = 'pending';
