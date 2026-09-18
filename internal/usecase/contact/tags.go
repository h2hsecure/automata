package contact

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/contact"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the interface for contact data persistence used by usecases.
type Repository interface {
	Create(c *contact.Contact) error
	GetByID(id string) (*contact.Contact, error)
	List(offset, limit int, filters contact.FilterOptions) ([]*contact.Contact, error)
	Update(c *contact.Contact) error
	Delete(id string) error
	FindByEmail(tenantID, email string) (*contact.Contact, error)
	Merge(keepID, mergeIntoID string) error
	GetActivity(contactID string, offset, limit int) ([]contact.Activity, error)
	CountByTenant(tenantID string) (int64, error)
}

// tagRepo handles tag and membership operations via raw SQL.
type tagRepo struct {
	pool *pgxpool.Pool
}

// newTagRepo creates a new tag repository.
func newTagRepo(pool *pgxpool.Pool) *tagRepo {
	return &tagRepo{pool: pool}
}

// ensureOrCreateTag ensures a tag exists for the tenant, creating it if necessary.
// Returns the tag ID.
func (t *tagRepo) ensureOrCreateTag(ctx context.Context, tenantID, name string) (string, error) {
	// Try to find existing tag
	var tagID string
	query := "SELECT id FROM contact_tags WHERE tenant_id = $1 AND name = $2"
	err := t.pool.QueryRow(ctx, query, tenantID, name).Scan(&tagID)
	if err == nil {
		return tagID, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("query tag: %w", err)
	}

	// Create new tag
	tagID = uuid.New().String()
	color := "#6366f1"
	_, err = t.pool.Exec(ctx,
		"INSERT INTO contact_tags (id, tenant_id, name, color) VALUES ($1, $2, $3, $4)",
		tagID, tenantID, name, color,
	)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			// Race condition: another goroutine created it, try to find again
			err = t.pool.QueryRow(ctx, query, tenantID, name).Scan(&tagID)
			if err != nil {
				return "", fmt.Errorf("find tag after race: %w", err)
			}
			return tagID, nil
		}
		return "", fmt.Errorf("create tag: %w", err)
	}

	return tagID, nil
}

// addMemberships adds tag memberships for a contact, ignoring duplicates.
func (t *tagRepo) addMemberships(ctx context.Context, contactID string, tagIDs []string) error {
	if len(tagIDs) == 0 {
		return nil
	}

	query := `
		INSERT INTO contact_tag_memberships (contact_id, tag_id)
		VALUES ($1, $2)
		ON CONFLICT (contact_id, tag_id) DO NOTHING
	`

	for _, tagID := range tagIDs {
		_, err := t.pool.Exec(ctx, query, contactID, tagID)
		if err != nil {
			return fmt.Errorf("add membership: %w", err)
		}
	}

	return nil
}

// deleteMemberships deletes all tag memberships for a contact.
func (t *tagRepo) deleteMemberships(ctx context.Context, contactID string) error {
	query := "DELETE FROM contact_tag_memberships WHERE contact_id = $1"
	_, err := t.pool.Exec(ctx, query, contactID)
	if err != nil {
		return fmt.Errorf("delete memberships: %w", err)
	}
	return nil
}

// applyTags creates/looks up tags and assigns them to a contact.
// It handles the full lifecycle: ensure tags exist, clear old memberships, add new ones.
func applyTags(repo Repository, tenantID, contactID string, tagNames []string) error {
	// We need the pool to do tag operations; the repo interface doesn't expose it.
	// Since usecases can't directly access the pool, we'll use a different approach:
	// The Repository interface needs to be extended, or we use a type assertion.
	// For now, we'll handle this by having the postgres repo implement an extended interface.

	// Check if repo has an extended interface with tag operations
	type tagOpener interface {
		TagRepository() TagRepository
	}

	if opener, ok := repo.(tagOpener); ok {
		tagRepo := opener.TagRepository()
		return tagRepo.ApplyTags(tenantID, contactID, tagNames)
	}

	return fmt.Errorf("repository does not support tag operations")
}

// TagRepository defines operations for tag management.
type TagRepository interface {
	ApplyTags(tenantID, contactID string, tagNames []string) error
}

// poolRepo wraps a pgxpool.Pool to provide tag operations.
type poolRepo struct {
	pool *pgxpool.Pool
}

// ApplyTags ensures tags exist and assigns them to a contact.
func (p *poolRepo) ApplyTags(tenantID, contactID string, tagNames []string) error {
	ctx := context.Background()

	// Delete existing memberships
	_, err := p.pool.Exec(ctx, "DELETE FROM contact_tag_memberships WHERE contact_id = $1", contactID)
	if err != nil {
		return fmt.Errorf("delete memberships: %w", err)
	}

	// Ensure tags exist and collect IDs
	tagIDs := make([]string, 0, len(tagNames))
	for _, name := range tagNames {
		tagID, err := ensureTag(ctx, p.pool, tenantID, name)
		if err != nil {
			return fmt.Errorf("ensure tag %q: %w", name, err)
		}
		tagIDs = append(tagIDs, tagID)
	}

	// Add memberships
	if len(tagIDs) > 0 {
		query := `
			INSERT INTO contact_tag_memberships (contact_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT (contact_id, tag_id) DO NOTHING
		`
		for _, tagID := range tagIDs {
			_, err := p.pool.Exec(ctx, query, contactID, tagID); if err != nil {
				return fmt.Errorf("add membership: %w", err)
			}
		}
	}

	return nil
}

// ensureTag ensures a tag exists for the tenant, creating it if necessary.
func ensureTag(ctx context.Context, pool *pgxpool.Pool, tenantID, name string) (string, error) {
	var tagID string
	query := "SELECT id FROM contact_tags WHERE tenant_id = $1 AND name = $2"
	err := pool.QueryRow(ctx, query, tenantID, name).Scan(&tagID)
	if err == nil {
		return tagID, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("query tag: %w", err)
	}

	tagID = uuid.New().String()
	color := "#6366f1"
	_, err = pool.Exec(ctx,
		"INSERT INTO contact_tags (id, tenant_id, name, color) VALUES ($1, $2, $3, $4)",
		tagID, tenantID, name, color,
	)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			err = pool.QueryRow(ctx, query, tenantID, name).Scan(&tagID)
			if err != nil {
				return "", fmt.Errorf("find tag after race: %w", err)
			}
			return tagID, nil
		}
		return "", fmt.Errorf("create tag: %w", err)
	}

	return tagID, nil
}

// _jsonUnmarshal is a helper to satisfy unused import checks.
var _ = json.Unmarshal
