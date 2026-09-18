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
	id           string
	tenantID     string
	toStr        string
	subject      string
	body         string
	htmlBody     string
	attempts     int
	maxRetries   int
	createdAt    time.Time
}

// StartWorker polls for pending jobs and processes them.
func (q *emailQueue) StartWorker(ctx context.Context, mailer Mailer) {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		log.Info().Msg("email queue worker started")

		for {
			select {
			case <-ctx.Done():
				log.Info().Msg("email queue worker stopped")
				return
			case <-ticker.C:
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
	query := `
		SELECT id, tenant_id, to_addresses, subject, body, html_body, attempts, max_retries, created_at
		FROM email_jobs
		WHERE next_retry <= NOW() AND attempts < max_retries
		ORDER BY created_at ASC
		LIMIT $1
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
		err := rows.Scan(&j.id, &j.tenantID, &toAddresses, &j.subject, &j.body, &j.htmlBody, &j.attempts, &j.maxRetries, &j.createdAt)
		if err != nil {
			return fmt.Errorf("scan job: %w", err)
		}

		// Parse to_addresses from PostgreSQL array format
		var to []string
		if toAddresses.Valid && len(toAddresses.Elements) > 0 {
			to = toAddresses.Elements
		}
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
		if err := mailer.Send(to, j.subject, j.body, j.htmlBody); err != nil {
			return q.retryJob(ctx, j, err)
		}
	}

	return q.completeJob(ctx, j.id)
}

func (q *emailQueue) completeJob(ctx context.Context, id string) error {
	query := `DELETE FROM email_jobs WHERE id = $1`
	_, err := q.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	log.Info().Str("id", id).Msg("email job completed")
	return nil
}

func (q *emailQueue) retryJob(ctx context.Context, j jobRow, err error) error {
	newAttempts := j.attempts + 1

	var nextRetry time.Time
	if newAttempts >= j.maxRetries {
		// Max retries exceeded, delete the job
		query := `DELETE FROM email_jobs WHERE id = $1`
		_, execErr := q.pool.Exec(ctx, query, j.id)
		if execErr != nil {
			return fmt.Errorf("remove max-retry job: %w", execErr)
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
