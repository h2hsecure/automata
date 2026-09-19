package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type configRepo struct {
	pool *pgxpool.Pool
}

// NewConfigRepo creates a new PostgreSQL admin config repository.
func NewConfigRepo(pool *pgxpool.Pool) config.ConfigRepository {
	return &configRepo{pool: pool}
}

func (r *configRepo) GetByTenant(tenantID string) (map[config.ConfigKey]map[string]interface{}, error) {
	ctx := context.Background()
	query := `SELECT key, value FROM admin_configs WHERE tenant_id = $1 ORDER BY key`

	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get configs by tenant: %w", err)
	}
	defer rows.Close()

	result := make(map[config.ConfigKey]map[string]interface{})
	for rows.Next() {
		var key string
		var valueJSON []byte
		if err := rows.Scan(&key, &valueJSON); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		var value map[string]interface{}
		if err := json.Unmarshal(valueJSON, &value); err != nil {
			return nil, fmt.Errorf("unmarshal config value: %w", err)
		}
		result[config.ConfigKey(key)] = value
	}
	return result, nil
}

func (r *configRepo) GetByKey(tenantID string, key config.ConfigKey) (map[string]interface{}, error) {
	ctx := context.Background()
	query := `SELECT value FROM admin_configs WHERE tenant_id = $1 AND key = $2`

	var valueJSON []byte
	err := r.pool.QueryRow(ctx, query, tenantID, string(key)).Scan(&valueJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get config by key: %w", err)
	}

	var value map[string]interface{}
	if err := json.Unmarshal(valueJSON, &value); err != nil {
		return nil, fmt.Errorf("unmarshal config value: %w", err)
	}
	return value, nil
}

func (r *configRepo) Upsert(tenantID string, key config.ConfigKey, value map[string]interface{}) error {
	ctx := context.Background()
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal config value: %w", err)
	}

	query := `
		INSERT INTO admin_configs (id, tenant_id, key, value, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (tenant_id, key) DO UPDATE
			SET value = $4, updated_at = NOW()
		RETURNING id, tenant_id, key, value, created_at, updated_at
	`

	var id, storedKey string
	var storedValueJSON, createdAt, updatedAt []byte
	err = r.pool.QueryRow(ctx, query,
		uuid.New().String(), tenantID, string(key), valueJSON,
	).Scan(&id, &storedKey, &storedKey, &storedValueJSON, &createdAt, &updatedAt)
	if err != nil {
		return fmt.Errorf("upsert config: %w", err)
	}
	return nil
}

func (r *configRepo) Delete(tenantID string, key config.ConfigKey) error {
	ctx := context.Background()
	query := `DELETE FROM admin_configs WHERE tenant_id = $1 AND key = $2`

	result, err := r.pool.Exec(ctx, query, tenantID, string(key))
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("config not found")
	}
	return nil
}
