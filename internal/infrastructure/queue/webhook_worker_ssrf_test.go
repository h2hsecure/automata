package queue

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"
)

// ssrfFakeQueue implements WebhookQueue, recording MarkFailed calls.
type ssrfFakeQueue struct {
	failed map[string]string
	nextID int
}

func (q *ssrfFakeQueue) Enqueue(ctx context.Context, url string, payload map[string]interface{}, contextID, formID string) (string, error) {
	q.nextID++
	return "id", nil
}
func (q *ssrfFakeQueue) GetPending(limit int) ([]*WebhookDelivery, error) { return nil, nil }
func (q *ssrfFakeQueue) Claim(ctx context.Context, limit int) ([]*WebhookDelivery, error) {
	return nil, nil
}
func (q *ssrfFakeQueue) ResetStaleProcessing(ctx context.Context, olderThan time.Time) (int64, error) {
	return 0, nil
}
func (q *ssrfFakeQueue) MarkSuccess(id string) error { return nil }
func (q *ssrfFakeQueue) MarkFailed(id string, err string) error {
	q.failed[id] = err
	return nil
}
func (q *ssrfFakeQueue) UpdateNextRetry(id string, nextRetry time.Time) error { return nil }

// ssrfFakeClient counts HTTP requests; it must never be called for a URL
// that fails the dispatch-time SSRF revalidation.
type ssrfFakeClient struct {
	requests int
}

func (c *ssrfFakeClient) Do(req *http.Request) (*http.Response, error) {
	c.requests++
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}, nil
}

// TestWebhookWorker_SSRFRecheckAtDispatch is the regression test for the SSRF
// finding: deliveries enqueued before the allowlist tightened (or with rows
// crafted to bypass it) must be marked failed at dispatch time and never hit
// the network.
func TestWebhookWorker_SSRFRecheckAtDispatch(t *testing.T) {
	t.Run("loopback-prefix trick is blocked", func(t *testing.T) {
		fq := &ssrfFakeQueue{failed: map[string]string{}}
		fc := &ssrfFakeClient{}
		w := &WebhookWorker{queue: fq, client: fc, pollInterval: 0, batchSize: 10}

		_ = w.deliver(context.Background(), &WebhookDelivery{
			ID: "d1", URL: "http://localhost.evil.com/x",
		})
		if fq.failed["d1"] == "" {
			t.Fatal("MarkFailed was not called for the SSRF delivery")
		}
		if fc.requests != 0 {
			t.Fatalf("client made %d requests, want 0 (delivery must be blocked before HTTP)", fc.requests)
		}
	})

	t.Run("valid https URL is dispatched", func(t *testing.T) {
		fq := &ssrfFakeQueue{failed: map[string]string{}}
		fc := &ssrfFakeClient{}
		w := &WebhookWorker{queue: fq, client: fc, pollInterval: 0, batchSize: 10}

		_ = w.deliver(context.Background(), &WebhookDelivery{
			ID: "d2", URL: "https://example.com/x", Payload: map[string]interface{}{"a": 1},
		})
		if fc.requests != 1 {
			t.Fatalf("client made %d requests, want 1", fc.requests)
		}
		if len(fq.failed) != 0 {
			t.Fatalf("MarkFailed was called for a valid delivery: %v", fq.failed)
		}
	})
}
