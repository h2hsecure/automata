package support

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDB wraps a PostgreSQL test container with a pgxpool connection.
type TestDB struct {
	Container testcontainers.Container
	Pool      *pgxpool.Pool
	URI       string
}

// SetupTestDB creates a new PostgreSQL test container and returns a TestDB instance.
func SetupTestDB(t interface{}) (*TestDB, error) {
	ctx := context.Background()

	// Start PostgreSQL container
	container, err := postgres.RunContainer(
		ctx,
		testcontainers.WithImage("postgres:17-alpine"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	// Get container URI
	uri, err := container.ConnectionString(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("get container URI: %w", err)
	}

	// Create connection pool
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		container.Terminate(ctx)
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		container.Terminate(ctx)
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &TestDB{
		Container: container,
		Pool:      pool,
		URI:       uri,
	}, nil
}

// Close tears down the test container and closes the connection pool.
func (tdb *TestDB) Close() {
	ctx := context.Background()
	if tdb.Pool != nil {
		tdb.Pool.Close()
	}
	if tdb.Container != nil {
		tdb.Container.Terminate(ctx)
	}
}

// RunMigrations executes all embedded migrations against the test database.
func (tdb *TestDB) RunMigrations(ctx context.Context) error {
	migrationDirs := []string{
		"internal/infrastructure/database",
		"internal/infrastructure/contact",
		"internal/infrastructure/form/repo",
		"internal/infrastructure/banner/repo",
		"internal/infrastructure/tracking/repo",
		"internal/infrastructure/queue",
	}

	// Get the project root directory (go up from cicd/ if needed)
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}
	
	// Check if we're in the cicd/ directory and go up if needed
	if strings.HasSuffix(wd, "/cicd") || wd == "cicd" {
		wd = filepath.Dir(wd)
	}

	for _, dir := range migrationDirs {
		migrationDir := fmt.Sprintf("%s/%s", wd, dir)
		if err := runMigrationDir(ctx, tdb.Pool, migrationDir); err != nil {
			return fmt.Errorf("run migrations from %s: %w", dir, err)
		}
	}

	return nil
}

// runMigrationDir reads and executes all .sql files in a directory.
func runMigrationDir(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migration directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		migrationPath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("read migration file %s: %w", migrationPath, err)
		}

		if _, err := pool.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("execute migration %s: %w", migrationPath, err)
		}
	}

	return nil
}

// runMigrationFile reads and executes a migration SQL file.
func runMigrationFile(ctx context.Context, pool *pgxpool.Pool, path string) error {
	// Read migration file
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read migration file %s: %w", path, err)
	}

	// Execute migration
	if _, err := pool.Exec(ctx, string(data)); err != nil {
		return fmt.Errorf("execute migration %s: %w", path, err)
	}

	return nil
}
