package webhook

import "fmt"

var (
	ErrNotFound = fmt.Errorf("webhook endpoint not found")
)

// Repository defines the persistence interface for webhook endpoints.
type Repository interface {
	Create(e *WebhookEndpoint) error
	GetByID(id string) (*WebhookEndpoint, error)
	ListByContext(contextID string) ([]*WebhookEndpoint, error)
	Update(e *WebhookEndpoint) error
	Delete(id string) error
	ListActiveByContextAndEvent(contextID, event string) ([]*WebhookEndpoint, error)
}
