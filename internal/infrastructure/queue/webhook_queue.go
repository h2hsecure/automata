package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// WebhookDelivery represents a pending or completed webhook delivery.
type WebhookDelivery struct {
	ID         string
	ContextID  string
	FormID     string
	URL        string
	Payload    map[string]interface{}
	Status     string
	Attempts   int
	MaxRetries int
	NextRetry  time.Time
	ErrorMsg   *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// WebhookQueue defines the interface for webhook delivery queue operations.
type WebhookQueue interface {
	Enqueue(ctx context.Context, url string, payload map[string]interface{}, contextID, formID string) (string, error)
	GetPending(limit int) ([]*WebhookDelivery, error)
	Claim(ctx context.Context, limit int) ([]*WebhookDelivery, error)
	ResetStaleProcessing(ctx context.Context, olderThan time.Time) (int64, error)
	MarkSuccess(id string) error
	MarkFailed(id string, err string) error
	UpdateNextRetry(id string, nextRetry time.Time) error
}

type webhookQueue struct {
	pool *pgxpool.Pool
}

// NewWebhookQueue creates a new webhook queue.
func NewWebhookQueue(pool *pgxpool.Pool) WebhookQueue {
	return &webhookQueue{
		pool: pool,
	}
}

// Enqueue adds a webhook delivery to the queue and returns its delivery ID.
func (q *webhookQueue) Enqueue(ctx context.Context, url string, payload map[string]interface{}, contextID, formID string) (string, error) {
	id := uuid.New().String()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal webhook payload: %w", err)
	}

	query := `
		INSERT INTO webhook_deliveries (id, context_id, form_id, url, payload, status, attempts, max_retries, next_retry, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', 0, 3, NOW(), NOW(), NOW())
	`

	// form_id is nullable: deliveries without a form (e.g. webhooks_test) bind NULL.
	var formIDParam interface{}
	if formID != "" {
		formIDParam = formID
	}

	_, err = q.pool.Exec(ctx, query,
		id, contextID, formIDParam, url, payloadBytes,
	)
	if err != nil {
		return "", fmt.Errorf("enqueue webhook: %w", err)
	}

	log.Info().Str("id", id).Str("url", url).Msg("webhook enqueued")
	return id, nil
}

// GetPending retrieves pending webhook deliveries ready for processing.
func (q *webhookQueue) GetPending(limit int) ([]*WebhookDelivery, error) {
	query := `
		SELECT id, context_id, COALESCE(form_id::text, ''), url, payload, status, attempts, max_retries, next_retry, error_msg, created_at, updated_at
		FROM webhook_deliveries
		WHERE status IN ('pending', 'retrying') AND next_retry <= NOW()
		ORDER BY created_at ASC
		LIMIT $1
	`

	rows, err := q.pool.Query(context.Background(), query, limit)
	if err != nil {
		return nil, fmt.Errorf("query pending webhooks: %w", err)
	}
	defer rows.Close()

	var deliveries []*WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		var payloadBytes []byte

		err := rows.Scan(
			&d.ID, &d.ContextID, &d.FormID, &d.URL,
			&payloadBytes, &d.Status, &d.Attempts, &d.MaxRetries,
			&d.NextRetry, &d.ErrorMsg, &d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan webhook delivery: %w", err)
		}

		if err := json.Unmarshal(payloadBytes, &d.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal webhook payload: %w", err)
		}

		deliveries = append(deliveries, &d)
	}

	return deliveries, nil
}

// Claim atomically takes ownership of up to limit claimable deliveries: the
// picked rows are locked and flipped to 'processing' in a single statement,
// so two racing workers cannot claim the same row.
func (q *webhookQueue) Claim(ctx context.Context, limit int) ([]*WebhookDelivery, error) {
	query := `
		WITH picked AS (
			SELECT id FROM webhook_deliveries
			WHERE status IN ('pending', 'retrying') AND next_retry <= NOW()
			ORDER BY created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE webhook_deliveries d
		SET status = 'processing', updated_at = NOW()
		FROM picked p WHERE d.id = p.id
		RETURNING d.id, d.context_id, COALESCE(d.form_id::text, ''), d.url, d.payload,
			d.status, d.attempts, d.max_retries, d.next_retry, d.error_msg,
			d.created_at, d.updated_at
	`

	rows, err := q.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("claim webhooks: %w", err)
	}
	defer rows.Close()

	var deliveries []*WebhookDelivery
	for rows.Next() {
		var d WebhookDelivery
		var payloadBytes []byte

		if err := rows.Scan(
			&d.ID, &d.ContextID, &d.FormID, &d.URL,
			&payloadBytes, &d.Status, &d.Attempts, &d.MaxRetries,
			&d.NextRetry, &d.ErrorMsg, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan claimed webhook: %w", err)
		}
		if err := json.Unmarshal(payloadBytes, &d.Payload); err != nil {
			return nil, fmt.Errorf("unmarshal claimed webhook payload: %w", err)
		}
		deliveries = append(deliveries, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed webhooks: %w", err)
	}
	return deliveries, nil
}

// ResetStaleProcessing returns deliveries stuck in 'processing' (e.g. by a
// worker that crashed mid-delivery) to 'retrying' so they become claimable
// again. It returns the number of rows reset.
func (q *webhookQueue) ResetStaleProcessing(ctx context.Context, olderThan time.Time) (int64, error) {
	tag, err := q.pool.Exec(ctx, `
		UPDATE webhook_deliveries
		SET status = 'retrying', updated_at = NOW()
		WHERE status = 'processing' AND updated_at < $1
	`, olderThan)
	if err != nil {
		return 0, fmt.Errorf("reset stale webhook deliveries: %w", err)
	}
	if tag.RowsAffected() > 0 {
		log.Warn().Int64("reset", tag.RowsAffected()).Msg("reset stale webhook deliveries")
	}
	return tag.RowsAffected(), nil
}

// MarkSuccess marks a webhook delivery as successfully delivered.
func (q *webhookQueue) MarkSuccess(id string) error {
	query := `
		UPDATE webhook_deliveries
		SET status = 'success', updated_at = NOW()
		WHERE id = $1
	`

	_, err := q.pool.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("mark webhook success: %w", err)
	}

	log.Info().Str("id", id).Msg("webhook delivered successfully")
	return nil
}

// MarkFailed marks a webhook delivery as permanently failed.
func (q *webhookQueue) MarkFailed(id string, errMsg string) error {
	query := `
		UPDATE webhook_deliveries
		SET status = 'failed', error_msg = $2, updated_at = NOW()
		WHERE id = $1
	`

	_, err := q.pool.Exec(context.Background(), query, id, errMsg)
	if err != nil {
		return fmt.Errorf("mark webhook failed: %w", err)
	}

	log.Error().Str("id", id).Str("error", errMsg).Msg("webhook permanently failed")
	return nil
}

// UpdateNextRetry marks a webhook for retry at the specified time.
func (q *webhookQueue) UpdateNextRetry(id string, nextRetry time.Time) error {
	query := `
		UPDATE webhook_deliveries
		SET status = 'retrying', next_retry = $2, attempts = attempts + 1, updated_at = NOW()
		WHERE id = $1
	`

	_, err := q.pool.Exec(context.Background(), query, id, nextRetry)
	if err != nil {
		return fmt.Errorf("update webhook retry: %w", err)
	}

	log.Warn().Str("id", id).Time("next_retry", nextRetry).Msg("webhook scheduled for retry")
	return nil
}
