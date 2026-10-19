CREATE TABLE IF NOT EXISTS email_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id       UUID NOT NULL,
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

CREATE TABLE IF NOT EXISTS email_history (
    id            UUID PRIMARY KEY,
    context_id    UUID NOT NULL,
    to_addresses  TEXT[] NOT NULL,
    subject       TEXT NOT NULL,
    body          TEXT NOT NULL,
    html_body     TEXT,
    attempts      INT NOT NULL DEFAULT 0,
    status        TEXT NOT NULL,
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL,
    finished_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_email_history_context ON email_history(context_id, status, finished_at DESC);
