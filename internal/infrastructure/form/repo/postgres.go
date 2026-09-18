package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/hekemen/automata/internal/domain/form"
	"github.com/jackc/pgx/v5/pgxpool"
)

// repo implements the form.Repository interface using PostgreSQL.
type repo struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL form repository.
func New(pool *pgxpool.Pool) form.Repository {
	return &repo{pool: pool}
}

func (r *repo) Create(f *form.Form) error {
	ctx := context.Background()
	fieldsJSON, err := json.Marshal(f.Fields)
	if err != nil {
		return fmt.Errorf("marshal form fields: %w", err)
	}
	settingsJSON, err := json.Marshal(f.Settings)
	if err != nil {
		return fmt.Errorf("marshal form settings: %w", err)
	}
	query := `
		INSERT INTO forms (id, tenant_id, slug, name, description, fields, settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`
	_, err = r.pool.Exec(ctx, query, f.ID, f.TenantID, f.Slug, f.Name, f.Description, fieldsJSON, settingsJSON)
	if err != nil {
		return fmt.Errorf("create form: %w", err)
	}
	return nil
}

func (r *repo) GetByID(id string) (*form.Form, error) {
	ctx := context.Background()
	query := `
		SELECT id, tenant_id, slug, name, description, fields, settings, created_at, updated_at
		FROM forms
		WHERE id = $1
	`
	f := &form.Form{}
	var fieldsJSON, settingsJSON []byte
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&f.ID, &f.TenantID, &f.Slug, &f.Name, &f.Description,
		&fieldsJSON, &settingsJSON, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("form not found: %w", err)
		}
		return nil, fmt.Errorf("get form by id: %w", err)
	}
	if err := json.Unmarshal(fieldsJSON, &f.Fields); err != nil {
		return nil, fmt.Errorf("unmarshal form fields: %w", err)
	}
	if err := json.Unmarshal(settingsJSON, &f.Settings); err != nil {
		return nil, fmt.Errorf("unmarshal form settings: %w", err)
	}
	return f, nil
}

func (r *repo) GetBySlug(tenantID, slug string) (*form.Form, error) {
	ctx := context.Background()
	query := `
		SELECT id, tenant_id, slug, name, description, fields, settings, created_at, updated_at
		FROM forms
		WHERE tenant_id = $1 AND slug = $2
	`
	f := &form.Form{}
	var fieldsJSON, settingsJSON []byte
	err := r.pool.QueryRow(ctx, query, tenantID, slug).Scan(
		&f.ID, &f.TenantID, &f.Slug, &f.Name, &f.Description,
		&fieldsJSON, &settingsJSON, &f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("form not found: %w", err)
		}
		return nil, fmt.Errorf("get form by slug: %w", err)
	}
	if err := json.Unmarshal(fieldsJSON, &f.Fields); err != nil {
		return nil, fmt.Errorf("unmarshal form fields: %w", err)
	}
	if err := json.Unmarshal(settingsJSON, &f.Settings); err != nil {
		return nil, fmt.Errorf("unmarshal form settings: %w", err)
	}
	return f, nil
}

func (r *repo) List(tenantID string) ([]*form.Form, error) {
	ctx := context.Background()
	query := `
		SELECT id, tenant_id, slug, name, description, fields, settings, created_at, updated_at
		FROM forms
		WHERE tenant_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list forms: %w", err)
	}
	defer rows.Close()

	var forms []*form.Form
	for rows.Next() {
		f := &form.Form{}
		var fieldsJSON, settingsJSON []byte
		err := rows.Scan(
			&f.ID, &f.TenantID, &f.Slug, &f.Name, &f.Description,
			&fieldsJSON, &settingsJSON, &f.CreatedAt, &f.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan form: %w", err)
		}
		if err := json.Unmarshal(fieldsJSON, &f.Fields); err != nil {
			return nil, fmt.Errorf("unmarshal form fields: %w", err)
		}
		if err := json.Unmarshal(settingsJSON, &f.Settings); err != nil {
			return nil, fmt.Errorf("unmarshal form settings: %w", err)
		}
		forms = append(forms, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate forms: %w", err)
	}
	return forms, nil
}

func (r *repo) Update(f *form.Form) error {
	ctx := context.Background()
	fieldsJSON, err := json.Marshal(f.Fields)
	if err != nil {
		return fmt.Errorf("marshal form fields: %w", err)
	}
	settingsJSON, err := json.Marshal(f.Settings)
	if err != nil {
		return fmt.Errorf("marshal form settings: %w", err)
	}
	query := `
		UPDATE forms
		SET slug = $2, name = $3, description = $4, fields = $5, settings = $6, updated_at = NOW()
		WHERE id = $1
	`
	result, err := r.pool.Exec(ctx, query, f.ID, f.Slug, f.Name, f.Description, fieldsJSON, settingsJSON)
	if err != nil {
		return fmt.Errorf("update form: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("form not found: %w", sql.ErrNoRows)
	}
	return nil
}

func (r *repo) Delete(id string) error {
	ctx := context.Background()
	query := `DELETE FROM forms WHERE id = $1`
	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete form: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("form not found: %w", sql.ErrNoRows)
	}
	return nil
}

func (r *repo) CreateSubmission(s *form.Submission) error {
	ctx := context.Background()
	dataJSON, err := json.Marshal(s.Data)
	if err != nil {
		return fmt.Errorf("marshal submission data: %w", err)
	}
	filesJSON, err := json.Marshal(s.Files)
	if err != nil {
		return fmt.Errorf("marshal submission files: %w", err)
	}
	query := `
		INSERT INTO form_submissions (id, form_id, tenant_id, data, files, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
	`
	_, err = r.pool.Exec(ctx, query, s.ID, s.FormID, s.TenantID, dataJSON, filesJSON)
	if err != nil {
		return fmt.Errorf("create submission: %w", err)
	}
	return nil
}

func (r *repo) GetSubmission(id string) (*form.Submission, error) {
	ctx := context.Background()
	query := `
		SELECT id, form_id, tenant_id, data, files, created_at
		FROM form_submissions
		WHERE id = $1
	`
	s := &form.Submission{}
	var dataJSON, filesJSON []byte
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.FormID, &s.TenantID, &dataJSON, &filesJSON, &s.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("submission not found: %w", err)
		}
		return nil, fmt.Errorf("get submission: %w", err)
	}
	if err := json.Unmarshal(dataJSON, &s.Data); err != nil {
		return nil, fmt.Errorf("unmarshal submission data: %w", err)
	}
	if err := json.Unmarshal(filesJSON, &s.Files); err != nil {
		return nil, fmt.Errorf("unmarshal submission files: %w", err)
	}
	return s, nil
}

func (r *repo) ListSubmissions(formID string, opts form.ListSubmissionsOptions) ([]*form.Submission, int64, error) {
	ctx := context.Background()

	var countQuery string
	if opts.Limit > 0 || opts.Offset > 0 {
		countQuery = `SELECT COUNT(*) FROM form_submissions WHERE form_id = $1`
	} else {
		countQuery = `SELECT COUNT(*) FROM form_submissions WHERE form_id = $1`
	}

	var total int64
	err := r.pool.QueryRow(ctx, countQuery, formID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count submissions: %w", err)
	}

	dataQuery := `
		SELECT id, form_id, tenant_id, data, files, created_at
		FROM form_submissions
		WHERE form_id = $1
		ORDER BY created_at DESC
	`
	if opts.Limit > 0 {
		dataQuery += fmt.Sprintf(" LIMIT %d", opts.Limit)
		if opts.Offset > 0 {
			dataQuery += fmt.Sprintf(" OFFSET %d", opts.Offset)
		}
	}

	rows, err := r.pool.Query(ctx, dataQuery, formID)
	if err != nil {
		return nil, 0, fmt.Errorf("list submissions: %w", err)
	}
	defer rows.Close()

	var submissions []*form.Submission
	for rows.Next() {
		s := &form.Submission{}
		var dataJSON, filesJSON []byte
		err := rows.Scan(
			&s.ID, &s.FormID, &s.TenantID, &dataJSON, &filesJSON, &s.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan submission: %w", err)
		}
		if err := json.Unmarshal(dataJSON, &s.Data); err != nil {
			return nil, 0, fmt.Errorf("unmarshal submission data: %w", err)
		}
		if err := json.Unmarshal(filesJSON, &s.Files); err != nil {
			return nil, 0, fmt.Errorf("unmarshal submission files: %w", err)
		}
		submissions = append(submissions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate submissions: %w", err)
	}
	return submissions, total, nil
}
