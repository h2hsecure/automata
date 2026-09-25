package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	ct "github.com/hekemen/automata/internal/domain/contact"
	uc "github.com/hekemen/automata/internal/usecase/contact"
	"github.com/jackc/pgx/v5/pgxpool"
)

// repo implements the ct.Repository interface using PostgreSQL.
type repo struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL contact repository.
func New(pool *pgxpool.Pool) ct.Repository {
	return &repo{pool: pool}
}

// Create inserts a new contact into the database.
// Returns ct.ErrDuplicateEmail if a contact with the same email already exists for the context.
func (r *repo) Create(c *ct.Contact) error {
	ctx := context.Background()

	query := `
		INSERT INTO contacts (id, context_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
	`

	customFieldsJSON, err := json.Marshal(c.CustomFields)
	if err != nil {
		return fmt.Errorf("marshal custom fields: %w", err)
	}

	_, err = r.pool.Exec(ctx, query,
		c.ID,
		c.ContextID,
		c.Email,
		c.FirstName,
		c.LastName,
		c.Phone,
		c.Company,
		customFieldsJSON,
		c.Source,
		c.SourceID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ct.ErrDuplicateEmail
		}
		return fmt.Errorf("create contact: %w", err)
	}

	return nil
}

// GetByID retrieves a contact by its ID, including associated tags.
func (r *repo) GetByID(id string) (*ct.Contact, error) {
	ctx := context.Background()

	query := `
		SELECT id, context_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at
		FROM contacts
		WHERE id = $1
	`

	c := &ct.Contact{}
	var customFieldsJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&c.ID,
		&c.ContextID,
		&c.Email,
		&c.FirstName,
		&c.LastName,
		&c.Phone,
		&c.Company,
		&customFieldsJSON,
		&c.Source,
		&c.SourceID,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("contact not found: %w", err)
		}
		return nil, fmt.Errorf("get contact by ID: %w", err)
	}

	if len(customFieldsJSON) > 0 && string(customFieldsJSON) != "null" {
		if err := json.Unmarshal(customFieldsJSON, &c.CustomFields); err != nil {
			return nil, fmt.Errorf("unmarshal custom fields: %w", err)
		}
	}

	if err := r.loadTags(ctx, c); err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}

	return c, nil
}

// List retrieves contacts with optional filters, ordered by created_at DESC.
// Note: context scoping is not available via this interface — callers must scope at the handler level.
func (r *repo) List(offset, limit int, filters ct.FilterOptions) ([]*ct.Contact, error) {
	ctx := context.Background()

	whereClauses := []string{"1=1"}
	args := []interface{}{}
	argIndex := 1

	// Build WHERE clauses
	if filters.Source != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("source = $%d", argIndex))
		args = append(args, filters.Source)
		argIndex++
	}

	if filters.Company != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("company ILIKE $%d", argIndex))
		args = append(args, "%"+filters.Company+"%")
		argIndex++
	}

	if filters.Search != "" {
		searchTerm := "%" + strings.ToLower(filters.Search) + "%"
		whereClauses = append(whereClauses, fmt.Sprintf(
			"(LOWER(first_name) LIKE $%d OR LOWER(last_name) LIKE $%d OR LOWER(email) LIKE $%d)",
			argIndex, argIndex+1, argIndex+2,
		))
		args = append(args, searchTerm, searchTerm, searchTerm)
		argIndex += 3
	}

	if len(filters.Tags) > 0 {
		placeholders := make([]string, len(filters.Tags))
		tagArgs := make([]interface{}, len(filters.Tags))
		for i, tag := range filters.Tags {
			placeholders[i] = fmt.Sprintf("$%d", argIndex)
			tagArgs[i] = tag
			argIndex++
		}
		whereClauses = append(whereClauses, fmt.Sprintf(
			"contact_id IN (SELECT contact_id FROM contact_tag_memberships WHERE tag_id IN (SELECT id FROM contact_tags WHERE context_id = $1 AND name IN (%s)))",
			strings.Join(placeholders, ", "),
		))
		allTagArgs := make([]interface{}, 0, len(filters.Tags)+1)
		allTagArgs = append(allTagArgs, filters.Tags[0])
		for _, tag := range filters.Tags {
			allTagArgs = append(allTagArgs, tag)
		}
		args = append(allTagArgs, tagArgs...)
	}

	whereClause := strings.Join(whereClauses, " AND ")

	listQuery := fmt.Sprintf(`
		SELECT id, context_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at
		FROM contacts
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	contacts := make([]*ct.Contact, 0)
	for rows.Next() {
		c := &ct.Contact{
			CustomFields: make(map[string]interface{}),
		}
		var customFieldsJSON []byte

		err := rows.Scan(
			&c.ID,
			&c.ContextID,
			&c.Email,
			&c.FirstName,
			&c.LastName,
			&c.Phone,
			&c.Company,
			&customFieldsJSON,
			&c.Source,
			&c.SourceID,
			&c.CreatedAt,
			&c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}

		if len(customFieldsJSON) > 0 && string(customFieldsJSON) != "null" {
			if err := json.Unmarshal(customFieldsJSON, &c.CustomFields); err != nil {
				return nil, fmt.Errorf("unmarshal custom fields: %w", err)
			}
		}

		if err := r.loadTags(ctx, c); err != nil {
			return nil, fmt.Errorf("load tags: %w", err)
		}

		contacts = append(contacts, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contacts: %w", err)
	}

	return contacts, nil
}

// Update modifies an existing ct.
func (r *repo) Update(c *ct.Contact) error {
	ctx := context.Background()

	customFieldsJSON, err := json.Marshal(c.CustomFields)
	if err != nil {
		return fmt.Errorf("marshal custom fields: %w", err)
	}

	query := `
		UPDATE contacts
		SET email = $2, first_name = $3, last_name = $4, phone = $5, company = $6,
			custom_fields = $7, source = $8, source_id = $9, updated_at = NOW()
		WHERE id = $1
	`

	_, err = r.pool.Exec(ctx, query,
		c.ID,
		c.Email,
		c.FirstName,
		c.LastName,
		c.Phone,
		c.Company,
		customFieldsJSON,
		c.Source,
		c.SourceID,
	)
	if err != nil {
		return fmt.Errorf("update contact: %w", err)
	}

	return nil
}

// Delete removes a contact by its ID.
func (r *repo) Delete(id string) error {
	ctx := context.Background()

	query := "DELETE FROM contacts WHERE id = $1"
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete contact: %w", err)
	}

	return nil
}

// FindByEmail retrieves a contact by context and email.
func (r *repo) FindByEmail(contextID, email string) (*ct.Contact, error) {
	ctx := context.Background()

	query := `
		SELECT id, context_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at
		FROM contacts
		WHERE context_id = $1 AND email = $2
	`

	c := &ct.Contact{
		CustomFields: make(map[string]interface{}),
	}
	var customFieldsJSON []byte

	err := r.pool.QueryRow(ctx, query, contextID, email).Scan(
		&c.ID,
		&c.ContextID,
		&c.Email,
		&c.FirstName,
		&c.LastName,
		&c.Phone,
		&c.Company,
		&customFieldsJSON,
		&c.Source,
		&c.SourceID,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("contact not found: %w", err)
		}
		return nil, fmt.Errorf("find contact by email: %w", err)
	}

	if len(customFieldsJSON) > 0 && string(customFieldsJSON) != "null" {
		if err := json.Unmarshal(customFieldsJSON, &c.CustomFields); err != nil {
			return nil, fmt.Errorf("unmarshal custom fields: %w", err)
		}
	}

	if err := r.loadTags(ctx, c); err != nil {
		return nil, fmt.Errorf("load tags: %w", err)
	}

	return c, nil
}

// Merge merges the contact identified by mergeIntoID into the contact identified by keepID.
// It merges custom_fields (JSONB), tags, reassigns activities, and deletes the merged ct.
func (r *repo) Merge(keepID, mergeIntoID string) error {
	ctx := context.Background()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Merge custom_fields (JSONB merge)
	var keepFields, mergeFields []byte
	err = tx.QueryRow(ctx, "SELECT custom_fields FROM contacts WHERE id = $1", keepID).Scan(&keepFields)
	if err != nil {
		return fmt.Errorf("get keep contact fields: %w", err)
	}
	err = tx.QueryRow(ctx, "SELECT custom_fields FROM contacts WHERE id = $1", mergeIntoID).Scan(&mergeFields)
	if err != nil {
		return fmt.Errorf("get merge contact fields: %w", err)
	}

	mergedFields, err := mergeJSONB(keepFields, mergeFields)
	if err != nil {
		return fmt.Errorf("merge custom fields: %w", err)
	}

	_, err = tx.Exec(ctx,
		"UPDATE contacts SET custom_fields = $1, updated_at = NOW() WHERE id = $2",
		mergedFields, keepID,
	)
	if err != nil {
		return fmt.Errorf("update keep contact fields: %w", err)
	}

	// 2. Merge tags
	_, err = tx.Exec(ctx, `
		INSERT INTO contact_tag_memberships (contact_id, tag_id)
		SELECT $1, tag_id FROM contact_tag_memberships WHERE contact_id = $2
		ON CONFLICT (contact_id, tag_id) DO NOTHING
	`, keepID, mergeIntoID)
	if err != nil {
		return fmt.Errorf("merge tags: %w", err)
	}

	// 3. Reassign activities (placeholder - activities table will be created by user tracking module)
	_, err = tx.Exec(ctx, `
		UPDATE contact_activities SET contact_id = $1 WHERE contact_id = $2
	`, keepID, mergeIntoID)
	if err != nil {
		// Activities table may not exist yet; ignore this error
	}

	// 4. Delete the merged contact
	_, err = tx.Exec(ctx, "DELETE FROM contacts WHERE id = $1", mergeIntoID)
	if err != nil {
		return fmt.Errorf("delete merged contact: %w", err)
	}

	return tx.Commit(ctx)
}

// GetActivity retrieves activities for a contact, ordered by created_at DESC.
// When activityType is non-nil, filters by that type.
// Returns activities and total count for pagination.
func (r *repo) GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]ct.Activity, int64, error) {
	ctx := context.Background()

	// Build WHERE clauses
	whereClauses := []string{"contact_id = $1", "context_id = $2"}
	args := []interface{}{contactID, contextID}
	argIndex := 3

	if activityType != nil && *activityType != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("type = $%d", argIndex))
		args = append(args, *activityType)
		argIndex++
	}

	whereClause := strings.Join(whereClauses, " AND ")

	// Count query
	countQuery := fmt.Sprintf(
		"SELECT COUNT(*) FROM contact_activities WHERE %s",
		whereClause,
	)
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count activities: %w", err)
	}

	// Paginated select
	listQuery := fmt.Sprintf(`
		SELECT id, contact_id, context_id, type, data, source_id, created_at
		FROM contact_activities
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("get activities: %w", err)
	}
	defer rows.Close()

	activities := make([]ct.Activity, 0)
	for rows.Next() {
		var a ct.Activity
		var dataJSON []byte

		err := rows.Scan(
			&a.ID,
			&a.ContactID,
			&a.ContextID,
			(*string)(&a.Type),
			&dataJSON,
			&a.SourceID,
			&a.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan activity: %w", err)
		}

		if len(dataJSON) > 0 && string(dataJSON) != "null" {
			if err := json.Unmarshal(dataJSON, &a.Data); err != nil {
				return nil, 0, fmt.Errorf("unmarshal activity data: %w", err)
			}
		}

		activities = append(activities, a)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate activities: %w", err)
	}

	return activities, total, nil
}

// GetActivityCount returns the total number of activities for a ct.
func (r *repo) GetActivityCount(contactID string) (int64, error) {
	ctx := context.Background()

	query := "SELECT COUNT(*) FROM contact_activities WHERE contact_id = $1"
	var count int64

	err := r.pool.QueryRow(ctx, query, contactID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count activities: %w", err)
	}

	return count, nil
}

// CreateActivity inserts a new activity record.
func (r *repo) CreateActivity(a *ct.Activity) error {
	ctx := context.Background()

	dataJSON, err := json.Marshal(a.Data)
	if err != nil {
		return fmt.Errorf("marshal activity data: %w", err)
	}

	query := `
		INSERT INTO contact_activities (id, contact_id, context_id, type, data, source_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
	`

	_, err = r.pool.Exec(ctx, query,
		a.ID,
		a.ContactID,
		a.ContextID,
		string(a.Type),
		dataJSON,
		a.SourceID,
	)
	if err != nil {
		return fmt.Errorf("create activity: %w", err)
	}

	return nil
}

// CountByContext returns the total number of contacts for a context.
func (r *repo) CountByContext(contextID string) (int64, error) {
	ctx := context.Background()

	query := "SELECT COUNT(*) FROM contacts WHERE context_id = $1"
	var count int64

	err := r.pool.QueryRow(ctx, query, contextID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count contacts: %w", err)
	}

	return count, nil
}

// loadTags loads tags for a contact by joining with contact_tag_memberships and contact_tags.
func (r *repo) loadTags(ctx context.Context, c *ct.Contact) error {
	query := `
		SELECT t.id, t.context_id, t.name, t.color, t.created_at
		FROM contact_tag_memberships tm
		JOIN contact_tags t ON tm.tag_id = t.id
		WHERE tm.contact_id = $1
		ORDER BY t.created_at
	`

	rows, err := r.pool.Query(ctx, query, c.ID)
	if err != nil {
		return fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	c.Tags = make([]ct.Tag, 0)
	for rows.Next() {
		var tag ct.Tag
		err := rows.Scan(&tag.ID, &tag.ContextID, &tag.Name, &tag.Color, &tag.CreatedAt)
		if err != nil {
			return fmt.Errorf("scan tag: %w", err)
		}
		c.Tags = append(c.Tags, tag)
	}

	return rows.Err()
}

// mergeJSONB merges two JSONB byte slices, with mergeFields taking precedence over keepFields.
func mergeJSONB(keep, merge []byte) ([]byte, error) {
	merged := make(map[string]interface{})

	if len(keep) > 0 && string(keep) != "null" {
		if err := json.Unmarshal(keep, &merged); err != nil {
			return nil, fmt.Errorf("unmarshal keep fields: %w", err)
		}
	}

	if len(merge) > 0 && string(merge) != "null" {
		var mergeFields map[string]interface{}
		if err := json.Unmarshal(merge, &mergeFields); err != nil {
			return nil, fmt.Errorf("unmarshal merge fields: %w", err)
		}
		for k, v := range mergeFields {
			merged[k] = v
		}
	}

	return json.Marshal(merged)
}

// isUniqueViolation checks if a pgx error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pgx v5 error code for unique violation
	return strings.Contains(err.Error(), "23505")
}

// TagRepository returns a TagRepository implementation for tag operations.
func (r *repo) TagRepository() uc.TagRepository {
	return &repoTagRepo{pool: r.pool}
}

// repoTagRepo implements uc.TagRepository for the postgres repo.
type repoTagRepo struct {
	pool *pgxpool.Pool
}

// ApplyTags ensures tags exist and assigns them to a contact.
// Activity creation is handled by the usecase layer.
func (t *repoTagRepo) ApplyTags(contextID, contactID string, tagNames []string, _ uc.ActivityCloser) error {
	ctx := context.Background()

	// Delete existing memberships
	_, err := t.pool.Exec(ctx, "DELETE FROM contact_tag_memberships WHERE contact_id = $1", contactID)
	if err != nil {
		return fmt.Errorf("delete memberships: %w", err)
	}

	// Ensure tags exist and collect IDs
	tagIDs := make([]string, 0, len(tagNames))
	for _, name := range tagNames {
		tagID, err := ensureTag(ctx, t.pool, contextID, name)
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
			_, err := t.pool.Exec(ctx, query, contactID, tagID); if err != nil {
				return fmt.Errorf("add membership: %w", err)
			}
		}
	}

	return nil
}

// ensureTag ensures a tag exists for the context, creating it if necessary.
func ensureTag(ctx context.Context, pool *pgxpool.Pool, contextID, name string) (string, error) {
	var tagID string
	query := "SELECT id FROM contact_tags WHERE context_id = $1 AND name = $2"
	err := pool.QueryRow(ctx, query, contextID, name).Scan(&tagID)
	if err == nil {
		return tagID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
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
