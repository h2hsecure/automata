package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type apiKeyPostgresRepo struct {
	pool *pgxpool.Pool
}

// NewApiKeyPostgresRepo creates a new PostgreSQL API key repository.
func NewApiKeyPostgresRepo(pool *pgxpool.Pool) ApiKeyRepository {
	return &apiKeyPostgresRepo{pool: pool}
}

// ApiKeyRepository defines the API key database operations.
type ApiKeyRepository interface {
	Create(key *auth.APIKey) error
	ListByContext(contextID string) ([]*auth.APIKey, error)
	Delete(id string) error
	GetByHash(hash string) (*auth.APIKey, error)
	GetByPrefix(prefix string, contextID string) (*auth.APIKey, error)
}

func (r *apiKeyPostgresRepo) Create(key *auth.APIKey) error {
	if key.ID == "" {
		key.ID = uuid.New().String()
	}

	query := `
		INSERT INTO api_keys (id, context_id, user_id, key_hash, name, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id, created_at
	`

	err := r.pool.QueryRow(context.Background(), query,
		key.ID, key.ContextID, key.UserID, key.KeyHash, key.Name, key.ExpiresAt,
	).Scan(&key.ID, &key.CreatedAt)

	if err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return nil
}

func (r *apiKeyPostgresRepo) GetByHash(hash string) (*auth.APIKey, error) {
	key := &auth.APIKey{}

	query := `
		SELECT id, context_id, user_id, key_hash, name, expires_at, created_at
		FROM api_keys WHERE key_hash = $1
	`

	err := r.pool.QueryRow(context.Background(), query, hash).Scan(
		&key.ID, &key.ContextID, &key.UserID, &key.KeyHash,
		&key.Name, &key.ExpiresAt, &key.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}

	return key, nil
}

func (r *apiKeyPostgresRepo) GetByPrefix(prefix, contextID string) (*auth.APIKey, error) {
	key := &auth.APIKey{}

	query := `
		SELECT id, context_id, user_id, key_hash, name, expires_at, created_at
		FROM api_keys WHERE context_id = $1 AND key_hash LIKE $2
		LIMIT 1
	`

	err := r.pool.QueryRow(context.Background(), query, contextID, prefix+"%").Scan(
		&key.ID, &key.ContextID, &key.UserID, &key.KeyHash,
		&key.Name, &key.ExpiresAt, &key.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get api key by prefix: %w", err)
	}

	return key, nil
}

func (r *apiKeyPostgresRepo) ListByContext(contextID string) ([]*auth.APIKey, error) {
	query := `
		SELECT id, context_id, user_id, key_hash, name, expires_at, created_at
		FROM api_keys WHERE context_id = $1 ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(context.Background(), query, contextID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []*auth.APIKey
	for rows.Next() {
		key := &auth.APIKey{}
		err := rows.Scan(
			&key.ID, &key.ContextID, &key.UserID, &key.KeyHash,
			&key.Name, &key.ExpiresAt, &key.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func (r *apiKeyPostgresRepo) Delete(id string) error {
	query := `DELETE FROM api_keys WHERE id = $1`
	_, err := r.pool.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	return nil
}
