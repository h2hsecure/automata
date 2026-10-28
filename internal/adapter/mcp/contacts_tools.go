package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	contact "github.com/h2hsecure/automata/internal/domain/contact"
	contactusecase "github.com/h2hsecure/automata/internal/usecase/contact"
	csvimport "github.com/h2hsecure/automata/pkg/import"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The SDK infers an input schema in which every struct field is required,
// which is wrong for these tools (all arguments except the primary
// identifiers are optional). Input schemas are therefore stated explicitly
// per tool.
const (
	schemaContactsList = `{
		"type": "object",
		"properties": {
			"page": {"type": "integer", "description": "1-based page number, default 1"},
			"limit": {"type": "integer", "description": "page size, default 20, max 100"},
			"search": {"type": "string", "description": "filter by email or name"},
			"tags": {"type": "array", "items": {"type": "string"}, "description": "filter by tags"},
			"source": {"type": "string", "description": "filter by source"},
			"company": {"type": "string", "description": "filter by company"},
			"min_submissions": {"type": "integer", "description": "minimum number of form submissions"},
			"hidden": {"type": "boolean", "description": "filter by hidden state: true = hidden only, false = exclude hidden"}
		}
	}`

	schemaContactID = `{
		"type": "object",
		"properties": {
			"contact_id": {"type": "string", "description": "contact ID"}
		},
		"required": ["contact_id"]
	}`

	schemaContactsCreate = `{
		"type": "object",
		"properties": {
			"email": {"type": "string", "description": "contact email (dedup key)"},
			"first_name": {"type": "string"},
			"last_name": {"type": "string"},
			"phone": {"type": "string"},
			"company": {"type": "string"},
			"custom_fields": {"type": "object", "additionalProperties": true},
			"source": {"type": "string"},
			"source_id": {"type": "string"},
			"tags": {"type": "array", "items": {"type": "string"}}
		}
	}`

	schemaContactsUpdate = `{
		"type": "object",
		"properties": {
			"contact_id": {"type": "string", "description": "contact ID"},
			"email": {"type": "string"},
			"first_name": {"type": "string"},
			"last_name": {"type": "string"},
			"phone": {"type": "string"},
			"company": {"type": "string"},
			"custom_fields": {"type": "object", "additionalProperties": true},
			"tags": {"type": "array", "items": {"type": "string"}, "description": "replaces all existing tags when provided"}
		},
		"required": ["contact_id"]
	}`

	schemaContactsMerge = `{
		"type": "object",
		"properties": {
			"keep_contact_id": {"type": "string", "description": "contact to keep (its ID survives)"},
			"merge_contact_id": {"type": "string", "description": "contact to merge into the kept one, then remove"}
		},
		"required": ["keep_contact_id", "merge_contact_id"]
	}`

	schemaContactsActivity = `{
		"type": "object",
		"properties": {
			"contact_id": {"type": "string", "description": "contact ID"},
			"page": {"type": "integer", "description": "1-based page number, default 1"},
			"limit": {"type": "integer", "description": "page size, default 20, max 100"},
			"activity_type": {"type": "string", "description": "filter by activity type"}
		},
		"required": ["contact_id"]
	}`

	schemaContactsImport = `{
		"type": "object",
		"properties": {
			"csv_data": {"type": "string", "description": "CSV data to import: header row plus data rows"},
			"mapping": {"type": "object", "additionalProperties": {"type": "integer"}, "description": "optional field name to CSV column index mapping"},
			"source": {"type": "string", "description": "source label recorded on imported contacts"}
		},
		"required": ["csv_data"]
	}`

	schemaContactsExport = `{
		"type": "object",
		"properties": {
			"tags": {"type": "array", "items": {"type": "string"}, "description": "filter by tags"},
			"fields": {"type": "array", "items": {"type": "string"}, "description": "fields to export, default id,email,first_name,last_name,phone,company,source,tags,created_at"}
		}
	}`
)

// registerContactTools registers the contact domain tools on s, all scoped
// to the authenticated key's context.
func registerContactTools(s *mcp.Server, d Deps, contextID string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:          "contacts_list",
		Description:   "List contacts for this context, paginated. Optional filters: search (email/name), tags, source, company, min_submissions, hidden.",
		InputSchema:   json.RawMessage(schemaContactsList),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in contactsListIn) (*mcp.CallToolResult, any, error) {
		offset, page, limit := clampPage(in.Page, in.Limit)
		cw, err := contactusecase.ListContacts(d.ContactRepo, contextID, contactusecase.ListOptions{
			Offset: offset,
			Limit:  limit,
			Filters: contact.FilterOptions{
				Tags:           in.Tags,
				Source:         in.Source,
				Company:        in.Company,
				MinSubmissions: in.MinSubmissions,
				Search:         in.Search,
				Hidden:         in.Hidden,
			},
		})
		if err != nil {
			return nil, contactsListOut{}, err
		}
		items := make([]contact.Contact, 0, len(cw.Contacts))
		for _, c := range cw.Contacts {
			items = append(items, *c)
		}
		return nil, contactsListOut{
			Items:    items,
			pageMeta: pageMeta{Total: cw.Total, Page: page, Limit: limit, HasMore: int64(page*limit) < cw.Total},
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "contacts_get",
		Description:   "Get a single contact by ID.",
		InputSchema:   json.RawMessage(schemaContactID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in contactsIDIn) (*mcp.CallToolResult, any, error) {
		c, err := contactusecase.GetContact(d.ContactRepo, in.ContactID, contextID)
		if err != nil {
			return nil, contact.Contact{}, err
		}
		return nil, *c, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "contacts_create",
		Description:   "Create a contact, or return the existing one when the email is already known.",
		InputSchema:   json.RawMessage(schemaContactsCreate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in contactsCreateIn) (*mcp.CallToolResult, any, error) {
		c, err := contactusecase.CreateContact(d.ContactRepo, contextID, contactusecase.CreateInput{
			Email:        in.Email,
			FirstName:    in.FirstName,
			LastName:     in.LastName,
			Phone:        in.Phone,
			Company:      in.Company,
			CustomFields: in.CustomFields,
			Source:       in.Source,
			SourceID:     in.SourceID,
			Tags:         in.Tags,
		})
		if err != nil {
			return nil, contact.Contact{}, err
		}
		return nil, *c, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "contacts_update",
		Description:   "Update a contact by ID. Tags (when provided) replace all existing tags.",
		InputSchema:   json.RawMessage(schemaContactsUpdate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in contactsUpdateIn) (*mcp.CallToolResult, any, error) {
		c, err := contactusecase.UpdateContact(d.ContactRepo, in.ContactID, contextID, contactusecase.UpdateInput{
			Email:        in.Email,
			FirstName:    in.FirstName,
			LastName:     in.LastName,
			Phone:        in.Phone,
			Company:      in.Company,
			CustomFields: in.CustomFields,
			Tags:         in.Tags,
		})
		if err != nil {
			return nil, contact.Contact{}, err
		}
		return nil, *c, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "contacts_delete",
		Description:   "Delete a contact by ID.",
		InputSchema:   json.RawMessage(schemaContactID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in contactsIDIn) (*mcp.CallToolResult, any, error) {
		if err := contactusecase.DeleteContact(d.ContactRepo, in.ContactID, contextID); err != nil {
			return nil, contactDeletedOut{}, err
		}
		return nil, contactDeletedOut{Deleted: true}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "contacts_merge",
		Description:   "Merge one contact into another: the kept contact keeps its ID and the merged one is removed.",
		InputSchema:   json.RawMessage(schemaContactsMerge),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in contactsMergeIn) (*mcp.CallToolResult, any, error) {
		if err := contactusecase.MergeContacts(d.ContactRepo, contextID, in.KeepContactID, in.MergeContactID); err != nil {
			return nil, contactMergedOut{}, err
		}
		return nil, contactMergedOut{Merged: true}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "contacts_activity",
		Description:   "List a contact's activity feed, paginated. Optional filter: activity_type.",
		InputSchema:   json.RawMessage(schemaContactsActivity),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in contactsActivityIn) (*mcp.CallToolResult, any, error) {
		offset, page, limit := clampPage(in.Page, in.Limit)
		res, err := contactusecase.GetActivity(d.ContactRepo, in.ContactID, contextID, offset, limit, in.ActivityType)
		if err != nil {
			return nil, contactsActivityOut{}, err
		}
		return nil, contactsActivityOut{
			Items: res.Activities,
			pageMeta: pageMeta{
				Total:   res.Total,
				Page:    page,
				Limit:   limit,
				HasMore: int64(page*limit) < res.Total,
			},
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "contacts_import",
		Description:   "Import contacts from CSV data (header row plus data rows). Returns created/updated/skipped counts and per-row errors. Duplicate emails are counted as updated.",
		InputSchema:   json.RawMessage(schemaContactsImport),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in contactsImportIn) (*mcp.CallToolResult, any, error) {
		result, err := contactusecase.ImportContacts(d.ContactRepo, contextID, contactusecase.ImportCSVInput{
			CSVData: strings.NewReader(in.CSVData),
			Mapping: in.Mapping,
			Source:  in.Source,
		})
		if err != nil {
			return nil, csvimport.ImportResult{}, err
		}
		return nil, *result, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:          "contacts_export",
		Description:   "Export contacts for this context as a base64-encoded CSV. Optional filters: tags, and fields (defaults to id,email,first_name,last_name,phone,company,source,tags,created_at).",
		InputSchema:   json.RawMessage(schemaContactsExport),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in contactsExportIn) (*mcp.CallToolResult, any, error) {
		fields := in.Fields
		if len(fields) == 0 {
			fields = []string{"id", "email", "first_name", "last_name", "phone", "company", "source", "tags", "created_at"}
		}
		csv, err := contactusecase.ExportContacts(d.ContactRepo, contextID, contact.FilterOptions{Tags: in.Tags}, fields)
		if err != nil {
			return nil, contactsExportOut{}, err
		}
		return nil, contactsExportOut{CSVBase64: base64.StdEncoding.EncodeToString(csv)}, nil
	})
}

// --- In/Out types (In shapes mirror the schemas above) ---

type contactsListIn struct {
	Page           int      `json:"page"`
	Limit          int      `json:"limit"`
	Search         string   `json:"search"`
	Tags           []string `json:"tags"`
	Source         string   `json:"source"`
	Company        string   `json:"company"`
	MinSubmissions int      `json:"min_submissions"`
	Hidden         *bool    `json:"hidden"`
}

type contactsListOut struct {
	Items []contact.Contact `json:"items"`
	pageMeta
}

type contactsIDIn struct {
	ContactID string `json:"contact_id"`
}

type contactsCreateIn struct {
	Email        *string                `json:"email"`
	FirstName    string                 `json:"first_name"`
	LastName     string                 `json:"last_name"`
	Phone        string                 `json:"phone"`
	Company      string                 `json:"company"`
	CustomFields map[string]interface{} `json:"custom_fields"`
	Source       string                 `json:"source"`
	SourceID     string                 `json:"source_id"`
	Tags         []string               `json:"tags"`
}

type contactsUpdateIn struct {
	ContactID    string                 `json:"contact_id"`
	Email        *string                `json:"email"`
	FirstName    string                 `json:"first_name"`
	LastName     string                 `json:"last_name"`
	Phone        string                 `json:"phone"`
	Company      string                 `json:"company"`
	CustomFields map[string]interface{} `json:"custom_fields"`
	Tags         []string               `json:"tags"`
}

type contactDeletedOut struct {
	Deleted bool `json:"deleted"`
}

type contactsMergeIn struct {
	KeepContactID  string `json:"keep_contact_id"`
	MergeContactID string `json:"merge_contact_id"`
}

type contactMergedOut struct {
	Merged bool `json:"merged"`
}

type contactsActivityIn struct {
	ContactID    string  `json:"contact_id"`
	Page         int     `json:"page"`
	Limit        int     `json:"limit"`
	ActivityType *string `json:"activity_type"`
}

type contactsActivityOut struct {
	Items []contact.ActivityDetail `json:"items"`
	pageMeta
}

type contactsImportIn struct {
	CSVData string                   `json:"csv_data"`
	Mapping csvimport.FieldMapping   `json:"mapping"`
	Source  string                   `json:"source"`
}

type contactsExportIn struct {
	Tags   []string `json:"tags"`
	Fields []string `json:"fields"`
}

type contactsExportOut struct {
	CSVBase64 string `json:"csv_base64"`
}