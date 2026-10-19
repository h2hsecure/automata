package queue

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// ErrJobNotFound is returned by Retry and Cancel when the job does not
// exist in the expected state for the given context.
var ErrJobNotFound = errors.New("job not found")

// EmailJob represents a pending email to send.
type EmailJob struct {
	ID         string
	ContextID  string
	To         []string
	Subject    string
	Body       string
	HTMLBody   string
	Attempts   int
	MaxRetries int
	NextRetry  time.Time
	CreatedAt  time.Time
}

// Mailer interface for sending emails. The context ID lets the mailer
// resolve per-context SMTP settings at send time.
type Mailer interface {
	Send(contextID string, to []string, subject, body, htmlBody string) error
}

// EmailHistory records the outcome of a processed email job.
type EmailHistory struct {
	ID         string    `json:"id"`
	ContextID  string    `json:"context_id"`
	To         []string  `json:"to"`
	Subject    string    `json:"subject"`
	Attempts   int       `json:"attempts"`
	Status     string    `json:"status"`
	LastError  string    `json:"last_error"`
	CreatedAt  time.Time `json:"created_at"`
	FinishedAt time.Time `json:"finished_at"`
}

// Queue defines the email queue interface.
type Queue interface {
	Enqueue(job *EmailJob) error
	StartWorker(ctx context.Context, mailer Mailer)
	ListPending(contextID string, limit int) ([]*EmailJob, error)
	ListHistory(contextID string, status string, limit int) ([]*EmailHistory, error)
	Retry(contextID, jobID string) error
	Cancel(contextID, jobID string) error
	PruneHistory(olderThan time.Time) (int64, error)
}

type emailQueue struct {
	pool      *pgxpool.Pool
	batchSize int
}

// NewEmailQueue creates a new email queue.
func NewEmailQueue(pool *pgxpool.Pool) Queue {
	return &emailQueue{
		pool:      pool,
		batchSize: 10,
	}
}

// Enqueue adds an email job to the queue.
func (q *emailQueue) Enqueue(job *EmailJob) error {
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if job.MaxRetries == 0 {
		job.MaxRetries = 3
	}
	if job.To == nil {
		job.To = []string{}
	}

	toStr := "{" + strings.Join(job.To, ",") + "}"

	query := `
		INSERT INTO email_jobs (id, context_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`

	_, err := q.pool.Exec(context.Background(), query,
		job.ID, job.ContextID, toStr, job.Subject, job.Body, job.HTMLBody,
		job.Attempts, job.MaxRetries,
	)
	if err != nil {
		return fmt.Errorf("enqueue email job: %w", err)
	}

	log.Info().Str("id", job.ID).Strs("to", job.To).Msg("email job enqueued")
	return nil
}

// ListPending returns pending jobs for a context, newest first.
func (q *emailQueue) ListPending(contextID string, limit int) ([]*EmailJob, error) {
	query := `
		SELECT id, context_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at
		FROM email_jobs
		WHERE context_id = $1 AND attempts < max_retries
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := q.pool.Query(context.Background(), query, contextID, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*EmailJob
	for rows.Next() {
		job := &EmailJob{}
		var toAddresses pgtype.Array[string]
		var htmlBody pgtype.Text
		var nextRetry pgtype.Timestamptz
		if err := rows.Scan(&job.ID, &job.ContextID, &toAddresses, &job.Subject, &job.Body, &htmlBody, &job.Attempts, &job.MaxRetries, &nextRetry, &job.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending job: %w", err)
		}
		if toAddresses.Valid {
			job.To = toAddresses.Elements
		} else {
			job.To = []string{}
		}
		job.HTMLBody = htmlBody.String
		if nextRetry.Valid {
			job.NextRetry = nextRetry.Time
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending jobs: %w", err)
	}
	return jobs, nil
}

// ListHistory returns processed job history for a context, newest first.
// An empty status returns all statuses.
func (q *emailQueue) ListHistory(contextID string, status string, limit int) ([]*EmailHistory, error) {
	query := `
		SELECT id, context_id, to_addresses, subject, attempts, status, last_error, created_at, finished_at
		FROM email_history
		WHERE context_id = $1
	`
	args := []interface{}{contextID}
	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}
	query += " ORDER BY finished_at DESC LIMIT $" + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := q.pool.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()

	var history []*EmailHistory
	for rows.Next() {
		h := &EmailHistory{}
		var toAddresses pgtype.Array[string]
		var lastError pgtype.Text
		if err := rows.Scan(&h.ID, &h.ContextID, &toAddresses, &h.Subject, &h.Attempts, &h.Status, &lastError, &h.CreatedAt, &h.FinishedAt); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		if toAddresses.Valid {
			h.To = toAddresses.Elements
		} else {
			h.To = []string{}
		}
		h.LastError = lastError.String
		history = append(history, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history rows: %w", err)
	}
	return history, nil
}

// Retry re-enqueues a failed job (from history) with attempts reset to 0.
func (q *emailQueue) Retry(contextID, jobID string) error {
	ctx := context.Background()
	tx, err := q.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin retry transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var (
		toAddresses pgtype.Array[string]
		subject     string
		body        string
		htmlBody    pgtype.Text
	)
	err = tx.QueryRow(ctx, `
		SELECT to_addresses, subject, body, html_body
		FROM email_history
		WHERE context_id = $1 AND id = $2 AND status = 'failed'
	`, contextID, jobID).Scan(&toAddresses, &subject, &body, &htmlBody)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("retry: %w: %s", ErrJobNotFound, jobID)
		}
		return fmt.Errorf("read failed job: %w", err)
	}

	var toArr pgtype.Array[string]
	if toAddresses.Valid {
		toArr = toAddresses
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO email_jobs (id, context_id, to_addresses, subject, body, html_body, attempts, max_retries, next_retry, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, 0, 3, NOW(), NOW())
	`, jobID, contextID, toArr, subject, body, htmlBody)
	if err != nil {
		return fmt.Errorf("re-enqueue job: %w", err)
	}

	_, err = tx.Exec(ctx, `DELETE FROM email_history WHERE id = $1 AND context_id = $2`, jobID, contextID)
	if err != nil {
		return fmt.Errorf("delete history row: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit retry transaction: %w", err)
	}

	log.Info().Str("id", jobID).Msg("email job re-enqueued for retry")
	return nil
}

// Cancel removes a pending job for a context.
func (q *emailQueue) Cancel(contextID, jobID string) error {
	tag, err := q.pool.Exec(context.Background(), `DELETE FROM email_jobs WHERE id = $1 AND context_id = $2`, jobID, contextID)
	if err != nil {
		return fmt.Errorf("cancel job: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("cancel: %w: %s", ErrJobNotFound, jobID)
	}
	return nil
}

// PruneHistory deletes history rows older than olderThan and returns the
// number of rows removed.
func (q *emailQueue) PruneHistory(olderThan time.Time) (int64, error) {
	tag, err := q.pool.Exec(context.Background(), `DELETE FROM email_history WHERE finished_at < $1`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("prune history: %w", err)
	}
	return tag.RowsAffected(), nil
}
