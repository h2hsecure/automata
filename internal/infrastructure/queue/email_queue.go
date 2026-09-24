package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// EmailJob represents a pending email to send.
type EmailJob struct {
	ID         string
	ContextID   string
	To         []string
	Subject    string
	Body       string
	HTMLBody   string
	Attempts   int
	MaxRetries int
	NextRetry  time.Time
	CreatedAt  time.Time
}

// Mailer interface for sending emails.
type Mailer interface {
	Send(to []string, subject, body, htmlBody string) error
}

// Queue defines the email queue interface.
type Queue interface {
	Enqueue(job *EmailJob) error
	StartWorker(ctx context.Context, mailer Mailer)
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

	toStr := "{" + fmt.Sprint(job.To) + "}"

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
