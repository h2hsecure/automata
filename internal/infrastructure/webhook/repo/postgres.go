package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/h2hsecure/automata/internal/domain/webhook"
	"github.com/jackc/pgx/v5/pgxpool"
)

// webhookRepo implements the webhook.Repository interface using PostgreSQL.
type webhookRepo struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL webhook endpoint repository.
func New(pool *pgxpool.Pool) webhook.Repository {
	return &webhookRepo{pool: pool}
}

// Create inserts a new webhook endpoint.
func (r *webhookRepo) Create(e *webhook.WebhookEndpoint) error {
	ctx := context.Background()

	eventsStr := webhook.FormatEvents(e.Events)
	query := `
		INSERT INTO webhook_endpoints (id, context_id, name, url, events, secret_hash, signing_secret_enc, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
	`

	// The signing secret is encrypted at rest; plaintext never touches the DB.
	var encSecret interface{}
	if e.SigningSecret != nil && *e.SigningSecret != "" {
		enc, err := encryptSecret(*e.SigningSecret)
		if err != nil {
			return fmt.Errorf("encrypt signing secret: %w", err)
		}
		encSecret = enc
	}

	id := uuid.New().String()
	_, err := r.pool.Exec(ctx, query,
		id, e.ContextID, e.Name, e.URL, eventsStr,
		e.SecretHash, encSecret, e.Active,
	)
	if err != nil {
		return fmt.Errorf("create webhook endpoint: %w", err)
	}

	e.ID = id
	return nil
}

// GetByID retrieves a webhook endpoint by its ID.
func (r *webhookRepo) GetByID(id string) (*webhook.WebhookEndpoint, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, name, url, events, secret_hash, signing_secret_enc, active, created_at, updated_at
		FROM webhook_endpoints
		WHERE id = $1
	`

	e := &webhook.WebhookEndpoint{}
	var eventsStr string

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.ContextID, &e.Name, &e.URL, &eventsStr,
		&e.SecretHash, &e.SigningSecret, &e.Active, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("%w: %w", webhook.ErrNotFound, sql.ErrNoRows)
		}
		return nil, fmt.Errorf("get webhook endpoint: %w", err)
	}

	if err := decryptSigningSecret(e); err != nil {
		return nil, fmt.Errorf("decrypt webhook endpoint secret: %w", err)
	}

	e.Events = webhook.ParseEvents(eventsStr)
	return e, nil
}

// decryptSigningSecret replaces the encrypted-at-rest signing secret with the
// plaintext value (in memory only, at delivery time). A NULL/empty stored
// value leaves SigningSecret as nil.
func decryptSigningSecret(e *webhook.WebhookEndpoint) error {
	if e.SigningSecret == nil || *e.SigningSecret == "" {
		return nil
	}
	plain, err := decryptSecret(*e.SigningSecret)
	if err != nil {
		return err
	}
	e.SigningSecret = &plain
	return nil
}

// ListByContext retrieves all webhook endpoints for a context, ordered by name.
func (r *webhookRepo) ListByContext(contextID string) ([]*webhook.WebhookEndpoint, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, name, url, events, secret_hash, signing_secret_enc, active, created_at, updated_at
		FROM webhook_endpoints
		WHERE context_id = $1
		ORDER BY name ASC
	`

	rows, err := r.pool.Query(ctx, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}
	defer rows.Close()

	endpoints := make([]*webhook.WebhookEndpoint, 0)
	for rows.Next() {
		e := &webhook.WebhookEndpoint{}
		var eventsStr string

		err := rows.Scan(
			&e.ID, &e.ContextID, &e.Name, &e.URL, &eventsStr,
			&e.SecretHash, &e.SigningSecret, &e.Active, &e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan webhook endpoint: %w", err)
		}

		if err := decryptSigningSecret(e); err != nil {
			return nil, fmt.Errorf("decrypt webhook endpoint secret: %w", err)
		}

		e.Events = webhook.ParseEvents(eventsStr)
		endpoints = append(endpoints, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook endpoints: %w", err)
	}

	return endpoints, nil
}

// Update modifies an existing webhook endpoint, only updating non-empty fields.
func (r *webhookRepo) Update(e *webhook.WebhookEndpoint) error {
	ctx := context.Background()

	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{e.ID}
	argIndex := 2

	if e.Name != "" {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIndex))
		args = append(args, e.Name)
		argIndex++
	}
	if e.URL != "" {
		setClauses = append(setClauses, fmt.Sprintf("url = $%d", argIndex))
		args = append(args, e.URL)
		argIndex++
	}
	if len(e.Events) > 0 {
		setClauses = append(setClauses, fmt.Sprintf("events = $%d", argIndex))
		args = append(args, webhook.FormatEvents(e.Events))
		argIndex++
	}
	if e.SecretHash != nil {
		setClauses = append(setClauses, fmt.Sprintf("secret_hash = $%d", argIndex))
		args = append(args, *e.SecretHash)
		argIndex++
	}
	if e.SigningSecret != nil && *e.SigningSecret != "" {
		enc, err := encryptSecret(*e.SigningSecret)
		if err != nil {
			return fmt.Errorf("encrypt signing secret: %w", err)
		}
		setClauses = append(setClauses, fmt.Sprintf("signing_secret_enc = $%d", argIndex))
		args = append(args, enc)
		argIndex++
	}

	// If active flag changed, use a separate update
	if e.Active {
		setClauses = append(setClauses, fmt.Sprintf("active = $%d", argIndex))
		args = append(args, true)
	}

	setClause := strings.Join(setClauses, ", ")
	query := fmt.Sprintf("UPDATE webhook_endpoints SET %s WHERE id = $1", setClause)

	_, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update webhook endpoint: %w", err)
	}

	return nil
}

// Delete removes a webhook endpoint by ID.
func (r *webhookRepo) Delete(id string) error {
	ctx := context.Background()
	query := "DELETE FROM webhook_endpoints WHERE id = $1"

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete webhook endpoint: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("%w: %w", webhook.ErrNotFound, sql.ErrNoRows)
	}

	return nil
}

// ListActiveByContextAndEvent retrieves active endpoints for a context, filtering by event.
// Note: SQL fetches all active endpoints; filtering by event is done in Go since events
// are stored as comma-separated TEXT.
func (r *webhookRepo) ListActiveByContextAndEvent(contextID, event string) ([]*webhook.WebhookEndpoint, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, name, url, events, secret_hash, signing_secret_enc, active, created_at, updated_at
		FROM webhook_endpoints
		WHERE context_id = $1 AND active = true
		ORDER BY created_at ASC
	`

	rows, err := r.pool.Query(ctx, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("list active webhook endpoints: %w", err)
	}
	defer rows.Close()

	endpoints := make([]*webhook.WebhookEndpoint, 0)
	for rows.Next() {
		e := &webhook.WebhookEndpoint{}
		var eventsStr string

		err := rows.Scan(
			&e.ID, &e.ContextID, &e.Name, &e.URL, &eventsStr,
			&e.SecretHash, &e.SigningSecret, &e.Active, &e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan webhook endpoint: %w", err)
		}

		if err := decryptSigningSecret(e); err != nil {
			return nil, fmt.Errorf("decrypt webhook endpoint secret: %w", err)
		}

		e.Events = webhook.ParseEvents(eventsStr)
		// Filter by event match in Go
		if e.HasEvent(event) {
			endpoints = append(endpoints, e)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate webhook endpoints: %w", err)
	}

	return endpoints, nil
}
