package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationLockKey is a stable advisory-lock key shared by every migration
// set in the process. It serializes concurrent migration runs across pods
// (the automata and bff deployments both run migrations at startup),
// preventing races on non-atomic DDL such as CREATE TABLE IF NOT EXISTS.
const migrationLockKey int64 = 0x0A10CA7E

// RunUnderMigrationLock runs fn while holding a session-level advisory lock
// on a dedicated connection, so concurrent migration runs from multiple
// processes/pods are serialized. fn executes on the lock-holding connection;
// the lock is released when fn returns. A session-level lock is also released
// automatically by Postgres if the connection drops (e.g. the pod is killed),
// so a crashed runner cannot wedge the lock.
func RunUnderMigrationLock(ctx context.Context, db *pgxpool.Pool, fn func(conn *pgx.Conn) error) error {
	conn, err := db.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for migration lock: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()

	return fn(conn.Conn())
}
