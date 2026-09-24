-- Migration: 202609240000 — Add platform users and user_contexts tables

-- 1. Create platform-level users table
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

-- 2. Create user-context memberships table
CREATE TABLE IF NOT EXISTS user_contexts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    context_id      UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    role            VARCHAR(32) NOT NULL DEFAULT 'member',
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, context_id)
);

-- 3. Migrate data from context_users → users + user_contexts
DO $$
BEGIN
    -- Copy distinct users (by email) to users table
    INSERT INTO users (email, password_hash, is_admin, created_at, updated_at)
    SELECT email, password_hash, is_owner, created_at, updated_at
    FROM context_users
    ON CONFLICT (email) DO NOTHING;

    -- Copy context memberships to user_contexts table
    INSERT INTO user_contexts (user_id, context_id, role, created_at, updated_at)
    SELECT cu.id, cu.context_id,
           CASE WHEN cu.is_owner THEN 'owner' ELSE 'admin' END,
           NOW(), NOW()
    FROM context_users cu
    INNER JOIN users u ON u.email = cu.email;
END $$;

-- 4. Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_user_contexts_user ON user_contexts(user_id);
CREATE INDEX IF NOT EXISTS idx_user_contexts_context ON user_contexts(context_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
