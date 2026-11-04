-- Encrypts webhook signing secrets at rest.
-- The backfill (encrypt + clear) and column drop are performed in Go in
-- migration.go, under the shared migration advisory lock.
-- The column is TEXT holding base64(nonce || ciphertext) (AES-GCM).
ALTER TABLE webhook_endpoints ADD COLUMN IF NOT EXISTS signing_secret_enc TEXT;