package webhook

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/h2hsecure/automata/internal/domain/webhook"
	"github.com/h2hsecure/automata/internal/infrastructure/queue"
)

// fakeRepo is an in-memory webhook.Repository for service tests.
type fakeRepo struct {
	endpoint *webhook.WebhookEndpoint
	getErr   error
	active   []*webhook.WebhookEndpoint
}

func (r *fakeRepo) Create(e *webhook.WebhookEndpoint) error { return nil }

func (r *fakeRepo) GetByID(id string) (*webhook.WebhookEndpoint, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.endpoint, nil
}

func (r *fakeRepo) ListByContext(contextID string) ([]*webhook.WebhookEndpoint, error) {
	return r.active, nil
}

func (r *fakeRepo) Update(e *webhook.WebhookEndpoint) error { return nil }

func (r *fakeRepo) Delete(id string) error { return nil }

func (r *fakeRepo) ListActiveByContextAndEvent(contextID, event string) ([]*webhook.WebhookEndpoint, error) {
	return r.active, nil
}

// fakeQueue records Enqueue calls and implements queue.WebhookQueue.
type fakeQueue struct {
	enqueued []enqueuedDelivery
	nextID   string
}

type enqueuedDelivery struct {
	url       string
	payload   map[string]interface{}
	contextID string
	formID    string
}

func (f *fakeQueue) Enqueue(_ context.Context, url string, payload map[string]interface{}, contextID, formID string) (string, error) {
	f.nextID = fmt.Sprintf("delivery-%d", len(f.enqueued)+1)
	f.enqueued = append(f.enqueued, enqueuedDelivery{url: url, payload: payload, contextID: contextID, formID: formID})
	return f.nextID, nil
}

func (f *fakeQueue) GetPending(limit int) ([]*queue.WebhookDelivery, error) { return nil, nil }

func (f *fakeQueue) MarkSuccess(id string) error { return nil }

func (f *fakeQueue) MarkFailed(id string, err string) error { return nil }

func (f *fakeQueue) UpdateNextRetry(id string, nextRetry time.Time) error { return nil }

func notFoundErr() error {
	return fmt.Errorf("%w: %w", webhook.ErrNotFound, sql.ErrNoRows)
}

func TestUpdateEndpoint_NotFound_ReturnsSentinel(t *testing.T) {
	svc := NewService(&fakeRepo{getErr: notFoundErr()}, &fakeQueue{})

	_, err := svc.UpdateEndpoint(context.Background(), "ctx-1", "ep-1", webhook.UpdateWebhookInput{Name: "n"})
	if !errors.Is(err, webhook.ErrNotFound) {
		t.Fatalf("expected errors.Is(err, webhook.ErrNotFound), got %q", err)
	}
}

func TestUpdateEndpoint_ForeignContext_ReturnsSentinel(t *testing.T) {
	svc := NewService(&fakeRepo{endpoint: &webhook.WebhookEndpoint{ID: "ep-1", ContextID: "other"}}, &fakeQueue{})

	_, err := svc.UpdateEndpoint(context.Background(), "ctx-1", "ep-1", webhook.UpdateWebhookInput{Name: "n"})
	if !errors.Is(err, webhook.ErrNotFound) {
		t.Fatalf("expected errors.Is(err, webhook.ErrNotFound), got %q", err)
	}
}

func TestDeliverTest_NotFound_ReturnsSentinel(t *testing.T) {
	svc := NewService(&fakeRepo{getErr: notFoundErr()}, &fakeQueue{})

	_, err := svc.DeliverTest(context.Background(), "ctx-1", "ep-1")
	if !errors.Is(err, webhook.ErrNotFound) {
		t.Fatalf("expected errors.Is(err, webhook.ErrNotFound), got %q", err)
	}
}

func TestDeliverTest_ForeignContext_ReturnsSentinel(t *testing.T) {
	svc := NewService(&fakeRepo{endpoint: &webhook.WebhookEndpoint{ID: "ep-1", ContextID: "other", URL: "https://example.com/hook"}}, &fakeQueue{})

	_, err := svc.DeliverTest(context.Background(), "ctx-1", "ep-1")
	if !errors.Is(err, webhook.ErrNotFound) {
		t.Fatalf("expected errors.Is(err, webhook.ErrNotFound), got %q", err)
	}
}

func TestDeliverTest_ReturnsDeliveryID(t *testing.T) {
	q := &fakeQueue{}
	svc := NewService(&fakeRepo{endpoint: &webhook.WebhookEndpoint{ID: "ep-1", ContextID: "ctx-1", URL: "https://example.com/hook"}}, q)

	id, err := svc.DeliverTest(context.Background(), "ctx-1", "ep-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatalf("expected non-empty delivery ID, got %q", id)
	}
	if len(q.enqueued) != 1 {
		t.Fatalf("expected 1 enqueued delivery, got %d", len(q.enqueued))
	}
	if q.enqueued[0].url != "https://example.com/hook" {
		t.Fatalf("unexpected enqueued url %q", q.enqueued[0].url)
	}
}

func TestDeliverEvent_EnqueuesPerMatchingEndpoint(t *testing.T) {
	q := &fakeQueue{}
	endpoint := &webhook.WebhookEndpoint{ID: "ep-1", ContextID: "ctx-1", URL: "https://example.com/hook", Events: []string{"form.submitted"}, Active: true}
	svc := NewService(&fakeRepo{active: []*webhook.WebhookEndpoint{endpoint}}, q)

	data := map[string]interface{}{"email": "user@example.com"}
	if err := svc.DeliverEvent(context.Background(), "ctx-1", "form.submitted", "form-1", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.enqueued) != 1 {
		t.Fatalf("expected 1 enqueued delivery, got %d", len(q.enqueued))
	}
	if q.enqueued[0].contextID != "ctx-1" || q.enqueued[0].formID != "form-1" {
		t.Fatalf("unexpected enqueue args: %+v", q.enqueued[0])
	}
	if q.enqueued[0].payload["event"] != "form.submitted" {
		t.Fatalf("unexpected payload event: %v", q.enqueued[0].payload["event"])
	}
}

func TestDeliverEvent_NoMatchingEndpoints_EnqueuesNothing(t *testing.T) {
	q := &fakeQueue{}
	svc := NewService(&fakeRepo{active: nil}, q)

	if err := svc.DeliverEvent(context.Background(), "ctx-1", "form.submitted", "form-1", map[string]interface{}{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.enqueued) != 0 {
		t.Fatalf("expected no enqueued deliveries, got %d", len(q.enqueued))
	}
}