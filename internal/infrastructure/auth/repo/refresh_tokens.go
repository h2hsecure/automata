package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RefreshTokenPostgresStore persists refresh token jtis for server-side
// rotation with reuse detection (spec 4.6).
type RefreshTokenPostgresStore struct {
	pool *pgxpool.Pool
}

// NewRefreshTokenPostgresStore creates a refresh token store backed by Postgres.
func NewRefreshTokenPostgresStore(pool *pgxpool.Pool) *RefreshTokenPostgresStore {
	return &RefreshTokenPostgresStore{pool: pool}
}

// Insert registers a newly minted refresh token jti.
func (s *RefreshTokenPostgresStore) Insert(ctx context.Context, jti, userID string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (jti, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, jti, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

// RevokeIfActive revokes an active jti and reports the outcome:
//   - "ok"              — was active, now revoked (safe rotation)
//   - "already_revoked" — reuse attempt (theft signal)
//   - "missing"         — unknown jti
func (s *RefreshTokenPostgresStore) RevokeIfActive(ctx context.Context, jti string) (string, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW()
		WHERE jti = $1 AND revoked_at IS NULL
	`, jti)
	if err != nil {
		return "", fmt.Errorf("revoke refresh token: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return "ok", nil
	}

	// Distinguish a revoked jti from a missing one.
	var revoked bool
	err = s.pool.QueryRow(ctx, `
		SELECT revoked_at IS NOT NULL FROM refresh_tokens WHERE jti = $1
	`, jti).Scan(&revoked)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "missing", nil
		}
		return "", fmt.Errorf("check refresh token: %w", err)
	}
	if revoked {
		return "already_revoked", nil
	}
	return "missing", nil
}

// Prune deletes rows revoked or expired before olderThan and returns the
// number of rows removed.
func (s *RefreshTokenPostgresStore) Prune(ctx context.Context, olderThan time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM refresh_tokens
		WHERE revoked_at < $1 OR expires_at < $1
	`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("prune refresh tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}