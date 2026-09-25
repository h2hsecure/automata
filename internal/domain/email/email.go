package email

import (
	"fmt"
	"regexp"
	"time"
)

// EmailTemplate represents a stored email template scoped to a context.
type EmailTemplate struct {
	ID        string
	ContextID string
	Key       string
	Subject   string
	BodyText  string
	BodyHTML  string
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreateTemplateRequest is the request body for creating a template.
type CreateTemplateRequest struct {
	ContextID string `json:"context_id" binding:"required,uuid"`
	Key       string `json:"key" binding:"required,min=1,max=64"`
	Subject   string `json:"subject" binding:"required"`
	BodyText  string `json:"body_text"`
	BodyHTML  string `json:"body_html"`
	IsDefault bool   `json:"is_default"`
}

// UpdateTemplateRequest is the request body for updating a template (all fields optional).
type UpdateTemplateRequest struct {
	Subject   string `json:"subject"`
	BodyText  string `json:"body_text"`
	BodyHTML  string `json:"body_html"`
	IsDefault *bool  `json:"is_default"`
}

// SendEmailRequest is the tenant-scoped send-email request.
type SendEmailRequest struct {
	To              string            `json:"to"`
	ToArray         []string          `json:"to_array"`
	Template        string            `json:"template" binding:"required"`
	Variables       map[string]string `json:"variables"`
	SubjectOverride *string           `json:"subject_override"`
	BodyOverride    *string           `json:"body_override"`
	HTML            *bool             `json:"html"`
}

// TestSendRequest is the request for testing template rendering.
type TestSendRequest struct {
	To        string            `json:"to" binding:"required,email"`
	Template  string            `json:"template" binding:"required"`
	Variables map[string]string `json:"variables"`
}

// TemplateResponse is the JSON response for template queries.
type TemplateResponse struct {
	ID        string    `json:"id"`
	ContextID string    `json:"context_id"`
	Key       string    `json:"key"`
	Subject   string    `json:"subject"`
	BodyText  string    `json:"body_text"`
	BodyHTML  string    `json:"body_html"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TemplateListResponse wraps a list of templates with a count.
type TemplateListResponse struct {
	Templates []*TemplateResponse `json:"templates"`
	Count     int                 `json:"count"`
}

// SendEmailResponse is the response after enqueuing an email job.
type SendEmailResponse struct {
	JobID string `json:"job_id"`
}

// TestSendResponse includes rendered preview info.
type TestSendResponse struct {
	JobID               string `json:"job_id"`
	RenderedSubject     string `json:"rendered_subject"`
	RenderedBodyPreview string `json:"rendered_body_preview"`
}

// keyPattern validates template key format: alphanumeric + underscores only.
var keyPattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// ValidateKey checks that a template key is alphanumeric + underscore only.
func ValidateKey(key string) error {
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("template key must contain only alphanumeric characters and underscores")
	}
	return nil
}

// ValidateCreateRequest checks that at least body_text or body_html is non-empty.
func ValidateCreateRequest(req CreateTemplateRequest) error {
	if req.BodyText == "" && req.BodyHTML == "" {
		return fmt.Errorf("at least one of body_text or body_html must be non-empty")
	}
	return nil
}
