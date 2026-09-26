-- Migration: Add display_name, avatar_url, password_changed_at to users table

ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ DEFAULT NOW();
