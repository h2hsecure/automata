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
	TenantID   string
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
	Enqueue(url string, payload map[string]interface{}, tenantID, formID string) error
	GetPending(limit int) ([]*WebhookDelivery, error)
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

// Enqueue adds a webhook delivery to the queue.
func (q *webhookQueue) Enqueue(url string, payload map[string]interface{}, tenantID, formID string) error {
	id := uuid.New().String()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	query := `
		INSERT INTO webhook_deliveries (id, tenant_id, form_id, url, payload, status, attempts, max_retries, next_retry, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'pending', 0, 3, NOW(), NOW(), NOW())
	`

	_, err = q.pool.Exec(context.Background(), query,
		id, tenantID, formID, url, payloadBytes,
	)
	if err != nil {
		return fmt.Errorf("enqueue webhook: %w", err)
	}

	log.Info().Str("id", id).Str("url", url).Msg("webhook enqueued")
	return nil
}

// GetPending retrieves pending webhook deliveries ready for processing.
func (q *webhookQueue) GetPending(limit int) ([]*WebhookDelivery, error) {
	query := `
		SELECT id, tenant_id, form_id, url, payload, status, attempts, max_retries, next_retry, error_msg, created_at, updated_at
		FROM webhook_deliveries
		WHERE status = 'pending' AND next_retry <= NOW()
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
			&d.ID, &d.TenantID, &d.FormID, &d.URL,
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
