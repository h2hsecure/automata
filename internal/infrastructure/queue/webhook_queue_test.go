package queue_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/infrastructure/queue"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startTestPostgres(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	container, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("connection string: %v", err)
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("create pool: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("ping: %v", err)
	}
	if err := queue.RunMigrations(pool); err != nil {
		_ = container.Terminate(ctx)
		t.Fatalf("run migrations: %v", err)
	}
	return pool, func() {
		pool.Close()
		_ = container.Terminate(ctx)
	}
}

// TestWebhookQueue_EnqueueWithoutFormID covers the webhooks_test regression:
// DeliverTest enqueues with an empty form_id, which used to fail with
// "invalid input syntax for type uuid" because form_id was NOT NULL.
func TestWebhookQueue_EnqueueWithoutFormID(t *testing.T) {
	pool, cleanup := startTestPostgres(t)
	defer cleanup()

	q := queue.NewWebhookQueue(pool)
	ctx := context.Background()

	id, err := q.Enqueue(ctx, "https://example.com/hook", map[string]interface{}{"test": true}, uuid.New().String(), "")
	if err != nil {
		t.Fatalf("Enqueue with empty formID: %v", err)
	}

	deliveries, err := q.GetPending(10)
	if err != nil {
		t.Fatalf("GetPending: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 pending delivery, got %d", len(deliveries))
	}
	d := deliveries[0]
	if d.ID != id {
		t.Errorf("delivery ID = %q, want %q", d.ID, id)
	}
	if d.FormID != "" {
		t.Errorf("FormID = %q, want empty", d.FormID)
	}
	if d.URL != "https://example.com/hook" {
		t.Errorf("URL = %q", d.URL)
	}
	if d.Payload["test"] != true {
		t.Errorf("Payload = %v", d.Payload)
	}
}

func TestWebhookQueue_EnqueueWithFormID(t *testing.T) {
	pool, cleanup := startTestPostgres(t)
	defer cleanup()

	q := queue.NewWebhookQueue(pool)
	ctx := context.Background()
	formID := uuid.New().String()

	id, err := q.Enqueue(ctx, "https://example.com/hook", map[string]interface{}{"form": formID}, uuid.New().String(), formID)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	deliveries, err := q.GetPending(10)
	if err != nil {
		t.Fatalf("GetPending: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 pending delivery, got %d", len(deliveries))
	}
	if got := deliveries[0].FormID; got != formID {
		t.Errorf("FormID = %q, want %q", got, formID)
	}
	if deliveries[0].ID != id {
		t.Errorf("delivery ID = %q, want %q", deliveries[0].ID, id)
	}
}
