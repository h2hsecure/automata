-- Migration: 202609250000 — Add profile and session tracking columns

ALTER TABLE users
  ADD COLUMN display_name VARCHAR(64),
  ADD COLUMN avatar_url TEXT,
  ADD COLUMN password_changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_users_password_changed_at ON users(password_changed_at);
