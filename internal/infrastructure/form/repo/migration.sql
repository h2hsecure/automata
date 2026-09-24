CREATE TABLE IF NOT EXISTS forms (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id   UUID NOT NULL,
    slug        TEXT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    fields      JSONB NOT NULL DEFAULT '[]',
    settings    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_forms_context_slug ON forms(context_id, slug);
CREATE INDEX IF NOT EXISTS idx_forms_context_created ON forms(context_id, created_at DESC);

CREATE TABLE IF NOT EXISTS form_submissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    form_id     UUID NOT NULL REFERENCES forms(id),
    context_id   UUID NOT NULL,
    data        JSONB NOT NULL,
    files       JSONB DEFAULT '[]',
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_submissions_form ON form_submissions(form_id);
CREATE INDEX IF NOT EXISTS idx_submissions_context_created ON form_submissions(context_id, created_at DESC);
