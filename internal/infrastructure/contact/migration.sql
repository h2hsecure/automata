-- Contacts table
CREATE TABLE IF NOT EXISTS contacts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id       UUID NOT NULL,
    email           VARCHAR(254),
    first_name      VARCHAR(128),
    last_name       VARCHAR(128),
    phone           VARCHAR(32),
    company         VARCHAR(256),
    custom_fields   JSONB DEFAULT '{}',
    source          VARCHAR(64),
    source_id       VARCHAR(128),
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_context_email
    ON contacts(context_id, email) WHERE email IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_contacts_context_created
    ON contacts(context_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_contacts_context_source
    ON contacts(context_id, source);

-- Tags table
CREATE TABLE IF NOT EXISTS contact_tags (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL,
    name        TEXT NOT NULL,
    color       VARCHAR(7) DEFAULT '#6366f1',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tags_context_name
    ON contact_tags(context_id, name);

-- Contact-tag junction
CREATE TABLE IF NOT EXISTS contact_tag_memberships (
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    tag_id      UUID NOT NULL REFERENCES contact_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (contact_id, tag_id)
);

-- Custom fields definition (per context)
CREATE TABLE IF NOT EXISTS contact_field_definitions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL,
    key         TEXT NOT NULL,
    label       TEXT NOT NULL,
    type        VARCHAR(32) NOT NULL,
    options     JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_field_defs_context_key
    ON contact_field_definitions(context_id, key);
