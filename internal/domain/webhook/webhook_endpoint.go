package webhook

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// WebhookEndpoint represents a configurable webhook endpoint scoped to a context.
type WebhookEndpoint struct {
	ID            string
	ContextID     string
	Name          string
	URL           string
	Events        []string
	SecretHash    *string // bcrypt hash (not exposed in responses)
	SigningSecret *string // raw secret for signing (used only at delivery time)
	Active        bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CreateWebhookInput is the request body for creating a webhook endpoint.
type CreateWebhookInput struct {
	ContextID string   `json:"context_id" binding:"required,uuid"`
	Name      string   `json:"name" binding:"required,min=1,max=128"`
	URL       string   `json:"url" binding:"required"`
	Events    []string `json:"events" binding:"required,min=1"`
	Secret    string   `json:"secret"`
	Active    *bool    `json:"active"`
}

// UpdateWebhookInput is the request body for updating a webhook endpoint (all fields optional).
type UpdateWebhookInput struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Events   []string `json:"events"`
	Secret   string   `json:"secret"`
	Active   *bool    `json:"active"`
}

// WebhookResponse is the JSON response for webhook queries (never includes raw secret).
type WebhookResponse struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	SecretSet bool     `json:"secret_set"`
	Active    bool     `json:"active"`
	ContextID string   `json:"context_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// WebhookListResponse wraps a list of webhooks with a total count.
type WebhookListResponse struct {
	Webhooks []*WebhookResponse `json:"webhooks"`
	Total    int                `json:"total"`
}

// AllowedEvents is the whitelist of event types that can be subscribed to.
var AllowedEvents = map[string]bool{
	"contact.created":    true,
	"contact.updated":    true,
	"contact.deleted":    true,
	"form.submitted":     true,
	"banner.impressed":   true,
	"banner.clicked":     true,
	"test.ping":          true,
}

// urlPattern validates webhook URLs (HTTPS or localhost/127.0.0.1).
var urlPattern = regexp.MustCompile(`^https://|^http://(localhost|127\.0\.0\.1)`)

// ValidateCreate validates a CreateWebhookInput.
func (input CreateWebhookInput) ValidateCreate() error {
	if !urlPattern.MatchString(input.URL) {
		return fmt.Errorf("url must use https:// or http://localhost / http://127.0.0.1")
	}
	for _, event := range input.Events {
		if !AllowedEvents[event] {
			return fmt.Errorf("event %q is not in the allowed events whitelist", event)
		}
	}
	if input.Secret != "" {
		if len(input.Secret) < 8 || len(input.Secret) > 256 {
			return fmt.Errorf("secret must be between 8 and 256 characters")
		}
	}
	return nil
}

// ValidateUpdate validates an UpdateWebhookInput (all fields optional).
func (input UpdateWebhookInput) ValidateUpdate() error {
	if input.URL != "" && !urlPattern.MatchString(input.URL) {
		return fmt.Errorf("url must use https:// or http://localhost / http://127.0.0.1")
	}
	for _, event := range input.Events {
		if !AllowedEvents[event] {
			return fmt.Errorf("event %q is not in the allowed events whitelist", event)
		}
	}
	if input.Secret != "" && (len(input.Secret) < 8 || len(input.Secret) > 256) {
		return fmt.Errorf("secret must be between 8 and 256 characters")
	}
	return nil
}

// HasEvent checks whether the endpoint's events list contains the given event.
func (e *WebhookEndpoint) HasEvent(event string) bool {
	for _, ev := range e.Events {
		if ev == event {
			return true
		}
	}
	return false
}

// ParseEvents converts a comma-separated string to a string slice, filtering empty entries.
func ParseEvents(events string) []string {
	parts := strings.Split(events, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// FormatEvents converts a string slice to a comma-separated string for storage.
func FormatEvents(events []string) string {
	return strings.Join(events, ",")
}

// IsValidEmail checks basic email format.
func IsValidEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// HTTPStatusText maps an HTTP status code to its standard text.
func HTTPStatusText(code int) string {
	return http.StatusText(code)
}
