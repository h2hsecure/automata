package mcp

import (
	"context"
	"encoding/json"

	"github.com/hekemen/automata/internal/domain/form"
	formusecase "github.com/hekemen/automata/internal/usecase/form"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input schemas are stated explicitly (the SDK's inference marks every
// struct field as required). form_id is the form slug, matching the REST
// routes.
const (
	schemaFormField = `{"type": "object", "properties": {
		"slug": {"type": "string"},
		"type": {"type": "string"},
		"label": {"type": "string"},
		"required": {"type": "boolean"},
		"validation": {"type": "object"},
		"order": {"type": "integer"}
	}, "required": ["slug", "type"]}`

	schemaFormID = `{
		"type": "object",
		"properties": {
			"form_id": {"type": "string", "description": "form slug (see forms_create/forms_get output field Slug)"}
		},
		"required": ["form_id"]
	}`

schemaFormIDRef = `{
		"type": "object",
		"properties": {
			"form_id": {"type": "string", "description": "form ID (see forms_create/forms_get output field ID)"}
		},
		"required": ["form_id"]
	}`

	schemaFormCreate = `{
		"type": "object",
		"properties": {
			"slug": {"type": "string", "description": "URL-safe form slug"},
			"name": {"type": "string"},
			"description": {"type": "string"},
			"fields": {"type": "array", "items": ` + schemaFormField + `},
			"settings": {"type": "object"}
		},
		"required": ["slug", "name", "fields"]
	}`

	schemaFormUpdate = `{
		"type": "object",
		"properties": {
			"form_id": {"type": "string", "description": "form ID (see forms_create/forms_get output field ID)"},
			"slug": {"type": "string", "description": "new slug; omitted keeps the current slug"},
			"name": {"type": "string"},
			"description": {"type": "string"},
			"fields": {"type": "array", "items": ` + schemaFormField + `},
			"settings": {"type": "object"}
		},
		"required": ["form_id", "fields"]
	}`

	schemaFormSubmit = `{
		"type": "object",
		"properties": {
			"form_id": {"type": "string", "description": "form slug"},
			"data": {"type": "object", "description": "submission values keyed by field slug"},
			"visitor_id": {"type": "string", "description": "anonymous visitor identifier"}
		},
		"required": ["form_id", "data"]
	}`

	schemaSubmissionsList = `{
		"type": "object",
		"properties": {
			"form_id": {"type": "string", "description": "form ID (see forms_create/forms_get output field ID)"},
			"page": {"type": "integer", "description": "1-based page number, default 1"},
			"limit": {"type": "integer", "description": "page size, default 20, max 100"}
		},
		"required": ["form_id"]
	}`

	schemaSubmissionID = `{
		"type": "object",
		"properties": {
			"submission_id": {"type": "string", "description": "submission ID"}
		},
		"required": ["submission_id"]
	}`
)

// registerFormTools registers the form domain tools on s, all scoped to the
// authenticated key's context.
func registerFormTools(s *mcp.Server, d Deps, contextID string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:          "forms_list",
		Description:   "List all forms for this context (unpaged).",
		InputSchema:   json.RawMessage(`{"type": "object", "properties": {}}`),
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ formsEmptyIn) (*mcp.CallToolResult, any, error) {
		forms, err := formusecase.ListForms(d.FormRepo, contextID)
		if err != nil {
			return nil, nil, err
		}
		items := make([]form.Form, 0, len(forms))
		for _, f := range forms {
			items = append(items, *f)
		}
		return nil, formsListOut{Items: items}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "forms_get",
		Description:   "Get a single form by slug.",
		InputSchema:   json.RawMessage(schemaFormID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in formsIDIn) (*mcp.CallToolResult, any, error) {
		f, err := formusecase.GetForm(d.FormRepo, contextID, in.FormID)
		if err != nil {
			return nil, nil, err
		}
		return nil, f, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "forms_create",
		Description:   "Create a form with its fields and settings.",
		InputSchema:   json.RawMessage(schemaFormCreate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in formsCreateIn) (*mcp.CallToolResult, any, error) {
		f, err := formusecase.CreateForm(d.FormRepo, contextID, formusecase.CreateFormInput{
			Slug:        in.Slug,
			Name:        in.Name,
			Description: in.Description,
			Fields:      in.Fields,
			Settings:    in.Settings,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, f, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "forms_update",
		Description:   "Update a form by slug. Fields (when provided) replace the form's fields.",
		InputSchema:   json.RawMessage(schemaFormUpdate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in formsUpdateIn) (*mcp.CallToolResult, any, error) {
		// The usecase overwrites the slug from the input, so keep the current
		// one when the client omits it.
		slug := in.Slug
		if slug == "" {
			existing, err := d.FormRepo.GetByID(in.FormID)
			if err != nil {
				return nil, nil, err
			}
			slug = existing.Slug
		}
		f, err := formusecase.UpdateForm(d.FormRepo, contextID, in.FormID, formusecase.CreateFormInput{
			Slug:        slug,
			Name:        in.Name,
			Description: in.Description,
			Fields:      in.Fields,
			Settings:    in.Settings,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, f, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "forms_delete",
		Description:   "Delete a form by ID.",
		InputSchema:   json.RawMessage(schemaFormIDRef),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in formsIDIn) (*mcp.CallToolResult, any, error) {
		if err := formusecase.DeleteForm(d.FormRepo, contextID, in.FormID); err != nil {
			return nil, nil, err
		}
		return nil, formDeletedOut{Deleted: true}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "forms_submit",
		Description:   "Submit a form by slug; data is validated against the form's fields.",
		InputSchema:   json.RawMessage(schemaFormSubmit),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in formsSubmitIn) (*mcp.CallToolResult, any, error) {
		sub, err := formusecase.SubmitForm(d.FormRepo, contextID, formusecase.SubmitFormInput{
			Slug:      in.FormID,
			Data:      in.Data,
			VisitorID: in.VisitorID,
		}, nil)
		if err != nil {
			return nil, nil, err
		}
		return nil, sub, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "form_submissions_list",
		Description:   "List submissions for a form, paginated.",
		InputSchema:   json.RawMessage(schemaSubmissionsList),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in submissionsListIn) (*mcp.CallToolResult, any, error) {
		offset, page, limit := clampPage(in.Page, in.Limit)
		subs, total, err := formusecase.ListSubmissions(d.FormRepo, contextID, in.FormID, form.ListSubmissionsOptions{
			Offset: offset,
			Limit:  limit,
		})
		if err != nil {
			return nil, nil, err
		}
		items := make([]form.Submission, 0, len(subs))
		for _, sub := range subs {
			items = append(items, *sub)
		}
		return nil, submissionsListOut{
			Items:    items,
			pageMeta: pageMeta{Total: total, Page: page, Limit: limit, HasMore: int64(page*limit) < total},
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "form_submissions_get",
		Description:   "Get a single submission by ID.",
		InputSchema:   json.RawMessage(schemaSubmissionID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in submissionIDIn) (*mcp.CallToolResult, any, error) {
		sub, err := formusecase.GetSubmission(d.FormRepo, contextID, in.SubmissionID)
		if err != nil {
			return nil, nil, err
		}
		return nil, sub, nil
	})
}

// --- In/Out types ---

type formsEmptyIn struct{}

type formsListOut struct {
	Items []form.Form `json:"items"`
}

type formsIDIn struct {
	FormID string `json:"form_id"`
}

type formsCreateIn struct {
	Slug        string                 `json:"slug"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Fields      []form.FormField       `json:"fields"`
	Settings    map[string]interface{} `json:"settings"`
}

type formsUpdateIn struct {
	FormID      string                 `json:"form_id"`
	Slug        string                 `json:"slug"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Fields      []form.FormField       `json:"fields"`
	Settings    map[string]interface{} `json:"settings"`
}

type formDeletedOut struct {
	Deleted bool `json:"deleted"`
}

type formsSubmitIn struct {
	FormID    string                 `json:"form_id"`
	Data      map[string]interface{} `json:"data"`
	VisitorID string                 `json:"visitor_id"`
}

type submissionsListIn struct {
	FormID string `json:"form_id"`
	Page   int    `json:"page"`
	Limit  int    `json:"limit"`
}

type submissionsListOut struct {
	Items []form.Submission `json:"items"`
	pageMeta
}

type submissionIDIn struct {
	SubmissionID string `json:"submission_id"`
}