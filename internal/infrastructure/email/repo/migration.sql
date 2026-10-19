-- Email templates table
CREATE TABLE IF NOT EXISTS email_templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id  UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    key         VARCHAR(64) NOT NULL,
    subject     TEXT NOT NULL DEFAULT '',
    body_text   TEXT NOT NULL DEFAULT '',
    body_html   TEXT NOT NULL DEFAULT '',
    is_default  BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT uq_email_templates_context_key UNIQUE (context_id, key)
);

CREATE INDEX IF NOT EXISTS idx_email_templates_context ON email_templates(context_id);

-- Template kind (standalone | form) and optional form binding
ALTER TABLE email_templates ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'standalone';
ALTER TABLE email_templates ADD COLUMN IF NOT EXISTS form_id UUID;
