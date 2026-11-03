package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	pagedomain "github.com/h2hsecure/automata/internal/domain/page"
	pageuc "github.com/h2hsecure/automata/internal/usecase/page"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Per-context editable HTML pages, served publicly at /page/<ctx>/<slug>.
// Schemas are stated explicitly (the SDK's inference marks every struct
// field as required).
const (
	schemaPagesList = `{
		"type": "object",
		"properties": {
			"page": {"type": "integer", "description": "1-based page number, default 1"},
			"limit": {"type": "integer", "description": "page size, default 20, max 100"}
		},
		"required": []
	}`

	schemaPageGet = `{
		"type": "object",
		"properties": {
			"page_id": {"type": "string", "description": "page ID (see pages_create/pages_list output field ID)"},
			"slug": {"type": "string", "description": "page slug (e.g. cookie-policy); alternative to page_id"}
		},
		"required": []
	}`

	schemaButton = `{
		"type": "object",
		"properties": {
			"label": {"type": "string", "description": "button label"},
			"href": {"type": "string", "description": "link target; if it starts with / it is served from this Automata instance (inline page) when the button is clicked"},
			"action": {"type": "string", "description": "link | dismiss_accept | dismiss_decline"},
			"variant": {"type": "string", "description": "visual style hint (e.g. primary, secondary)"}
		},
		"required": ["label", "action"]
	}`

	schemaPageCreate = `{
		"type": "object",
		"properties": {
			"slug": {"type": "string", "description": "URL slug served at /page/<context>/<slug> (e.g. cookie-policy)"},
			"title": {"type": "string", "description": "page title"},
			"content": {"type": "string", "description": "HTML content"},
			"is_active": {"type": "boolean", "description": "default true"},
			"position": {"type": "string", "description": "rendering position: top | bottom | left | right | full (empty = plain content page)"},
			"buttons": {"type": "array", "description": "banner buttons rendered with the page", "items":` + schemaButton + `}
		},
		"required": ["slug", "title"]
	}`

	schemaPageUpdate = `{
		"type": "object",
		"properties": {
			"page_id": {"type": "string", "description": "page ID (see pages_create/pages_list output field ID)"},
			"slug": {"type": "string"},
			"title": {"type": "string"},
			"content": {"type": "string"},
			"is_active": {"type": "boolean"},
			"position": {"type": "string", "description": "rendering position: top | bottom | left | right | full (empty = plain content page)"},
			"buttons": {"type": "array", "description": "banner buttons rendered with the page", "items":` + schemaButton + `}
		},
		"required": ["page_id", "slug", "title"]
	}`

	schemaPageID = `{"type": "object", "properties": {
		"page_id": {"type": "string", "description": "page ID (see pages_create/pages_list output field ID)"}
	}, "required": ["page_id"]}`
)

type pagesListIn struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

type pagesListOut struct {
	Items []pagedomain.Page `json:"items"`
	pageMeta
}

type pageGetIn struct {
	PageID string `json:"page_id"`
	Slug   string `json:"slug"`
}

type pageCreateIn struct {
	Slug     string              `json:"slug"`
	Title    string              `json:"title"`
	Content  string              `json:"content"`
	IsActive *bool               `json:"is_active"`
	Position pagedomain.Position `json:"position"`
	Buttons  []pagedomain.Button `json:"buttons"`
}

type pageUpdateIn struct {
	PageID   string              `json:"page_id"`
	Slug     string              `json:"slug"`
	Title    string              `json:"title"`
	Content  string              `json:"content"`
	IsActive *bool               `json:"is_active"`
	Position pagedomain.Position `json:"position"`
	Buttons  []pagedomain.Button `json:"buttons"`
}

type pageIDIn struct {
	PageID string `json:"page_id"`
}

type pageDeletedOut struct {
	Deleted bool `json:"deleted"`
}

// registerPageTools registers the per-context page domain tools on s, all
// scoped to the authenticated key's context.
func registerPageTools(s *mcp.Server, d Deps, contextID string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "pages_list",
		Description: "List pages for this context, paginated.",
		InputSchema: json.RawMessage(schemaPagesList),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in pagesListIn) (*mcp.CallToolResult, any, error) {
		pages, err := pageuc.ListPages(d.PageRepo, contextID)
		if err != nil {
			return nil, nil, err
		}
		items := make([]pagedomain.Page, 0, len(pages))
		for _, p := range pages {
			items = append(items, *p)
		}
		_, pageNum, limit := clampPage(in.Page, in.Limit)
		return nil, pagesListOut{
			Items: items,
			pageMeta: pageMeta{
				Total:   int64(len(items)),
				Page:    pageNum,
				Limit:   limit,
				HasMore: false,
			},
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "pages_get",
		Description: "Get a single page by ID or by slug.",
		InputSchema: json.RawMessage(schemaPageGet),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in pageGetIn) (*mcp.CallToolResult, any, error) {
		var (
			p   *pagedomain.Page
			err error
		)
		switch {
		case in.PageID != "":
			p, err = pageuc.GetPage(d.PageRepo, in.PageID)
			if err == nil && p.ContextID != contextID {
				// Not-found (not "not yours"): foreign pages are
				// indistinguishable from nonexistent ones.
				p, err = nil, fmt.Errorf("page not found")
			}
		case in.Slug != "":
			p, err = pageuc.GetPageBySlug(d.PageRepo, contextID, in.Slug)
			if err != nil {
				p, err = nil, fmt.Errorf("page not found")
			}
		default:
			return nil, nil, fmt.Errorf("page_id or slug required")
		}
		if err != nil {
			return nil, nil, err
		}
		return nil, p, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "pages_create",
		Description: "Create an editable HTML page served publicly at /page/<context>/<slug>.",
		InputSchema: json.RawMessage(schemaPageCreate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in pageCreateIn) (*mcp.CallToolResult, any, error) {
		p, err := pageuc.CreatePage(d.PageRepo, contextID, pageuc.CreatePageInput{
			Slug:     in.Slug,
			Title:    in.Title,
			Content:  in.Content,
			IsActive: in.IsActive,
			Position: in.Position,
			Buttons:  in.Buttons,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, p, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "pages_update",
		Description: "Update a page by ID (full replace of slug, title, content; is_active optional).",
		InputSchema: json.RawMessage(schemaPageUpdate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in pageUpdateIn) (*mcp.CallToolResult, any, error) {
		p, err := pageuc.UpdatePage(d.PageRepo, contextID, in.PageID, pageuc.CreatePageInput{
			Slug:     in.Slug,
			Title:    in.Title,
			Content:  in.Content,
			IsActive: in.IsActive,
			Position: in.Position,
			Buttons:  in.Buttons,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, p, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "pages_delete",
		Description: "Delete a page by ID.",
		InputSchema: json.RawMessage(schemaPageID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in pageIDIn) (*mcp.CallToolResult, any, error) {
		if err := pageuc.DeletePage(d.PageRepo, contextID, in.PageID); err != nil {
			return nil, nil, err
		}
		return nil, pageDeletedOut{Deleted: true}, nil
	})
}
