package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hekemen/automata/internal/domain/email"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// emailRepo implements the email.Repository interface using PostgreSQL.
type emailRepo struct {
	pool *pgxpool.Pool
}

// New creates a new PostgreSQL email template repository.
func New(pool *pgxpool.Pool) email.Repository {
	return &emailRepo{pool: pool}
}

// Create inserts a new email template, using ON CONFLICT DO NOTHING for idempotency.
func (r *emailRepo) Create(t *email.EmailTemplate) error {
	ctx := context.Background()

	t.ID = uuid.New().String()

	query := `
		INSERT INTO email_templates (id, context_id, key, subject, body_text, body_html, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		ON CONFLICT (context_id, key) DO NOTHING
	`

	result, err := r.pool.Exec(ctx, query,
		t.ID, t.ContextID, t.Key, t.Subject, t.BodyText, t.BodyHTML, t.IsDefault,
	)
	if err != nil {
		return fmt.Errorf("create email template: %w", err)
	}

	if result.RowsAffected() == 0 {
		return email.ErrDuplicateKey
	}

	return nil
}

// GetByID retrieves a template by its ID.
func (r *emailRepo) GetByID(id string) (*email.EmailTemplate, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, key, subject, body_text, body_html, is_default, created_at, updated_at
		FROM email_templates
		WHERE id = $1
	`

	t := &email.EmailTemplate{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.ContextID, &t.Key, &t.Subject, &t.BodyText, &t.BodyHTML,
		&t.IsDefault, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("email template not found: %w", err)
		}
		return nil, fmt.Errorf("get email template by id: %w", err)
	}

	return t, nil
}

// GetByKey retrieves a template by context_id and key.
func (r *emailRepo) GetByKey(contextID, key string) (*email.EmailTemplate, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, key, subject, body_text, body_html, is_default, created_at, updated_at
		FROM email_templates
		WHERE context_id = $1 AND key = $2
	`

	t := &email.EmailTemplate{}
	err := r.pool.QueryRow(ctx, query, contextID, key).Scan(
		&t.ID, &t.ContextID, &t.Key, &t.Subject, &t.BodyText, &t.BodyHTML,
		&t.IsDefault, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("email template not found: %w", err)
		}
		return nil, fmt.Errorf("get email template by key: %w", err)
	}

	return t, nil
}

// List retrieves all templates for a context, ordered by key ASC.
func (r *emailRepo) List(contextID string) ([]*email.EmailTemplate, error) {
	ctx := context.Background()
	query := `
		SELECT id, context_id, key, subject, body_text, body_html, is_default, created_at, updated_at
		FROM email_templates
		WHERE context_id = $1
		ORDER BY key ASC
	`

	rows, err := r.pool.Query(ctx, query, contextID)
	if err != nil {
		return nil, fmt.Errorf("list email templates: %w", err)
	}
	defer rows.Close()

	templates := make([]*email.EmailTemplate, 0)
	for rows.Next() {
		t := &email.EmailTemplate{}
		err := rows.Scan(
			&t.ID, &t.ContextID, &t.Key, &t.Subject, &t.BodyText, &t.BodyHTML,
			&t.IsDefault, &t.CreatedAt, &t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan email template: %w", err)
		}
		templates = append(templates, t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate email templates: %w", err)
	}

	return templates, nil
}

// Update modifies an existing template, only updating non-empty fields.
func (r *emailRepo) Update(t *email.EmailTemplate) error {
	ctx := context.Background()

	// Build dynamic query — only update non-empty fields
	setClauses := []string{}
	args := []interface{}{t.ID}
	argIndex := 2

	if t.Subject != "" {
		setClauses = append(setClauses, fmt.Sprintf("subject = $%d", argIndex))
		args = append(args, t.Subject)
		argIndex++
	}
	if t.BodyText != "" {
		setClauses = append(setClauses, fmt.Sprintf("body_text = $%d", argIndex))
		args = append(args, t.BodyText)
		argIndex++
	}
	if t.BodyHTML != "" {
		setClauses = append(setClauses, fmt.Sprintf("body_html = $%d", argIndex))
		args = append(args, t.BodyHTML)
		argIndex++
	}

	// If nothing to update, just touch updated_at
	setClauses = append(setClauses, "updated_at = NOW()")

	setClause := strings.Join(setClauses, ", ")

	query := fmt.Sprintf("UPDATE email_templates SET %s WHERE id = $1", setClause)

	_, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update email template: %w", err)
	}

	return nil
}

// Delete removes a template by ID.
func (r *emailRepo) Delete(id string) error {
	ctx := context.Background()
	query := "DELETE FROM email_templates WHERE id = $1"

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete email template: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("email template not found: %w", sql.ErrNoRows)
	}

	return nil
}

// SeedDefaults inserts the 5 default templates for a context, using ON CONFLICT DO NOTHING.
func (r *emailRepo) SeedDefaults(contextID string) error {
	ctx := context.Background()

	defaultTemplates := []struct {
		key, subject, bodyText, bodyHTML string
	}{
		{
			key:      "welcome",
			subject:  "Welcome to {{ .ContextName }}",
			bodyText: "Welcome to {{ .ContextName }}!\n\nHi {{ .ContactName }}, welcome aboard!\n\nLogin URL: {{ .LoginURL }}",
			bodyHTML: "<h1>Welcome to {{ .ContextName }}</h1><p>Hi <strong>{{ .ContactName }}</strong>, welcome aboard!</p><p>Login URL: <a href=\"{{ .LoginURL }}\">{{ .LoginURL }}</a></p>",
		},
		{
			key:      "contact_created",
			subject:  "New contact: {{ .ContactName }}",
			bodyText: "A new contact has been created:\nName: {{ .ContactName }}\nEmail: {{ .ContactEmail }}\nSource: {{ .Source }}",
			bodyHTML: "<h2>New contact created</h2><p><strong>{{ .ContactName }}</strong> ({{ .ContactEmail }}) via {{ .Source }}</p>",
		},
		{
			key:      "form_submission",
			subject:  "New form submission: {{ .FormName }}",
			bodyText: "A new form has been submitted:\nForm: {{ .FormName }}\n{{ .SubmissionData }}",
			bodyHTML: "<h2>New form submission</h2><p><strong>{{ .FormName }}</strong></p><pre>{{ .SubmissionData }}</pre>",
		},
		{
			key:      "password_reset",
			subject:  "Reset your password",
			bodyText: "Hi {{ .ContactName }},\n\nClick the link below to reset your password:\n{{ .ResetURL }}\n\nThis link expires in {{ .Expiry }}.",
			bodyHTML: "<h2>Password Reset</h2><p>Hi <strong>{{ .ContactName }}</strong>,</p><p><a href=\"{{ .ResetURL }}\">Reset your password</a></p><p>This link expires in {{ .Expiry }}.</p>",
		},
		{
			key:      "notification",
			subject:  "{{ .Subject }}",
			bodyText: "{{ .Body }}",
			bodyHTML: "<h3>{{ .Subject }}</h3><p>{{ .Body }}</p>{{ if .CTA }}<p><a href=\"{{ .CTALink }}\">{{ .CTAText }}</a></p>{{ end }}",
		},
	}

	for _, tmpl := range defaultTemplates {
		query := `
			INSERT INTO email_templates (id, context_id, key, subject, body_text, body_html, is_default, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
			ON CONFLICT (context_id, key) DO NOTHING
		`

		_, err := r.pool.Exec(ctx, query,
			uuid.New().String(), contextID, tmpl.key, tmpl.subject, tmpl.bodyText, tmpl.bodyHTML, true,
		)
		if err != nil {
			// Individual seed failures are non-fatal
			log.Warn().Err(err).Str("key", tmpl.key).Msg("failed to seed default email template (may already exist)")
		}
	}

	return nil
}

// SeedDefaultsForContext is a convenience function that can be called from main.go.
func SeedDefaultsForContext(pool *pgxpool.Pool, contextID string) error {
	repo := New(pool)
	return repo.SeedDefaults(contextID)
}
