package queue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

type jobRow struct {
	id         string
	contextID  string
	to         []string
	toStr      string
	subject    string
	body       string
	htmlBody   string
	attempts   int
	maxRetries int
	createdAt  time.Time
}

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
	// Per-context starvation guard: at most 3 jobs per context per batch. The
	// window function lives in the CTE because FOR UPDATE cannot be combined
	// with window functions in the same query.
	query := `
		WITH candidates AS (
			SELECT id, created_at
			FROM (
				SELECT id, created_at, ROW_NUMBER() OVER (PARTITION BY context_id ORDER BY created_at) AS rn
				FROM email_jobs
				WHERE next_retry <= NOW() AND attempts < max_retries
			) t
			WHERE rn <= 3
			ORDER BY created_at
			LIMIT $1
		)
		SELECT e.id, e.context_id, e.to_addresses, e.subject, e.body, e.html_body, e.attempts, e.max_retries, e.created_at
		FROM email_jobs e
		JOIN candidates c ON c.id = e.id
		ORDER BY e.created_at
		FOR UPDATE SKIP LOCKED
	`

	rows, err := q.pool.Query(ctx, query, q.batchSize)
	if err != nil {
		return fmt.Errorf("query email jobs: %w", err)
	}
	defer rows.Close()

	var jobs []jobRow
	for rows.Next() {
		j := jobRow{}
		var toAddresses pgtype.Array[string]
		err := rows.Scan(&j.id, &j.contextID, &toAddresses, &j.subject, &j.body, &j.htmlBody, &j.attempts, &j.maxRetries, &j.createdAt)
		if err != nil {
			return fmt.Errorf("scan job: %w", err)
		}

		// Parse to_addresses from PostgreSQL array format
		var to []string
		if toAddresses.Valid && len(toAddresses.Elements) > 0 {
			to = toAddresses.Elements
		}
		j.to = to
		j.toStr = strings.Join(to, ",")

		jobs = append(jobs, j)
	}

	for _, j := range jobs {
		if err := q.sendJob(ctx, j, mailer); err != nil {
			log.Error().Str("id", j.id).Err(err).Msg("failed to send email job")
		}
	}

	return nil
}

func (q *emailQueue) sendJob(ctx context.Context, j jobRow, mailer Mailer) error {
	// Parse to addresses
	toStr := strings.Trim(j.toStr, "{}")
	var to []string
	if toStr != "" {
		parts := strings.Split(toStr, ",")
		for _, p := range parts {
			p = strings.Trim(p, "\"")
			if p != "" {
				to = append(to, p)
			}
		}
	}

	if mailer != nil {
		if err := mailer.Send(j.contextID, to, j.subject, j.body, j.htmlBody); err != nil {
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

func (q *emailQueue) completeJob(ctx context.Context, j jobRow) error {
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin complete transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	toArr := toArray(j.to)

	_, err = tx.Exec(ctx, `
		INSERT INTO email_history (id, context_id, to_addresses, subject, body, html_body, attempts, status, created_at, finished_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'sent', $8, NOW())
	`, j.id, j.contextID, toArr, j.subject, j.body, j.htmlBody, j.attempts, j.createdAt)
	if err != nil {
		return fmt.Errorf("insert sent history: %w", err)
	}

	_, err = tx.Exec(ctx, `DELETE FROM email_jobs WHERE id = $1`, j.id)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete transaction: %w", err)
	}
	log.Info().Str("id", j.id).Msg("email job completed")
	return nil
}

func (q *emailQueue) retryJob(ctx context.Context, j jobRow, err error) error {
	newAttempts := j.attempts + 1

	var nextRetry time.Time
	if newAttempts >= j.maxRetries {
		// Max retries exceeded: record the failure in history, then delete the job.
		tx, txErr := q.pool.Begin(ctx)
		if txErr != nil {
			return fmt.Errorf("begin failed history transaction: %w", txErr)
		}
		defer tx.Rollback(ctx)

		toArr := toArray(j.to)

		_, execErr := tx.Exec(ctx, `
			INSERT INTO email_history (id, context_id, to_addresses, subject, body, html_body, attempts, status, last_error, created_at, finished_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 'failed', $8, $9, NOW())
		`, j.id, j.contextID, toArr, j.subject, j.body, j.htmlBody, newAttempts, err.Error(), j.createdAt)
		if execErr != nil {
			return fmt.Errorf("insert failed history: %w", execErr)
		}

		_, execErr = tx.Exec(ctx, `DELETE FROM email_jobs WHERE id = $1`, j.id)
		if execErr != nil {
			return fmt.Errorf("remove max-retry job: %w", execErr)
		}

		if execErr = tx.Commit(ctx); execErr != nil {
			return fmt.Errorf("commit failed history transaction: %w", execErr)
		}
		log.Error().Str("id", j.id).Int("attempts", newAttempts).Msg("email job max retries exceeded")
		return nil
	}

	// Exponential backoff: 2^attempts * 10 seconds, capped at 1 hour
	backoff := time.Duration(1<<uint(newAttempts)) * 10 * time.Second
	if backoff > time.Hour {
		backoff = time.Hour
	}
	nextRetry = time.Now().Add(backoff)

	query := `
		UPDATE email_jobs SET attempts = $1, next_retry = $2
		WHERE id = $3
	`
	_, err = q.pool.Exec(ctx, query, newAttempts, nextRetry, j.id)
	if err != nil {
		return fmt.Errorf("retry job: %w", err)
	}

	log.Warn().Str("id", j.id).Int("attempt", newAttempts).Dur("backoff", backoff).Msg("email job scheduled for retry")
	return nil
}
