package queue_test

import (
	"context"
	"sync"
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

// TestWebhookQueue_ClaimConcurrentSingleWinner proves the claim is atomic:
// two workers racing for one pending row must yield exactly one winner.
func TestWebhookQueue_ClaimConcurrentSingleWinner(t *testing.T) {
	pool, cleanup := startTestPostgres(t)
	defer cleanup()

	q := queue.NewWebhookQueue(pool)
	ctx := context.Background()

	if _, err := q.Enqueue(ctx, "https://example.com/hook", map[string]interface{}{"a": 1}, uuid.New().String(), ""); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan []string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deliveries, err := q.Claim(ctx, 10)
			if err != nil {
				t.Errorf("Claim: %v", err)
				return
			}
			ids := make([]string, len(deliveries))
			for i, d := range deliveries {
				ids[i] = d.ID
			}
			results <- ids
		}()
	}
	wg.Wait()
	close(results)

	total := 0
	for ids := range results {
		total += len(ids)
	}
	if total != 1 {
		t.Fatalf("concurrent claims returned %d deliveries, want exactly 1", total)
	}
}

func TestWebhookQueue_ClaimMarksProcessingAndSecondClaimEmpty(t *testing.T) {
	pool, cleanup := startTestPostgres(t)
	defer cleanup()

	q := queue.NewWebhookQueue(pool)
	ctx := context.Background()

	id, err := q.Enqueue(ctx, "https://example.com/hook", map[string]interface{}{"a": 1}, uuid.New().String(), "")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	deliveries, err := q.Claim(ctx, 10)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].ID != id {
		t.Fatalf("claimed = %d deliveries, want 1 for %s", len(deliveries), id)
	}
	if deliveries[0].Status != "processing" {
		t.Errorf("claimed status = %q, want processing", deliveries[0].Status)
	}

	second, err := q.Claim(ctx, 10)
	if err != nil {
		t.Fatalf("second Claim: %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("second claim = %d deliveries, want 0 (row is processing)", len(second))
	}
}

func TestWebhookQueue_ResetStaleProcessing(t *testing.T) {
	pool, cleanup := startTestPostgres(t)
	defer cleanup()

	q := queue.NewWebhookQueue(pool)
	ctx := context.Background()

	id, err := q.Enqueue(ctx, "https://example.com/hook", map[string]interface{}{"a": 1}, uuid.New().String(), "")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Simulate a crashed worker: row stuck in processing with an old stamp.
	if _, err := pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status = 'processing', updated_at = NOW() - interval '10 minutes'
		WHERE id = $1
	`, id); err != nil {
		t.Fatalf("force stale processing: %v", err)
	}

	// Fresh processing rows must not be reset.
	if _, err := q.Enqueue(ctx, "https://example.com/hook2", map[string]interface{}{}, uuid.New().String(), ""); err != nil {
		t.Fatalf("Enqueue 2: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE webhook_deliveries SET status = 'processing' WHERE url = 'https://example.com/hook2'`); err != nil {
		t.Fatalf("force fresh processing: %v", err)
	}

	reset, err := q.ResetStaleProcessing(ctx, time.Now().Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("ResetStaleProcessing: %v", err)
	}
	if reset != 1 {
		t.Fatalf("reset = %d, want 1 (only the stale row)", reset)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM webhook_deliveries WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != "retrying" {
		t.Errorf("stale row status = %q, want retrying", status)
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
