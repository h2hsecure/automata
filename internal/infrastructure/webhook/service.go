package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hekemen/automata/internal/domain/webhook"
	"github.com/hekemen/automata/internal/infrastructure/queue"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

// Service handles business logic for webhook endpoints.
type Service struct {
	repo   webhook.Repository
	queue  queue.WebhookQueue
	dbPool interface {
		SeedDefaultsForContext(ctx context.Context, contextID string) error
	}
}

// NewService creates a new webhook Service.
func NewService(repo webhook.Repository, q queue.WebhookQueue) *Service {
	return &Service{
		repo:  repo,
		queue: q,
	}
}

// CreateEndpoint validates and creates a new webhook endpoint.
func (s *Service) CreateEndpoint(ctx context.Context, input webhook.CreateWebhookInput) (*webhook.WebhookResponse, error) {
	if err := input.ValidateCreate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	var secretHash string
	var signingSecret string
	if input.Secret != "" {
		hashBytes, err := bcrypt.GenerateFromPassword([]byte(input.Secret), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash secret: %w", err)
		}
		secretHash = string(hashBytes)
		signingSecret = input.Secret
	}

	active := true
	if input.Active != nil {
		active = *input.Active
	}

	e := &webhook.WebhookEndpoint{
		ContextID:     input.ContextID,
		Name:          input.Name,
		URL:           input.URL,
		Events:        input.Events,
		SecretHash:    &secretHash,
		SigningSecret: &signingSecret,
		Active:        active,
	}

	if err := s.repo.Create(e); err != nil {
		return nil, fmt.Errorf("create webhook endpoint: %w", err)
	}

	return ToResponse(e), nil
}

// UpdateEndpoint validates and updates an existing webhook endpoint.
func (s *Service) UpdateEndpoint(ctx context.Context, id string, input webhook.UpdateWebhookInput) (*webhook.WebhookResponse, error) {
	if err := input.ValidateUpdate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("webhook endpoint not found: %w", err)
	}

	// Update fields
	if input.Name != "" {
		existing.Name = input.Name
	}
	if input.URL != "" {
		existing.URL = input.URL
	}
	if len(input.Events) > 0 {
		existing.Events = input.Events
	}
	if input.Active != nil {
		existing.Active = *input.Active
	}

	// Handle secret update — hash new secret if provided
	if input.Secret != "" {
		hashBytes, err := bcrypt.GenerateFromPassword([]byte(input.Secret), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash secret: %w", err)
		}
		hashStr := string(hashBytes)
		existing.SecretHash = &hashStr
		existing.SigningSecret = &input.Secret
	}

	if err := s.repo.Update(existing); err != nil {
		return nil, fmt.Errorf("update webhook endpoint: %w", err)
	}

	return ToResponse(existing), nil
}

// SignPayload computes HMAC-SHA256 signature of the payload using the endpoint's signing secret.
func (s *Service) SignPayload(endpoint *webhook.WebhookEndpoint, payload map[string]interface{}) (map[string]interface{}, error) {
	if endpoint.SigningSecret == nil || *endpoint.SigningSecret == "" {
		return payload, nil // No secret set — return payload unsigned
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload for signing: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(*endpoint.SigningSecret))
	mac.Write(payloadBytes)
	signature := hex.EncodeToString(mac.Sum(nil))

	payload["signature"] = signature
	return payload, nil
}

// DeliverTest sends a test payload to the webhook endpoint.
func (s *Service) DeliverTest(ctx context.Context, endpointID string) (string, error) {
	endpoint, err := s.repo.GetByID(endpointID)
	if err != nil {
		return "", fmt.Errorf("webhook endpoint not found: %w", err)
	}

	// Build sample payload
	samplePayload := map[string]interface{}{
		"event":        "test.ping",
		"context_id":   endpoint.ContextID,
		"data":         map[string]interface{}{"test": true, "note": "webhook test delivery from Automata"},
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}

	// Sign the payload
	signedPayload, err := s.SignPayload(endpoint, samplePayload)
	if err != nil {
		return "", fmt.Errorf("sign test payload: %w", err)
	}

	// Enqueue via the existing webhook queue
	deliveryID, err := enqueueDelivery(s.queue, endpoint.URL, signedPayload, endpoint.ContextID, "")
	if err != nil {
		return "", fmt.Errorf("enqueue test delivery: %w", err)
	}

	log.Info().Str("endpoint_id", endpointID).Str("delivery_id", deliveryID).Msg("webhook test delivery enqueued")
	return deliveryID, nil
}

// ListActiveByContextAndEvent retrieves active endpoints for a context that match the given event.
func (s *Service) ListActiveByContextAndEvent(ctx context.Context, contextID, event string) ([]*webhook.WebhookEndpoint, error) {
	return s.repo.ListActiveByContextAndEvent(contextID, event)
}

// DeliverEvent enqueues deliveries for all matching active endpoints.
func (s *Service) DeliverEvent(ctx context.Context, contextID, event, formID string, data map[string]interface{}) error {
	endpoints, err := s.repo.ListActiveByContextAndEvent(contextID, event)
	if err != nil {
		return fmt.Errorf("list matching endpoints: %w", err)
	}

	for _, endpoint := range endpoints {
		payload := map[string]interface{}{
			"event":        event,
			"context_id":   contextID,
			"data":         data,
			"timestamp":    time.Now().UTC().Format(time.RFC3339),
		}

		signedPayload, err := s.SignPayload(endpoint, payload)
		if err != nil {
			log.Error().Err(err).Str("endpoint_id", endpoint.ID).Msg("failed to sign webhook payload")
			continue
		}

		_, err = enqueueDelivery(s.queue, endpoint.URL, signedPayload, contextID, formID)
		if err != nil {
			log.Error().Err(err).Str("endpoint_id", endpoint.ID).Msg("failed to enqueue webhook delivery")
		}
	}

	return nil
}

// Repo returns the underlying repository for handler access.
func (s *Service) Repo() webhook.Repository {
	return s.repo
}

// ToResponse converts a domain endpoint to an API response (never exposes raw secret).
func ToResponse(e *webhook.WebhookEndpoint) *webhook.WebhookResponse {
	resp := &webhook.WebhookResponse{
		ID:        e.ID,
		Name:      e.Name,
		URL:       e.URL,
		Events:    e.Events,
		Active:    e.Active,
		ContextID: e.ContextID,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}

	if e.SecretHash != nil && *e.SecretHash != "" {
		resp.SecretSet = true
	}

	return resp
}

// enqueueDelivery is a helper that wraps queue.Enqueue with the delivery ID.
func enqueueDelivery(q queue.WebhookQueue, url string, payload map[string]interface{}, contextID, formID string) (string, error) {
	if err := q.Enqueue(url, payload, contextID, formID); err != nil {
		return "", fmt.Errorf("enqueue delivery: %w", err)
	}

	// We don't have the delivery ID from Enqueue, so generate one
	// The queue actually generates its own ID internally. Return empty string.
	return "", nil
}
