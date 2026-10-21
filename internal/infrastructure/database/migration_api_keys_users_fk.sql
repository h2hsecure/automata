-- Migration: repoint api_keys.user_id from context_users(id) to users(id).
--
-- The users table (see migration_user_contexts.sql) is the current user
-- model; the main API creates API keys with the global user ID taken from
-- the authenticated JWT. The original FK to the legacy context_users table
-- made key creation impossible, so it is replaced here.
--
-- Idempotent: safe to run on any ordering relative to the users migration.

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(254) NOT NULL UNIQUE,
    password_hash   TEXT,
    sso_provider    VARCHAR(64),
    sso_id          VARCHAR(512),
    is_admin        BOOLEAN DEFAULT false,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'api_keys_user_id_fkey'
          AND pg_get_constraintdef(oid) LIKE '%context_users(id)%'
    ) THEN
        ALTER TABLE api_keys DROP CONSTRAINT api_keys_user_id_fkey;
        ALTER TABLE api_keys ADD CONSTRAINT api_keys_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
END $$;