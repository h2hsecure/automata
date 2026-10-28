package mcp

import (
	"context"
	"encoding/json"
	"time"

	bdomain "github.com/h2hsecure/automata/internal/domain/banner"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input schemas are stated explicitly (the SDK's inference marks every
// struct field as required). banner_id is the banner's database ID, as
// returned in the output.
const (
	schemaBannerID = `{"type": "object", "properties": {
		"banner_id": {"type": "string", "description": "banner ID (see banners_create/banners_get output field ID)"}
	}, "required": ["banner_id"]}`

	schemaBannersList = `{
		"type": "object",
		"properties": {
			"campaign_id": {"type": "string", "description": "filter by campaign ID"},
			"is_active": {"type": "boolean", "description": "when true, only active banners"},
			"page": {"type": "integer", "description": "1-based page number, default 1"},
			"limit": {"type": "integer", "description": "page size, default 20, max 100"}
		},
		"required": []
	}`

	schemaABVariant = `{"type": "object", "properties": {
		"id": {"type": "string"},
		"name": {"type": "string"},
		"content": {"type": "string"},
		"link_url": {"type": "string"},
		"weight": {"type": "integer"},
		"priority": {"type": "integer"}
	}}`

	schemaBannerCreate = `{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "banner name"},
			"type": {"type": "string", "description": "html, image, video, popup, or sticky"},
			"content": {"type": "string", "description": "HTML content"},
			"link_url": {"type": "string"},
			"image_url": {"type": "string"},
			"alt_text": {"type": "string"},
			"campaign_id": {"type": "string"},
			"placements": {"type": "array", "items": {"type": "string"}, "description": "placement slugs/IDs to serve on"},
			"priority": {"type": "integer"},
			"start_date": {"type": "string", "description": "RFC3339 timestamp"},
			"end_date": {"type": "string", "description": "RFC3339 timestamp"},
			"ab_test": {"type": "boolean"},
			"ab_variants": {"type": "array", "items": ` + schemaABVariant + `}
		},
		"required": ["name"]
	}`

	schemaBannerUpdate = `{
		"type": "object",
		"properties": {
			"banner_id": {"type": "string", "description": "banner ID (see banners_create/banners_get output field ID)"},
			"name": {"type": "string"},
			"type": {"type": "string", "description": "html, image, video, popup, or sticky"},
			"content": {"type": "string"},
			"link_url": {"type": "string"},
			"image_url": {"type": "string"},
			"alt_text": {"type": "string"},
			"campaign_id": {"type": "string"},
			"placements": {"type": "array", "items": {"type": "string"}},
			"priority": {"type": "integer"},
			"start_date": {"type": "string", "description": "RFC3339 timestamp"},
			"end_date": {"type": "string", "description": "RFC3339 timestamp"},
			"is_active": {"type": "boolean"},
			"ab_test": {"type": "boolean"},
			"ab_variants": {"type": "array", "items": ` + schemaABVariant + `}
		},
		"required": ["banner_id"]
	}`
)

func mapABVariants(in []abVariantIn) []bdomain.ABVariant {
	out := make([]bdomain.ABVariant, 0, len(in))
	for _, v := range in {
		out = append(out, bdomain.ABVariant{
			ID:       v.ID,
			Name:     v.Name,
			Content:  v.Content,
			LinkURL:  v.LinkURL,
			Weight:   v.Weight,
			Priority: v.Priority,
		})
	}
	return out
}

// registerBannerTools registers the banner domain tools on s, all scoped to
// the authenticated key's context (get/update/delete resolve by ID).
func registerBannerTools(s *mcp.Server, d Deps, contextID string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "banners_list",
		Description: "List banners for this context, paginated.",
		InputSchema: json.RawMessage(schemaBannersList),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in bannersListIn) (*mcp.CallToolResult, any, error) {
		offset, page, limit := clampPage(in.Page, in.Limit)
		banners, total, err := d.BannerUsecase.ListBanners(contextID, bdomain.ListOptions{
			CampaignID: in.CampaignID,
			IsActive:   in.IsActive,
			Offset:     offset,
			Limit:      limit,
		})
		if err != nil {
			return nil, nil, err
		}
		items := make([]bdomain.Banner, 0, len(banners))
		for _, b := range banners {
			items = append(items, *b)
		}
		return nil, bannersListOut{
			Items:    items,
			pageMeta: pageMeta{Total: total, Page: page, Limit: limit, HasMore: int64(page*limit) < total},
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "banners_get",
		Description: "Get a single banner by ID.",
		InputSchema: json.RawMessage(schemaBannerID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in bannerIDIn) (*mcp.CallToolResult, any, error) {
		b, err := d.BannerUsecase.GetBanner(contextID, in.BannerID)
		if err != nil {
			return nil, nil, err
		}
		return nil, b, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "banners_create",
		Description: "Create a banner.",
		InputSchema: json.RawMessage(schemaBannerCreate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in bannersCreateIn) (*mcp.CallToolResult, any, error) {
		b, err := d.BannerUsecase.CreateBanner(contextID, &bdomain.CreateBannerRequest{
			Name:       in.Name,
			Type:       bdomain.BannerType(in.Type),
			Content:    in.Content,
			LinkURL:    in.LinkURL,
			ImageURL:   in.ImageURL,
			AltText:    in.AltText,
			CampaignID: in.CampaignID,
			Placements: in.Placements,
			Priority:   in.Priority,
			StartDate:  in.StartDate,
			EndDate:    in.EndDate,
			ABTest:     in.ABTest,
			ABVariants: mapABVariants(in.ABVariants),
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, b, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "banners_update",
		Description: "Update a banner by ID; only provided fields change.",
		InputSchema: json.RawMessage(schemaBannerUpdate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in bannersUpdateIn) (*mcp.CallToolResult, any, error) {
		b, err := d.BannerUsecase.UpdateBanner(contextID, in.BannerID, &bdomain.UpdateBannerRequest{
			Name:       in.Name,
			Type:       bdomain.BannerType(in.Type),
			Content:    in.Content,
			LinkURL:    in.LinkURL,
			ImageURL:   in.ImageURL,
			AltText:    in.AltText,
			CampaignID: in.CampaignID,
			Placements: in.Placements,
			Priority:   in.Priority,
			StartDate:  in.StartDate,
			EndDate:    in.EndDate,
			IsActive:   in.IsActive,
			ABTest:     in.ABTest,
			ABVariants: mapABVariants(in.ABVariants),
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, b, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "banners_delete",
		Description: "Delete a banner by ID.",
		InputSchema: json.RawMessage(schemaBannerID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in bannerIDIn) (*mcp.CallToolResult, any, error) {
		if err := d.BannerUsecase.DeleteBanner(contextID, in.BannerID); err != nil {
			return nil, nil, err
		}
		return nil, bannerDeletedOut{Deleted: true}, nil
	})
}

// --- In/Out types ---

type bannersListIn struct {
	CampaignID string `json:"campaign_id"`
	IsActive   bool   `json:"is_active"`
	Page       int    `json:"page"`
	Limit      int    `json:"limit"`
}

type bannersListOut struct {
	Items []bdomain.Banner `json:"items"`
	pageMeta
}

type bannerIDIn struct {
	BannerID string `json:"banner_id"`
}

type abVariantIn struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Content  string `json:"content"`
	LinkURL  string `json:"link_url"`
	Weight   int    `json:"weight"`
	Priority int    `json:"priority"`
}

type bannersCreateIn struct {
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	Content    string        `json:"content"`
	LinkURL    string        `json:"link_url"`
	ImageURL   string        `json:"image_url"`
	AltText    string        `json:"alt_text"`
	CampaignID string        `json:"campaign_id"`
	Placements []string      `json:"placements"`
	Priority   int           `json:"priority"`
	StartDate  time.Time     `json:"start_date"`
	EndDate    time.Time     `json:"end_date"`
	ABTest     bool          `json:"ab_test"`
	ABVariants []abVariantIn `json:"ab_variants"`
}

type bannersUpdateIn struct {
	BannerID   string        `json:"banner_id"`
	Name       string        `json:"name"`
	Type       string        `json:"type"`
	Content    string        `json:"content"`
	LinkURL    string        `json:"link_url"`
	ImageURL   string        `json:"image_url"`
	AltText    string        `json:"alt_text"`
	CampaignID string        `json:"campaign_id"`
	Placements []string      `json:"placements"`
	Priority   int           `json:"priority"`
	StartDate  time.Time     `json:"start_date"`
	EndDate    time.Time     `json:"end_date"`
	IsActive   *bool         `json:"is_active"`
	ABTest     *bool         `json:"ab_test"`
	ABVariants []abVariantIn `json:"ab_variants"`
}

type bannerDeletedOut struct {
	Deleted bool `json:"deleted"`
}