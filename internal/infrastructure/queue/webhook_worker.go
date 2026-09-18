package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// WebhookHTTPClient defines the interface for making HTTP requests.
type WebhookHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// WebhookWorker is a background worker that processes webhook deliveries.
type WebhookWorker struct {
	queue    WebhookQueue
	client   WebhookHTTPClient
	pollInterval time.Duration
	batchSize  int
}

// NewWebhookWorker creates a new webhook worker.
func NewWebhookWorker(queue WebhookQueue) *WebhookWorker {
	return &WebhookWorker{
		queue:        queue,
		client:       &http.Client{Timeout: 30 * time.Second},
		pollInterval: 10 * time.Second,
		batchSize:    10,
	}
}

// Start begins the worker loop. It runs until ctx is cancelled.
func (w *WebhookWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	log.Info().Dur("interval", w.pollInterval).Msg("webhook worker started")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("webhook worker stopped")
			return
		case <-ticker.C:
			if err := w.processBatch(ctx); err != nil {
				log.Error().Err(err).Msg("error processing webhook batch")
			}
		}
	}
}

func (w *WebhookWorker) processBatch(ctx context.Context) error {
	deliveries, err := w.queue.GetPending(w.batchSize)
	if err != nil {
		return fmt.Errorf("get pending webhooks: %w", err)
	}

	if len(deliveries) == 0 {
		return nil
	}

	for _, d := range deliveries {
		if err := w.deliver(ctx, d); err != nil {
			log.Error().Str("id", d.ID).Err(err).Msg("webhook delivery failed")
		}
	}

	return nil
}

func (w *WebhookWorker) deliver(ctx context.Context, d *WebhookDelivery) error {
	payloadBytes, err := json.Marshal(d.Payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Source", "automata")
	req.Header.Set("X-Tenant-ID", d.TenantID)
	req.Header.Set("X-Form-ID", d.FormID)

	resp, err := w.client.Do(req)
	if err != nil {
		return w.handleDeliveryFailure(d, fmt.Errorf("http request: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return w.queue.MarkSuccess(d.ID)
	}

	return w.handleDeliveryFailure(d, fmt.Errorf("unexpected status: %d", resp.StatusCode))
}

func (w *WebhookWorker) handleDeliveryFailure(d *WebhookDelivery, err error) error {
	d.Attempts++

	if d.Attempts >= d.MaxRetries {
		errMsg := err.Error()
		if len(errMsg) > 500 {
			errMsg = errMsg[:500]
		}
		return w.queue.MarkFailed(d.ID, errMsg)
	}

	nextRetry := exponentialBackoff(d.Attempts)
	return w.queue.UpdateNextRetry(d.ID, nextRetry)
}

// exponentialBackoff calculates the retry delay using the formula:
// 1 * time.Minute * time.Duration(pow(3, attempt))
func exponentialBackoff(attempt int) time.Time {
	factor := math.Pow(3, float64(attempt))
	delay := time.Duration(factor) * time.Minute
	return time.Now().Add(delay)
}
