package mcp

import (
	"context"
	"encoding/json"

	bdomain "github.com/hekemen/automata/internal/domain/banner"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input schemas are stated explicitly (the SDK's inference marks every
// struct field as required). placement_id is the placement's database ID,
// as returned in the output.
const (
	schemaPlacementID = `{"type": "object", "properties": {
		"placement_id": {"type": "string", "description": "placement ID (see placements_create/placements_get output field ID)"}
	}, "required": ["placement_id"]}`

	schemaPlacementsList = `{"type": "object", "properties": {
		"is_active": {"type": "boolean", "description": "when true, only active placements"}
	}, "required": []}`

	schemaPlacementFields = `
		"name": {"type": "string"},
		"location": {"type": "string"},
		"css_selector": {"type": "string", "description": "CSS selector of the ad slot"},
		"max_banners": {"type": "integer"},
		"priority": {"type": "integer"}`

	schemaPlacementCreate = `{"type": "object", "properties": {` + schemaPlacementFields + `}, "required": ["name", "css_selector"]}`

	schemaPlacementUpdate = `{"type": "object", "properties": {
		"placement_id": {"type": "string", "description": "placement ID (see placements_create/placements_get output field ID)"},
		"is_active": {"type": "boolean"},` + schemaPlacementFields + `}, "required": ["placement_id"]}`
)

// registerPlacementTools registers the placement domain tools on s, all
// scoped to the authenticated key's context.
func registerPlacementTools(s *mcp.Server, d Deps, contextID string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "placements_list",
		Description: "List all placements for this context (unpaged).",
		InputSchema: json.RawMessage(schemaPlacementsList),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in placementsListIn) (*mcp.CallToolResult, any, error) {
		placements, err := d.BannerUsecase.ListPlacements(contextID, in.IsActive)
		if err != nil {
			return nil, nil, err
		}
		items := make([]bdomain.Placement, 0, len(placements))
		for _, p := range placements {
			items = append(items, *p)
		}
		return nil, placementsListOut{Items: items}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "placements_get",
		Description: "Get a single placement by ID.",
		InputSchema: json.RawMessage(schemaPlacementID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in placementIDIn) (*mcp.CallToolResult, any, error) {
		p, err := d.BannerUsecase.GetPlacement(in.PlacementID)
		if err != nil {
			return nil, nil, err
		}
		return nil, p, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "placements_create",
		Description: "Create a placement (ad slot).",
		InputSchema: json.RawMessage(schemaPlacementCreate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in placementsCreateIn) (*mcp.CallToolResult, any, error) {
		p, err := d.BannerUsecase.CreatePlacement(contextID, &bdomain.CreatePlacementRequest{
			Name:        in.Name,
			Location:    in.Location,
			CSSSelector: in.CSSSelector,
			MaxBanners:  in.MaxBanners,
			Priority:    in.Priority,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, p, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "placements_update",
		Description: "Update a placement by ID.",
		InputSchema: json.RawMessage(schemaPlacementUpdate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in placementsUpdateIn) (*mcp.CallToolResult, any, error) {
		p, err := d.BannerUsecase.UpdatePlacement(contextID, in.PlacementID, &bdomain.UpdatePlacementRequest{
			Name:        in.Name,
			Location:    in.Location,
			CSSSelector: in.CSSSelector,
			MaxBanners:  in.MaxBanners,
			Priority:    in.Priority,
			IsActive:    in.IsActive,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, p, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "placements_delete",
		Description: "Delete a placement by ID.",
		InputSchema: json.RawMessage(schemaPlacementID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in placementIDIn) (*mcp.CallToolResult, any, error) {
		if err := d.BannerUsecase.DeletePlacement(in.PlacementID); err != nil {
			return nil, nil, err
		}
		return nil, placementDeletedOut{Deleted: true}, nil
	})
}

// --- In/Out types ---

type placementsListIn struct {
	IsActive bool `json:"is_active"`
}

type placementsListOut struct {
	Items []bdomain.Placement `json:"items"`
}

type placementIDIn struct {
	PlacementID string `json:"placement_id"`
}

type placementsCreateIn struct {
	Name        string `json:"name"`
	Location    string `json:"location"`
	CSSSelector string `json:"css_selector"`
	MaxBanners  int    `json:"max_banners"`
	Priority    int    `json:"priority"`
}

type placementsUpdateIn struct {
	PlacementID string `json:"placement_id"`
	Name        string `json:"name"`
	Location    string `json:"location"`
	CSSSelector string `json:"css_selector"`
	MaxBanners  int    `json:"max_banners"`
	Priority    int    `json:"priority"`
	IsActive    *bool  `json:"is_active"`
}

type placementDeletedOut struct {
	Deleted bool `json:"deleted"`
}