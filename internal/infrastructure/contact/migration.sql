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
-- Hidden flag: contacts that represent anonymous page visitors only
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS hidden BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_contacts_context_source_id
    ON contacts(context_id, source, source_id);

-- Visitor client fields: raw user-agent, parsed device/browser, client IP,
-- and offline (GeoLite2) geolocation, enriched from page_visit events.
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS user_agent TEXT;
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS device_type VARCHAR(16);
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS browser VARCHAR(16);
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS ip_address INET;
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS geo_country VARCHAR(128);
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS geo_country_code CHAR(2);
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS geo_city VARCHAR(128);
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS geo_region VARCHAR(128);

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

-- Contact activities table
CREATE TABLE IF NOT EXISTS contact_activities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_id      UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    context_id      UUID NOT NULL,
    type            VARCHAR(64) NOT NULL,
    data            JSONB DEFAULT '{}',
    source_id       VARCHAR(128),
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_activities_contact ON contact_activities(contact_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activities_context ON contact_activities(context_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activities_data ON contact_activities USING GIN (data);

-- Contact cookie mappings for tracking resolution
CREATE TABLE IF NOT EXISTS contact_cookie_mappings (
    contact_id  UUID NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    context_id  UUID NOT NULL,
    cookie      TEXT NOT NULL,
    source      VARCHAR(32) NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (context_id, cookie)
);

CREATE INDEX IF NOT EXISTS idx_cookie_contact ON contact_cookie_mappings(contact_id);

-- Deduplicate visitor contacts (spec 5.1): historical races could create
-- several hidden contacts for the same (context_id, source_id). Keep the
-- oldest, reattach every dependent row to it, delete the rest, and add a
-- partial unique index so duplicates cannot reappear. Idempotent: once the
-- index exists there are no duplicates left, so every statement below is a
-- no-op on re-runs.
WITH dups AS (
    SELECT id AS dup_id,
           FIRST_VALUE(id) OVER (
               PARTITION BY context_id, source_id
               ORDER BY created_at, id) AS keep_id
    FROM contacts
    WHERE source = 'visitor' AND source_id IS NOT NULL
)
UPDATE contact_activities SET contact_id = d.keep_id
FROM dups d
WHERE contact_activities.contact_id = d.dup_id AND d.dup_id <> d.keep_id;

WITH dups AS (
    SELECT id AS dup_id,
           FIRST_VALUE(id) OVER (
               PARTITION BY context_id, source_id
               ORDER BY created_at, id) AS keep_id
    FROM contacts
    WHERE source = 'visitor' AND source_id IS NOT NULL
)
UPDATE contact_tag_memberships t
SET contact_id = d.keep_id
FROM dups d
WHERE t.contact_id = d.dup_id AND d.dup_id <> d.keep_id
  -- If the kept contact already carries the tag, skip the reattach: the
  -- UPDATE would violate PK (contact_id, tag_id) and fail the whole
  -- migration set. The duplicate's row is dropped with the contact
  -- (ON DELETE CASCADE). (t = the row being updated; the inner alias k
  -- must not shadow the outer table name.)
  AND NOT EXISTS (
      SELECT 1 FROM contact_tag_memberships k
      WHERE k.contact_id = d.keep_id AND k.tag_id = t.tag_id
  );

WITH dups AS (
    SELECT id AS dup_id,
           FIRST_VALUE(id) OVER (
               PARTITION BY context_id, source_id
               ORDER BY created_at, id) AS keep_id
    FROM contacts
    WHERE source = 'visitor' AND source_id IS NOT NULL
)
-- contact_cookie_mappings needs no collision guard: its PK is
-- (context_id, cookie), so the kept contact and a duplicate (always in
-- the same context) can never both hold the same (context, cookie) row.
UPDATE contact_cookie_mappings SET contact_id = d.keep_id
FROM dups d
WHERE contact_cookie_mappings.contact_id = d.dup_id AND d.dup_id <> d.keep_id;

-- Guarded: tracking_visitors is owned by the tracking package's migration,
-- which may not have run yet (migration order differs per binary).
DO $$
BEGIN
    IF to_regclass('tracking_visitors') IS NOT NULL THEN
        WITH dups AS (
            SELECT id AS dup_id,
                   FIRST_VALUE(id) OVER (
                       PARTITION BY context_id, source_id
                       ORDER BY created_at, id) AS keep_id
            FROM contacts
            WHERE source = 'visitor' AND source_id IS NOT NULL
        )
        UPDATE tracking_visitors SET contact_id = d.keep_id::text
        FROM dups d
        WHERE tracking_visitors.contact_id = d.dup_id::text AND d.dup_id <> d.keep_id;
    END IF;
END $$;

WITH keep AS (
    SELECT DISTINCT ON (context_id, source_id) id
    FROM contacts
    WHERE source = 'visitor' AND source_id IS NOT NULL
    ORDER BY context_id, source_id, created_at, id
)
DELETE FROM contacts c
WHERE c.source = 'visitor' AND c.source_id IS NOT NULL
  AND c.id NOT IN (SELECT id FROM keep);

CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_visitor_unique
    ON contacts(context_id, source_id)
    WHERE source = 'visitor' AND source_id IS NOT NULL;
