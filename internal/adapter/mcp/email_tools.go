package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	edomain "github.com/hekemen/automata/internal/domain/email"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input schemas are stated explicitly (the SDK's inference marks every
// struct field as required). Only true identifiers are required.
const (
	schemaEmailTemplateID = `{"type": "object", "properties": {
		"template_id": {"type": "string", "description": "email template ID (see email_templates_list output field ID)"}
	}, "required": ["template_id"]}`

	schemaEmailTemplateCreate = `{
		"type": "object",
		"properties": {
			"key": {"type": "string", "description": "unique template key for this context"},
			"subject": {"type": "string", "description": "subject line; may contain {{variables}}"},
			"body_text": {"type": "string", "description": "plain-text body; at least one of body_text/body_html required"},
			"body_html": {"type": "string", "description": "HTML body; at least one of body_text/body_html required"},
			"is_default": {"type": "boolean", "description": "default template for this context"},
			"kind": {"type": "string", "description": "standalone or form; omitted = standalone"},
			"form_id": {"type": "string", "description": "form ID; required when kind is form"}
		},
		"required": ["key", "subject"]
	}`

	schemaEmailTemplateUpdate = `{
		"type": "object",
		"properties": {
			"template_id": {"type": "string", "description": "email template ID"},
			"subject": {"type": "string", "description": "new subject (applied when non-empty)"},
			"body_text": {"type": "string", "description": "new plain-text body (applied when non-empty)"},
			"body_html": {"type": "string", "description": "new HTML body (applied when non-empty)"},
			"is_default": {"type": "boolean", "description": "new default flag (applied when present)"},
			"kind": {"type": "string", "description": "new kind, standalone or form (applied when present)"},
			"form_id": {"type": "string", "description": "new form ID; null clears it (applied when present)"}
		},
		"required": ["template_id"]
	}`

	schemaEmailSend = `{
		"type": "object",
		"properties": {
			"to": {"type": "string", "description": "recipient address; at least one of to/to_array required"},
			"to_array": {"type": "array", "items": {"type": "string"}, "description": "recipient addresses"},
			"template": {"type": "string", "description": "template key"},
			"variables": {"type": "object", "additionalProperties": {"type": "string"}, "description": "template variables"},
			"subject_override": {"type": "string", "description": "subject override"},
			"body_override": {"type": "string", "description": "body override"},
			"html": {"type": "boolean", "description": "send as HTML"}
		},
		"required": ["template"]
	}`

	schemaEmailSendToContact = `{"type": "object", "properties": {
		"template_id": {"type": "string", "description": "email template ID"},
		"contact_id": {"type": "string", "description": "contact ID (see contacts_list output field ID)"}
	}, "required": ["template_id", "contact_id"]}`

	schemaEmailTestSend = `{
		"type": "object",
		"properties": {
			"to": {"type": "string", "description": "recipient address"},
			"template": {"type": "string", "description": "template key"},
			"variables": {"type": "object", "additionalProperties": {"type": "string"}, "description": "template variables"}
		},
		"required": ["to", "template"]
	}`
)

// registerEmailTools registers the email tools on s, all scoped to the
// authenticated key's context.
func registerEmailTools(s *mcp.Server, d Deps, contextID string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "email_templates_list",
		Description: "List email templates for this context.",
		InputSchema: json.RawMessage(`{"type": "object", "properties": {}, "required": []}`),
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		templates, err := d.EmailUsecase.Repo().List(contextID)
		if err != nil {
			return nil, nil, err
		}
		return nil, templates, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "email_templates_get",
		Description: "Get one email template by ID.",
		InputSchema: json.RawMessage(schemaEmailTemplateID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in emailTemplateIDIn) (*mcp.CallToolResult, any, error) {
		tmpl, err := d.EmailUsecase.Repo().GetByID(in.TemplateID)
		if err != nil {
			return nil, nil, fmt.Errorf("template not found")
		}
		if tmpl.ContextID != contextID {
			// Not-found (not "not yours"): foreign templates are
			// indistinguishable from nonexistent ones.
			return nil, nil, fmt.Errorf("template not found")
		}
		return nil, tmpl, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "email_templates_create",
		Description: "Create an email template for this context.",
		InputSchema: json.RawMessage(schemaEmailTemplateCreate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in emailTemplateCreateIn) (*mcp.CallToolResult, any, error) {
		req := edomain.CreateTemplateRequest{
			ContextID: contextID,
			Key:       in.Key,
			Subject:   in.Subject,
			BodyText:  in.BodyText,
			BodyHTML:  in.BodyHTML,
			IsDefault: in.IsDefault,
			Kind:      in.Kind,
			FormID:    in.FormID,
		}
		if err := edomain.ValidateKey(req.Key); err != nil {
			return nil, nil, err
		}
		if err := edomain.ValidateCreateRequest(req); err != nil {
			return nil, nil, err
		}
		tmpl := &edomain.EmailTemplate{
			ContextID: contextID,
			Key:       in.Key,
			Subject:   in.Subject,
			BodyText:  in.BodyText,
			BodyHTML:  in.BodyHTML,
			IsDefault: in.IsDefault,
			Kind:      in.Kind,
			FormID:    in.FormID,
		}
		if err := d.EmailUsecase.Repo().Create(tmpl); err != nil {
			return nil, nil, fmt.Errorf("create template: %w", err)
		}
		return nil, tmpl, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "email_templates_update",
		Description: "Update an email template; only the provided fields are applied.",
		InputSchema: json.RawMessage(schemaEmailTemplateUpdate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in emailTemplateUpdateIn) (*mcp.CallToolResult, any, error) {
		existing, err := d.EmailUsecase.Repo().GetByID(in.TemplateID)
		if err != nil {
			return nil, nil, fmt.Errorf("template not found")
		}
		if existing.ContextID != contextID {
			return nil, nil, fmt.Errorf("template not found")
		}
		if in.Subject != "" {
			existing.Subject = in.Subject
		}
		if in.BodyText != "" {
			existing.BodyText = in.BodyText
		}
		if in.BodyHTML != "" {
			existing.BodyHTML = in.BodyHTML
		}
		if in.IsDefault != nil {
			existing.IsDefault = *in.IsDefault
		}
		if in.Kind != nil {
			if err := edomain.ValidateKind(*in.Kind); err != nil {
				return nil, nil, err
			}
			existing.Kind = *in.Kind
		}
		if in.FormID != nil {
			existing.FormID = *in.FormID
		}
		// Final state must be consistent: form kind requires a form_id.
		if existing.Kind == edomain.TemplateKindForm && existing.FormID == nil {
			return nil, nil, fmt.Errorf("form_id required for form templates")
		}
		if err := d.EmailUsecase.Repo().Update(existing); err != nil {
			return nil, nil, err
		}
		return nil, existing, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "email_templates_delete",
		Description: "Delete an email template.",
		InputSchema: json.RawMessage(schemaEmailTemplateID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in emailTemplateIDIn) (*mcp.CallToolResult, any, error) {
		tmpl, err := d.EmailUsecase.Repo().GetByID(in.TemplateID)
		if err != nil {
			return nil, nil, fmt.Errorf("template not found")
		}
		if tmpl.ContextID != contextID {
			return nil, nil, fmt.Errorf("template not found")
		}
		if err := d.EmailUsecase.Repo().Delete(in.TemplateID); err != nil {
			return nil, nil, fmt.Errorf("delete template: %w", err)
		}
		return nil, map[string]any{"deleted": true}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "email_send",
		Description: "Render an email template and enqueue a delivery job. Returns the job ID.",
		InputSchema: json.RawMessage(schemaEmailSend),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in emailSendIn) (*mcp.CallToolResult, any, error) {
		jobID, err := d.EmailUsecase.SendEmail(contextID, edomain.SendEmailRequest{
			To:              in.To,
			ToArray:         in.ToArray,
			Template:        in.Template,
			Variables:       in.Variables,
			SubjectOverride: in.SubjectOverride,
			BodyOverride:    in.BodyOverride,
			HTML:            in.HTML,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, jobIDOut{JobID: jobID}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "email_send_to_contact",
		Description: "Render a standalone template with a contact's variables, enqueue delivery to their address, and record the activity. Returns the job ID.",
		InputSchema: json.RawMessage(schemaEmailSendToContact),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in emailSendToContactIn) (*mcp.CallToolResult, any, error) {
		jobID, err := d.EmailUsecase.SendToContact(contextID, in.TemplateID, in.ContactID)
		if err != nil {
			return nil, nil, err
		}
		return nil, jobIDOut{JobID: jobID}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "email_test_send",
		Description: "Render a template with variables, enqueue a test delivery, and return the rendered subject and a body preview.",
		InputSchema: json.RawMessage(schemaEmailTestSend),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in emailTestSendIn) (*mcp.CallToolResult, any, error) {
		jobID, subject, preview, err := d.EmailUsecase.TestSend(contextID, edomain.TestSendRequest{
			To:        in.To,
			Template:  in.Template,
			Variables: in.Variables,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, emailTestSendOut{JobID: jobID, Subject: subject, Preview: preview}, nil
	})
}

// --- In/Out types ---

type emailTemplateIDIn struct {
	TemplateID string `json:"template_id"`
}

type emailTemplateCreateIn struct {
	Key       string  `json:"key"`
	Subject   string  `json:"subject"`
	BodyText  string  `json:"body_text"`
	BodyHTML  string  `json:"body_html"`
	IsDefault bool    `json:"is_default"`
	Kind      string  `json:"kind"`
	FormID    *string `json:"form_id"`
}

type emailTemplateUpdateIn struct {
	TemplateID string   `json:"template_id"`
	Subject    string   `json:"subject"`
	BodyText   string   `json:"body_text"`
	BodyHTML   string   `json:"body_html"`
	IsDefault  *bool    `json:"is_default"`
	Kind       *string  `json:"kind"`
	FormID     **string `json:"form_id"`
}

type emailSendIn struct {
	To              string            `json:"to"`
	ToArray         []string          `json:"to_array"`
	Template        string            `json:"template"`
	Variables       map[string]string `json:"variables"`
	SubjectOverride *string           `json:"subject_override"`
	BodyOverride    *string           `json:"body_override"`
	HTML            *bool             `json:"html"`
}

type emailSendToContactIn struct {
	TemplateID string `json:"template_id"`
	ContactID  string `json:"contact_id"`
}

type emailTestSendIn struct {
	To        string            `json:"to"`
	Template  string            `json:"template"`
	Variables map[string]string `json:"variables"`
}

type jobIDOut struct {
	JobID string `json:"job_id"`
}

type emailTestSendOut struct {
	JobID   string `json:"job_id"`
	Subject string `json:"subject"`
	Preview string `json:"preview"`
}