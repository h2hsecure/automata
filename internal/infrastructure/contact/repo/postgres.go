package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hekemen/automata/internal/domain/contact"
	"github.com/jackc/pgx/v5/pgxpool"
)

// repo implements the contact.Repository interface using PostgreSQL.
type repo struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL contact repository.
func New(pool *pgxpool.Pool) contact.Repository {
	return &repo{pool: pool}
}

// Create inserts a new contact into the database.
// Returns contact.ErrDuplicateEmail if a contact with the same email already exists for the tenant.
func (r *repo) Create(c *contact.Contact) error {
	ctx := context.Background()

	query := `
		INSERT INTO contacts (id, tenant_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
	`

	customFieldsJSON, err := json.Marshal(c.CustomFields)
	if err != nil {
		return fmt.Errorf("marshal custom fields: %w", err)
	}

	_, err = r.pool.Exec(ctx, query,
		c.ID,
		c.TenantID,
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
			return contact.ErrDuplicateEmail
		}
		return fmt.Errorf("create contact: %w", err)
	}

	return nil
}

// GetByID retrieves a contact by its ID, including associated tags.
func (r *repo) GetByID(id string) (*contact.Contact, error) {
	ctx := context.Background()

	query := `
		SELECT id, tenant_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at
		FROM contacts
		WHERE id = $1
	`

	c := &contact.Contact{}
	var customFieldsJSON []byte

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&c.ID,
		&c.TenantID,
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
		if err == sql.ErrNoRows {
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
// Note: tenant scoping is not available via this interface — callers must scope at the handler level.
func (r *repo) List(offset, limit int, filters contact.FilterOptions) ([]*contact.Contact, error) {
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
			"contact_id IN (SELECT contact_id FROM contact_tag_memberships WHERE tag_id IN (SELECT id FROM contact_tags WHERE tenant_id = $1 AND name IN (%s)))",
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
		SELECT id, tenant_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at
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

	contacts := make([]*contact.Contact, 0)
	for rows.Next() {
		c := &contact.Contact{
			CustomFields: make(map[string]interface{}),
		}
		var customFieldsJSON []byte

		err := rows.Scan(
			&c.ID,
			&c.TenantID,
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

// Update modifies an existing contact.
func (r *repo) Update(c *contact.Contact) error {
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

// FindByEmail retrieves a contact by tenant and email.
func (r *repo) FindByEmail(tenantID, email string) (*contact.Contact, error) {
	ctx := context.Background()

	query := `
		SELECT id, tenant_id, email, first_name, last_name, phone, company, custom_fields, source, source_id, created_at, updated_at
		FROM contacts
		WHERE tenant_id = $1 AND email = $2
	`

	c := &contact.Contact{
		CustomFields: make(map[string]interface{}),
	}
	var customFieldsJSON []byte

	err := r.pool.QueryRow(ctx, query, tenantID, email).Scan(
		&c.ID,
		&c.TenantID,
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
		if err == sql.ErrNoRows {
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
// It merges custom_fields (JSONB), tags, reassigns activities, and deletes the merged contact.
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
func (r *repo) GetActivity(contactID string, offset, limit int) ([]contact.Activity, error) {
	ctx := context.Background()

	query := `
		SELECT id, contact_id, tenant_id, type, data, source_id, created_at
		FROM contact_activities
		WHERE contact_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.pool.Query(ctx, query, contactID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("get activities: %w", err)
	}
	defer rows.Close()

	activities := make([]contact.Activity, 0)
	for rows.Next() {
		var a contact.Activity
		var dataJSON []byte

		err := rows.Scan(
			&a.ID,
			&a.ContactID,
			&a.TenantID,
			(*string)(&a.Type),
			&dataJSON,
			&a.SourceID,
			&a.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan activity: %w", err)
		}

		if len(dataJSON) > 0 && string(dataJSON) != "null" {
			if err := json.Unmarshal(dataJSON, &a.Data); err != nil {
				return nil, fmt.Errorf("unmarshal activity data: %w", err)
			}
		}

		activities = append(activities, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate activities: %w", err)
	}

	return activities, nil
}

// CountByTenant returns the total number of contacts for a tenant.
func (r *repo) CountByTenant(tenantID string) (int64, error) {
	ctx := context.Background()

	query := "SELECT COUNT(*) FROM contacts WHERE tenant_id = $1"
	var count int64

	err := r.pool.QueryRow(ctx, query, tenantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count contacts: %w", err)
	}

	return count, nil
}

// loadTags loads tags for a contact by joining with contact_tag_memberships and contact_tags.
func (r *repo) loadTags(ctx context.Context, c *contact.Contact) error {
	query := `
		SELECT t.id, t.tenant_id, t.name, t.color, t.created_at
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

	c.Tags = make([]contact.Tag, 0)
	for rows.Next() {
		var tag contact.Tag
		err := rows.Scan(&tag.ID, &tag.TenantID, &tag.Name, &tag.Color, &tag.CreatedAt)
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
