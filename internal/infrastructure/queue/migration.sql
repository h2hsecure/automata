CREATE TABLE IF NOT EXISTS email_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL,
    to_addresses    TEXT[] NOT NULL,
    subject         TEXT NOT NULL,
    body            TEXT NOT NULL,
    html_body       TEXT,
    attempts        INT DEFAULT 0,
    max_retries     INT DEFAULT 3,
    next_retry      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_jobs_status ON email_jobs(next_retry);
