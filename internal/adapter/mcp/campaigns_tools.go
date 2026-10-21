package mcp

import (
	"context"
	"encoding/json"
	"time"

	bdomain "github.com/hekemen/automata/internal/domain/banner"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input schemas are stated explicitly (the SDK's inference marks every
// struct field as required). campaign_id is the campaign's database ID,
// as returned in the output.
const (
	schemaCampaignID = `{"type": "object", "properties": {
		"campaign_id": {"type": "string", "description": "campaign ID (see campaigns_create/campaigns_get output field ID)"}
	}, "required": ["campaign_id"]}`

	schemaCampaignsList = `{"type": "object", "properties": {
		"is_active": {"type": "boolean", "description": "when true, only active campaigns"}
	}, "required": []}`

	schemaCampaignFields = `
		"name": {"type": "string"},
		"description": {"type": "string"},
		"start_date": {"type": "string", "description": "RFC3339 timestamp"},
		"end_date": {"type": "string", "description": "RFC3339 timestamp"},
		"target_url": {"type": "string"},
		"tracking_code": {"type": "string"}`

	schemaCampaignCreate = `{"type": "object", "properties": {` + schemaCampaignFields + `}, "required": ["name"]}`

	schemaCampaignUpdate = `{"type": "object", "properties": {
		"campaign_id": {"type": "string", "description": "campaign ID (see campaigns_create/campaigns_get output field ID)"},
		"is_active": {"type": "boolean"},` + schemaCampaignFields + `}, "required": ["campaign_id"]}`
)

// registerCampaignTools registers the campaign domain tools on s, all
// scoped to the authenticated key's context.
func registerCampaignTools(s *mcp.Server, d Deps, contextID string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "campaigns_list",
		Description: "List all campaigns for this context (unpaged).",
		InputSchema: json.RawMessage(schemaCampaignsList),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in campaignsListIn) (*mcp.CallToolResult, any, error) {
		campaigns, err := d.BannerUsecase.ListCampaigns(contextID, in.IsActive)
		if err != nil {
			return nil, nil, err
		}
		items := make([]bdomain.Campaign, 0, len(campaigns))
		for _, c := range campaigns {
			items = append(items, *c)
		}
		return nil, campaignsListOut{Items: items}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "campaigns_get",
		Description: "Get a single campaign by ID.",
		InputSchema: json.RawMessage(schemaCampaignID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in campaignIDIn) (*mcp.CallToolResult, any, error) {
		c, err := d.BannerUsecase.GetCampaign(contextID, in.CampaignID)
		if err != nil {
			return nil, nil, err
		}
		return nil, c, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "campaigns_create",
		Description: "Create a campaign.",
		InputSchema: json.RawMessage(schemaCampaignCreate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in campaignsCreateIn) (*mcp.CallToolResult, any, error) {
		c, err := d.BannerUsecase.CreateCampaign(contextID, &bdomain.CreateCampaignRequest{
			Name:         in.Name,
			Description:  in.Description,
			StartDate:    in.StartDate,
			EndDate:      in.EndDate,
			TargetURL:    in.TargetURL,
			TrackingCode: in.TrackingCode,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, c, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "campaigns_update",
		Description: "Update a campaign by ID.",
		InputSchema: json.RawMessage(schemaCampaignUpdate),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in campaignsUpdateIn) (*mcp.CallToolResult, any, error) {
		c, err := d.BannerUsecase.UpdateCampaign(contextID, in.CampaignID, &bdomain.UpdateCampaignRequest{
			Name:         in.Name,
			Description:  in.Description,
			StartDate:    in.StartDate,
			EndDate:      in.EndDate,
			TargetURL:    in.TargetURL,
			TrackingCode: in.TrackingCode,
			IsActive:     in.IsActive,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, c, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "campaigns_delete",
		Description: "Delete a campaign by ID.",
		InputSchema: json.RawMessage(schemaCampaignID),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in campaignIDIn) (*mcp.CallToolResult, any, error) {
		if err := d.BannerUsecase.DeleteCampaign(contextID, in.CampaignID); err != nil {
			return nil, nil, err
		}
		return nil, campaignDeletedOut{Deleted: true}, nil
	})
}

// --- In/Out types ---

type campaignsListIn struct {
	IsActive bool `json:"is_active"`
}

type campaignsListOut struct {
	Items []bdomain.Campaign `json:"items"`
}

type campaignIDIn struct {
	CampaignID string `json:"campaign_id"`
}

type campaignsCreateIn struct {
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	StartDate    *time.Time `json:"start_date"`
	EndDate      *time.Time `json:"end_date"`
	TargetURL    string     `json:"target_url"`
	TrackingCode string     `json:"tracking_code"`
}

type campaignsUpdateIn struct {
	CampaignID   string     `json:"campaign_id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	StartDate    *time.Time `json:"start_date"`
	EndDate      *time.Time `json:"end_date"`
	TargetURL    string     `json:"target_url"`
	TrackingCode string     `json:"tracking_code"`
	IsActive     *bool      `json:"is_active"`
}

type campaignDeletedOut struct {
	Deleted bool `json:"deleted"`
}