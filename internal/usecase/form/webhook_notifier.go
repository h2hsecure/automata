package form

import "context"

// WebhookNotifier enqueues webhook deliveries for domain events.
// Implemented by the webhook infrastructure service, which keeps the
// usecase free of any internal/infrastructure imports.
type WebhookNotifier interface {
	DeliverEvent(ctx context.Context, contextID, event, formID string, data map[string]interface{}) error
}