package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

// StartWorker polls for pending jobs and processes them. It also prunes
// email history older than 30 days once per hour.
func (q *emailQueue) StartWorker(ctx context.Context, mailer Mailer) {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		var lastPrune time.Time

		log.Info().Msg("email queue worker started")

		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("email queue worker stopped")
				return
			case <-ticker.C:
				if time.Since(lastPrune) >= time.Hour {
					removed, err := q.PruneHistory(time.Now().Add(-30 * 24 * time.Hour))
					if err != nil {
						log.Error().Err(err).Msg("error pruning email history")
					} else if removed > 0 {
						log.Info().Int64("removed", removed).Msg("pruned old email history")
					}
					lastPrune = time.Now()
				}
				if err := q.processNextBatch(ctx, mailer); err != nil {
					log.Error().Err(err).Msg("error processing email batch")
				}
			}
		}
	}()
}

// StopWorker stops the worker by cancelling its context.
func (q *emailQueue) StopWorker(cancel context.CancelFunc) {
	cancel()
}

func (q *emailQueue) processNextBatch(ctx context.Context, mailer Mailer) error {
	// Reap jobs stuck in 'processing' (e.g. by a worker that crashed
	// mid-send) so they become claimable again.
	if _, err := q.ResetStaleProcessing(ctx, time.Now().Add(-5*time.Minute)); err != nil {
		return fmt.Errorf("reap stale email jobs: %w", err)
	}

	jobs, err := q.Claim(ctx, q.batchSize)
	if err != nil {
		return fmt.Errorf("claim email jobs: %w", err)
	}

	for _, j := range jobs {
		if err := q.sendJob(ctx, j, mailer); err != nil {
			log.Error().Str("id", j.ID).Err(err).Msg("failed to send email job")
		}
	}

	return nil
}

func (q *emailQueue) sendJob(ctx context.Context, j *EmailJob, mailer Mailer) error {
	if mailer != nil {
		if err := mailer.Send(j.ContextID, j.To, j.Subject, j.Body, j.HTMLBody); err != nil {
			return q.retryJob(ctx, j, err)
		}
	}

	return q.completeJob(ctx, j)
}

// toArray builds a pgtype.Array for a string slice. pgx encodes arrays
// from their dimensions, so Dims must be set for both empty and non-empty
// slices or the value is encoded as NULL.
func toArray(elements []string) pgtype.Array[string] {
	return pgtype.Array[string]{
		Elements: elements,
		Dims:     []pgtype.ArrayDimension{{Length: int32(len(elements)), LowerBound: 1}},
		Valid:    true,
	}
}

func (q *emailQueue) completeJob(ctx context.Context, j *EmailJob) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin complete transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	toArr := toArray(j.To)

	_, err = tx.Exec(ctx, `
		INSERT INTO email_history (id, context_id, to_addresses, subject, body, html_body, attempts, status, created_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'sent', $8, NOW())
	`, j.ID, j.ContextID, toArr, j.Subject, j.Body, j.HTMLBody, j.Attempts, j.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert sent history: %w", err)
	}

	_, err = tx.Exec(ctx, `DELETE FROM email_jobs WHERE id = $1`, j.ID)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete transaction: %w", err)
	}
	log.Info().Str("id", j.ID).Msg("email job completed")
	return nil
}

func (q *emailQueue) retryJob(ctx context.Context, j *EmailJob, err error) error {
	newAttempts := j.Attempts + 1

	if newAttempts >= j.MaxRetries {
		// Max retries exceeded: record the failure in history, then delete the job.
		tx, txErr := q.pool.Begin(ctx)
		if txErr != nil {
			return fmt.Errorf("begin failed history transaction: %w", txErr)
		}
		defer tx.Rollback(ctx)

		toArr := toArray(j.To)

		_, execErr := tx.Exec(ctx, `
			INSERT INTO email_history (id, context_id, to_addresses, subject, body, html_body, attempts, status, last_error, created_at, finished_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'failed', $8, $9, NOW())
		`, j.ID, j.ContextID, toArr, j.Subject, j.Body, j.HTMLBody, newAttempts, err.Error(), j.CreatedAt)
		if execErr != nil {
			return fmt.Errorf("insert failed history: %w", execErr)
		}

		_, execErr = tx.Exec(ctx, `DELETE FROM email_jobs WHERE id = $1`, j.ID)
		if execErr != nil {
			return fmt.Errorf("remove max-retry job: %w", execErr)
		}

		if execErr = tx.Commit(ctx); execErr != nil {
			return fmt.Errorf("commit failed history transaction: %w", execErr)
		}
		log.Error().Str("id", j.ID).Int("attempts", newAttempts).Msg("email job max retries exceeded")
		return nil
	}

	// Exponential backoff: 2^attempts * 10 seconds, capped at 1 hour
	backoff := time.Duration(1<<uint(newAttempts)) * 10 * time.Second
	if backoff > time.Hour {
		backoff = time.Hour
	}
	nextRetry := time.Now().Add(backoff)

	// Back to 'pending' so the job is claimable again at next_retry.
	query := `
		UPDATE email_jobs
		SET status = 'pending', attempts = $1, next_retry = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err = q.pool.Exec(ctx, query, newAttempts, nextRetry, j.ID)
	if err != nil {
		return fmt.Errorf("retry job: %w", err)
	}

	log.Warn().Str("id", j.ID).Int("attempt", newAttempts).Dur("backoff", backoff).Msg("email job scheduled for retry")
	return nil
}
