package queue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/infrastructure/queue"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestEmailQueue_ClaimConcurrentSingleWinner proves the claim is atomic:
// two workers racing for one pending job must yield exactly one winner.
func TestEmailQueue_ClaimConcurrentSingleWinner(t *testing.T) {
	pool, cleanup := startTestPostgres(t)
	defer cleanup()

	q := queue.NewEmailQueue(pool)
	ctx := context.Background()

	if err := q.Enqueue(&queue.EmailJob{
		ContextID: uuid.New().String(),
		To:        []string{"a@example.com"},
		Subject:   "s",
		Body:      "b",
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			jobs, err := q.Claim(ctx, 10)
			if err != nil {
				t.Errorf("Claim: %v", err)
				results <- 0
				return
			}
			results <- len(jobs)
		}()
	}
	wg.Wait()
	close(results)

	total := 0
	for n := range results {
		total += n
	}
	if total != 1 {
		t.Fatalf("concurrent claims returned %d jobs, want exactly 1", total)
	}
}

// TestEmailQueue_EnqueueToAddressesRoundTrip proves recipients are bound as
// a proper array parameter (no hand-built SQL literal) and survive a
// round-trip for both multi-recipient and empty lists.
func TestEmailQueue_EnqueueToAddressesRoundTrip(t *testing.T) {
	pool, cleanup := startTestPostgres(t)
	defer cleanup()

	q := queue.NewEmailQueue(pool)
	ctx := context.Background()

	multiID := uuid.New().String()
	if err := q.Enqueue(&queue.EmailJob{
		ID: multiID, ContextID: uuid.New().String(),
		To:      []string{"a@example.com", "b@example.com"},
		Subject: "s", Body: "b",
	}); err != nil {
		t.Fatalf("Enqueue multi: %v", err)
	}

	var multi pgtype.Array[string]
	if err := pool.QueryRow(ctx, `SELECT to_addresses FROM email_jobs WHERE id = $1`, multiID).Scan(&multi); err != nil {
		t.Fatalf("scan multi to_addresses: %v", err)
	}
	if len(multi.Elements) != 2 || multi.Elements[0] != "a@example.com" || multi.Elements[1] != "b@example.com" {
		t.Errorf("to_addresses = %v, want [a@example.com b@example.com]", multi.Elements)
	}

	emptyID := uuid.New().String()
	if err := q.Enqueue(&queue.EmailJob{
		ID: emptyID, ContextID: uuid.New().String(),
		To:      nil,
		Subject: "s", Body: "b",
	}); err != nil {
		t.Fatalf("Enqueue empty: %v", err)
	}

	var empty pgtype.Array[string]
	if err := pool.QueryRow(ctx, `SELECT to_addresses FROM email_jobs WHERE id = $1`, emptyID).Scan(&empty); err != nil {
		t.Fatalf("scan empty to_addresses: %v", err)
	}
	if len(empty.Elements) != 0 {
		t.Errorf("empty to_addresses = %v, want empty (not NULL)", empty.Elements)
	}
}

func TestEmailQueue_ResetStaleProcessing(t *testing.T) {
	pool, cleanup := startTestPostgres(t)
	defer cleanup()

	q := queue.NewEmailQueue(pool)
	ctx := context.Background()

	id := uuid.New().String()
	if err := q.Enqueue(&queue.EmailJob{
		ID: id, ContextID: uuid.New().String(), To: []string{"a@example.com"}, Subject: "s", Body: "b",
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Simulate a crashed worker: job stuck in processing with an old stamp.
	if _, err := pool.Exec(ctx, `
		UPDATE email_jobs
		SET status = 'processing', updated_at = NOW() - interval '10 minutes'
		WHERE id = $1
	`, id); err != nil {
		t.Fatalf("force stale processing: %v", err)
	}
	// Fresh processing jobs must not be reset.
	freshID := uuid.New().String()
	if err := q.Enqueue(&queue.EmailJob{
		ID: freshID, ContextID: uuid.New().String(), To: []string{"b@example.com"}, Subject: "s", Body: "b",
	}); err != nil {
		t.Fatalf("Enqueue 2: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE email_jobs SET status = 'processing' WHERE id = $1`, freshID); err != nil {
		t.Fatalf("force fresh processing: %v", err)
	}

	reset, err := q.ResetStaleProcessing(ctx, time.Now().Add(-5*time.Minute))
	if err != nil {
		t.Fatalf("ResetStaleProcessing: %v", err)
	}
	if reset != 1 {
		t.Fatalf("reset = %d, want 1 (only the stale job)", reset)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM email_jobs WHERE id = $1`, id).Scan(&status); err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != "pending" {
		t.Errorf("stale job status = %q, want pending", status)
	}
}
