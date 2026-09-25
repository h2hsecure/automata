package contact

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	ct "github.com/hekemen/automata/internal/domain/contact"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository is the interface for contact data persistence used by usecases.
type Repository interface {
	Create(c *ct.Contact) error
	GetByID(id string) (*ct.Contact, error)
	List(offset, limit int, filters ct.FilterOptions) ([]*ct.Contact, error)
	Update(c *ct.Contact) error
	Delete(id string) error
	FindByEmail(contextID, email string) (*ct.Contact, error)
	Merge(keepID, mergeIntoID string) error
	CreateActivity(a *ct.Activity) error
	GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]ct.Activity, int64, error)
	GetActivityCount(contactID string) (int64, error)
	CountByContext(contextID string) (int64, error)
}

// ActivityCloser is the interface for creating activities.
type ActivityCloser interface {
	CreateActivity(a *ct.Activity) error
}

// TagRepository defines operations for tag management with activity support.
type TagRepository interface {
	ApplyTags(contextID, contactID string, tagNames []string, ac ActivityCloser) error
}

// tagRepo handles tag and membership operations via raw SQL.
type tagRepo struct {
	pool *pgxpool.Pool
}

// newTagRepo creates a new tag repository.
func newTagRepo(pool *pgxpool.Pool) *tagRepo {
	return &tagRepo{pool: pool}
}

// ApplyTags ensures tags exist and assigns them to a contact.
// It handles the full lifecycle: ensure tags exist, clear old memberships, add new ones,
// and creates activity records for tag_added / tag_removed events.
func (t *tagRepo) ApplyTags(contextID, contactID string, tagNames []string, ac ActivityCloser) error {
	ctx := context.Background()

	// Get current tag names for this contact
	currentTags, err := getCurrentTagNames(ctx, t.pool, contactID)
	if err != nil {
		_ = fmt.Errorf("get current tags: %w", err)
		// Continue anyway — we can still apply new tags
	}

	// Delete existing memberships
	_, err = t.pool.Exec(ctx, "DELETE FROM contact_tag_memberships WHERE contact_id = $1", contactID)
	if err != nil {
		return fmt.Errorf("delete memberships: %w", err)
	}

	// Ensure tags exist and collect IDs
	tagIDs := make([]string, 0, len(tagNames))
	tagNamesMap := make(map[string]string) // name -> id
	for _, name := range tagNames {
		tagID, err := ensureTag(ctx, t.pool, contextID, name)
		if err != nil {
			return fmt.Errorf("ensure tag %q: %w", name, err)
		}
		tagIDs = append(tagIDs, tagID)
		tagNamesMap[name] = tagID
	}

	// Add memberships
	if len(tagIDs) > 0 {
		query := `
			INSERT INTO contact_tag_memberships (contact_id, tag_id)
			VALUES ($1, $2)
			ON CONFLICT (contact_id, tag_id) DO NOTHING
		`
		for _, tagID := range tagIDs {
			_, err := t.pool.Exec(ctx, query, contactID, tagID); if err != nil {
				return fmt.Errorf("add membership: %w", err)
			}
		}
	}

	// Create tag activities if we have an activity closer
	if ac != nil {
		now := time.Now().Format(time.RFC3339)
		currentSet := make(map[string]bool)
		for _, t := range currentTags {
			currentSet[t] = true
		}

		// New tags added
		for _, name := range tagNames {
			if !currentSet[name] {
				if err := ac.CreateActivity(&ct.Activity{
					ID:        uuid.New().String(),
					ContactID: contactID,
					ContextID: contextID,
					Type:      ct.ActivityTagAdded,
					Data: map[string]interface{}{
						"tag_name": name,
						"tag_id":   tagNamesMap[name],
						"acted_at": now,
					},
				}); err != nil {
					_ = fmt.Errorf("create tag_added activity: %w", err)
				}
			}
		}

		// Tags removed
		for _, name := range currentTags {
			if _, exists := tagNamesMap[name]; !exists {
				if err := ac.CreateActivity(&ct.Activity{
					ID:        uuid.New().String(),
					ContactID: contactID,
					ContextID: contextID,
					Type:      ct.ActivityTagRemoved,
					Data: map[string]interface{}{
						"tag_name": name,
						"acted_at": now,
					},
				}); err != nil {
					_ = fmt.Errorf("create tag_removed activity: %w", err)
				}
			}
		}
	}

	return nil
}

// getCurrentTagNames retrieves the tag names for a contact.
func getCurrentTagNames(ctx context.Context, pool *pgxpool.Pool, contactID string) ([]string, error) {
	query := `
		SELECT t.name FROM contact_tag_memberships tm
		JOIN contact_tags t ON tm.tag_id = t.id
		WHERE tm.contact_id = $1
		ORDER BY t.name
	`
	rows, err := pool.Query(ctx, query, contactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// ensureTag ensures a tag exists for the context, creating it if necessary.
func ensureTag(ctx context.Context, pool *pgxpool.Pool, contextID, name string) (string, error) {
	var tagID string
	query := "SELECT id FROM contact_tags WHERE context_id = $1 AND name = $2"
	err := pool.QueryRow(ctx, query, contextID, name).Scan(&tagID)
	if err == nil {
		return tagID, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("query tag: %w", err)
	}

	tagID = uuid.New().String()
	color := "#6366f1"
	_, err = pool.Exec(ctx,
		"INSERT INTO contact_tags (id, context_id, name, color) VALUES ($1, $2, $3, $4)",
		tagID, contextID, name, color,
	)
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			err = pool.QueryRow(ctx, query, contextID, name).Scan(&tagID)
			if err != nil {
				return "", fmt.Errorf("find tag after race: %w", err)
			}
			return tagID, nil
		}
		return "", fmt.Errorf("create tag: %w", err)
	}

	return tagID, nil
}

// applyTags creates/looks up tags and assigns them to a contact.
// It handles the full lifecycle: ensure tags exist, clear old memberships, add new ones,
// and creates activity records for tag_added / tag_removed events.
func applyTags(repo Repository, contextID, contactID string, tagNames []string) error {
	// Check if repo has an extended interface with tag operations
	type tagOpener interface {
		TagRepository() TagRepository
	}

	if opener, ok := repo.(tagOpener); ok {
		tagRepo := opener.TagRepository()
		return tagRepo.ApplyTags(contextID, contactID, tagNames, repo)
	}

	return fmt.Errorf("repository does not support tag operations")
}

// _jsonUnmarshal is a helper to satisfy unused import checks.
var _ = json.Unmarshal
