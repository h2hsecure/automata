package database

import (
	"context"
	"fmt"
	"time"

	"github.com/h2hsecure/automata/internal/infrastructure/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool builds a pgxpool.Pool from config values.
// Expected config keys: database.host, database.port, database.name,
// database.user, database.password, database.max_connections.
func NewPool() (*pgxpool.Pool, error) {
	host := config.Get("database.host")
	port := config.Get("database.port")
	name := config.Get("database.name")
	user := config.Get("database.user")
	password := config.Get("database.password")
	maxConns := config.Get("database.max_connections")

	if host == "" || name == "" || user == "" || password == "" {
		return nil, fmt.Errorf("database config missing required fields: host, name, user, password")
	}

	if port == "" {
		port = "5432"
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, name)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool config: %w", err)
	}

	poolConfig.MaxConns = 4
	if maxConns != "" {
		var maxConnInt int64
		if _, err := fmt.Sscanf(maxConns, "%d", &maxConnInt); err == nil && maxConnInt > 0 {
			poolConfig.MaxConns = int32(maxConnInt)
		}
	}

	poolConfig.ConnConfig.ConnectTimeout = 10 * time.Second

	connPool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	return connPool, nil
}

// Ping checks if the pool can connect to the database.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}

// Close closes all connections in the pool.
func Close(pool *pgxpool.Pool) {
	pool.Close()
}
