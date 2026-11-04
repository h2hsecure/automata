CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL,
    form_id     UUID,
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

-- Existing deployments created webhook_deliveries with form_id NOT NULL
-- (before webhooks_test, which enqueues deliveries without a form). Make the
-- column nullable in place; idempotent, applied under the migration lock.
ALTER TABLE webhook_deliveries ALTER COLUMN form_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_webhooks_status ON webhook_deliveries(status) WHERE status IN ('pending', 'retrying');
CREATE INDEX IF NOT EXISTS idx_webhooks_next_retry ON webhook_deliveries(next_retry) WHERE status = 'pending';
