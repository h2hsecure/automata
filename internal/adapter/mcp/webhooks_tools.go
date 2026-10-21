package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	whook "github.com/hekemen/automata/internal/domain/webhook"
	"github.com/hekemen/automata/internal/infrastructure/webhook"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input schemas are stated explicitly (the SDK's inference marks every
// struct field as required). Only true identifiers are required.
const (
	schemaWebhookID = `{"type": "object", "properties": {
		"webhook_id": {"type": "string", "description": "webhook endpoint ID (see webhooks_list output field id)"}
	}, "required": ["webhook_id"]}`

	schemaWebhookCreate = `{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "endpoint name"},
			"url": {"type": "string", "description": "delivery URL"},
			"events": {"type": "array", "items": {"type": "string"}, "description": "event types to subscribe to (e.g. form.submitted)"},
			"secret": {"type": "string", "description": "HMAC secret used to sign payloads"},
			"active": {"type": "boolean", "description": "whether the endpoint is active"}
		},
		"required": ["name", "url"]
	}`

	schemaWebhookUpdate = `{
		"type": "object",
		"properties": {
			"webhook_id": {"type": "string", "description": "webhook endpoint ID"},
			"name": {"type": "string", "description": "new endpoint name"},
			"url": {"type": "string", "description": "new delivery URL"},
			"events": {"type": "array", "items": {"type": "string"}, "description": "event types to subscribe to"},
			"secret": {"type": "string", "description": "new HMAC secret"},
			"active": {"type": "boolean", "description": "new active flag"}
		},
		"required": ["webhook_id"]
	}`
)

// registerWebhookTools registers the webhook tools on s, all scoped to the
// authenticated key's context.
func registerWebhookTools(s *mcp.Server, d Deps, contextID string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "webhooks_list",
		Description: "List webhook endpoints for this context.",
		InputSchema: json.RawMessage(`{"type": "object", "properties": {}, "required": []}`),
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		endpoints, err := d.WebhookSvc.Repo().ListByContext(contextID)
		if err != nil {
			return nil, nil, err
		}
		out := make([]*whook.WebhookResponse, len(endpoints))
		for i, e := range endpoints {
			out[i] = webhook.ToResponse(e)
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "webhooks_get",
		Description: "Get one webhook endpoint by ID.",
		InputSchema: json.RawMessage(schemaWebhookID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in webhookIDIn) (*mcp.CallToolResult, any, error) {
		e, err := d.WebhookSvc.Repo().GetByID(in.WebhookID)
		if err != nil {
			return nil, nil, fmt.Errorf("webhook not found")
		}
		if e.ContextID != contextID {
			// Not-found (not "not yours"): foreign endpoints are
			// indistinguishable from nonexistent ones.
			return nil, nil, fmt.Errorf("webhook not found")
		}
		return nil, webhook.ToResponse(e), nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "webhooks_create",
		Description: "Create a webhook endpoint for this context.",
		InputSchema: json.RawMessage(schemaWebhookCreate),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in webhookCreateIn) (*mcp.CallToolResult, any, error) {
		resp, err := d.WebhookSvc.CreateEndpoint(ctx, whook.CreateWebhookInput{
			ContextID: contextID,
			Name:      in.Name,
			URL:       in.URL,
			Events:    in.Events,
			Secret:    in.Secret,
			Active:    in.Active,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create webhook: %w", err)
		}
		return nil, resp, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "webhooks_update",
		Description: "Update a webhook endpoint.",
		InputSchema: json.RawMessage(schemaWebhookUpdate),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in webhookUpdateIn) (*mcp.CallToolResult, any, error) {
		resp, err := d.WebhookSvc.UpdateEndpoint(ctx, contextID, in.WebhookID, whook.UpdateWebhookInput{
			Name:   in.Name,
			URL:    in.URL,
			Events: in.Events,
			Secret: in.Secret,
			Active: in.Active,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("update webhook: %w", err)
		}
		return nil, resp, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "webhooks_delete",
		Description: "Delete a webhook endpoint.",
		InputSchema: json.RawMessage(schemaWebhookID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in webhookIDIn) (*mcp.CallToolResult, any, error) {
		e, err := d.WebhookSvc.Repo().GetByID(in.WebhookID)
		if err != nil {
			return nil, nil, fmt.Errorf("webhook not found")
		}
		if e.ContextID != contextID {
			return nil, nil, fmt.Errorf("webhook not found")
		}
		if err := d.WebhookSvc.Repo().Delete(in.WebhookID); err != nil {
			return nil, nil, fmt.Errorf("delete webhook: %w", err)
		}
		return nil, map[string]any{"deleted": true}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "webhooks_test",
		Description: "Send a test delivery to a webhook endpoint and return the delivery message.",
		InputSchema: json.RawMessage(schemaWebhookID),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in webhookIDIn) (*mcp.CallToolResult, any, error) {
		msg, err := d.WebhookSvc.DeliverTest(ctx, contextID, in.WebhookID)
		if err != nil {
			return nil, nil, fmt.Errorf("test delivery: %w", err)
		}
		return nil, webhookTestOut{Message: msg}, nil
	})
}

// --- In/Out types ---

type webhookIDIn struct {
	WebhookID string `json:"webhook_id"`
}

type webhookCreateIn struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret"`
	Active *bool    `json:"active"`
}

type webhookUpdateIn struct {
	WebhookID string   `json:"webhook_id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Secret    string   `json:"secret"`
	Active    *bool    `json:"active"`
}

type webhookTestOut struct {
	Message string `json:"message"`
}