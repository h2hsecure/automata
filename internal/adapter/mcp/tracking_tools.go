package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	tdomain "github.com/hekemen/automata/internal/domain/tracking"
	tuc "github.com/hekemen/automata/internal/usecase/tracking"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input schemas are stated explicitly (the SDK's inference marks every
// struct field as required). visitor_id is the visitor's database ID, as
// returned in the tracking_visitors output.
const (
	schemaTrackingDashboard = `{
		"type": "object",
		"properties": {
			"start": {"type": "string", "description": "RFC3339 start of the window; omitted = no lower bound"},
			"end": {"type": "string", "description": "RFC3339 end of the window; omitted = no upper bound"},
			"active_min": {"type": "integer", "description": "minutes of activity for the active-visitor count, default 30"},
			"top_n": {"type": "integer", "description": "number of top pages/referrers, default 10"}
		},
		"required": []
	}`

	schemaTrackingEvents = `{
		"type": "object",
		"properties": {
			"type": {"type": "string", "description": "pageview or event; omitted = all"},
			"event_name": {"type": "string", "description": "filter by event name"},
			"url": {"type": "string", "description": "filter by URL"},
			"page": {"type": "integer", "description": "1-based page number, default 1"},
			"limit": {"type": "integer", "description": "page size, default 20, max 100"}
		},
		"required": []
	}`

	schemaTrackingVisitors = `{
		"type": "object",
		"properties": {
			"sort": {"type": "string", "description": "first_seen, last_seen, or page_views; omitted = last_seen"},
			"page": {"type": "integer", "description": "1-based page number, default 1"},
			"limit": {"type": "integer", "description": "page size, default 20, max 100"}
		},
		"required": []
	}`

	schemaVisitorID = `{"type": "object", "properties": {
		"visitor_id": {"type": "string", "description": "visitor ID (see tracking_visitors output field ID)"}
	}, "required": ["visitor_id"]}`

	schemaTrackingVisitorEvents = `{
		"type": "object",
		"properties": {
			"visitor_id": {"type": "string", "description": "visitor ID (see tracking_visitors output field ID)"},
			"type": {"type": "string", "description": "pageview or event; omitted = all"},
			"page": {"type": "integer", "description": "1-based page number, default 1"},
			"limit": {"type": "integer", "description": "page size, default 20, max 100"}
		},
		"required": ["visitor_id"]
	}`

	schemaTrackingVisitorGet = `{
		"type": "object",
		"properties": {
			"visitor_id": {"type": "string", "description": "visitor ID (see tracking_visitors output field ID)"},
			"limit": {"type": "integer", "description": "recent events to include, default 20, max 50"}
		},
		"required": ["visitor_id"]
	}`

	schemaTrackingTrack = `{
		"type": "object",
		"properties": {
			"type": {"type": "string", "description": "pageview or event (page_view/pv also accepted for pageviews)"},
			"visitor_id": {"type": "string", "description": "visitor identifier (cookie value)"},
			"url": {"type": "string", "description": "page URL (pageviews)"},
			"title": {"type": "string", "description": "page title (pageviews)"},
			"referrer": {"type": "string", "description": "referrer URL (pageviews)"},
			"event_name": {"type": "string", "description": "event name (events)"},
			"properties": {"type": "object", "additionalProperties": true, "description": "event properties (events)"},
			"user_agent": {"type": "string", "description": "client user agent"},
			"ip": {"type": "string", "description": "client IP address"}
		},
		"required": ["type"]
	}`
)

// registerTrackingTools registers the analytics/tracking tools on s, all
// scoped to the authenticated key's context.
func registerTrackingTools(s *mcp.Server, d Deps, contextID string) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "tracking_dashboard",
		Description: "Dashboard metrics for this context: visitors, page views, top pages/referrers, device and browser breakdowns.",
		InputSchema: json.RawMessage(schemaTrackingDashboard),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in dashboardIn) (*mcp.CallToolResult, any, error) {
		var start, end time.Time
		if in.Start != "" {
			var err error
			start, err = time.Parse(time.RFC3339, in.Start)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid start (want RFC3339): %w", err)
			}
		}
		if in.End != "" {
			var err error
			end, err = time.Parse(time.RFC3339, in.End)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid end (want RFC3339): %w", err)
			}
		}
		metrics, err := tuc.GetDashboard(d.TrackingRepo, contextID, tuc.GetDashboardOptions{
			Start:     start,
			End:       end,
			ActiveMin: in.ActiveMin,
			TopN:      in.TopN,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, metrics, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "tracking_events",
		Description: "List tracking events for this context, paginated.",
		InputSchema: json.RawMessage(schemaTrackingEvents),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in trackingEventsIn) (*mcp.CallToolResult, any, error) {
		offset, page, limit := clampPage(in.Page, in.Limit)
		events, total, err := tuc.GetEvents(d.TrackingRepo, contextID, tdomain.EventFilter{
			Type:      tdomain.EventType(in.Type),
			EventName: in.EventName,
			URL:       in.URL,
			Offset:    offset,
			Limit:     limit,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, trackingEventsOut{
			Items:    events,
			pageMeta: pageMeta{Total: total, Page: page, Limit: limit, HasMore: int64(page*limit) < total},
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "tracking_visitors",
		Description: "List visitors for this context, paginated.",
		InputSchema: json.RawMessage(schemaTrackingVisitors),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in trackingVisitorsIn) (*mcp.CallToolResult, any, error) {
		offset, page, limit := clampPage(in.Page, in.Limit)
		visitors, total, err := tuc.ListVisitors(d.TrackingRepo, contextID, tdomain.ListVisitorsOpts{
			Offset: offset,
			Limit:  limit,
			Sort:   in.Sort,
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, trackingVisitorsOut{
			Items:    visitors,
			pageMeta: pageMeta{Total: total, Page: page, Limit: limit, HasMore: int64(page*limit) < total},
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "tracking_visitor_events",
		Description: "List a visitor's tracking events, paginated.",
		InputSchema: json.RawMessage(schemaTrackingVisitorEvents),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in trackingVisitorEventsIn) (*mcp.CallToolResult, any, error) {
		offset, page, limit := clampPage(in.Page, in.Limit)
		events, total, err := tuc.GetVisitorEvents(d.TrackingRepo, contextID, in.VisitorID, tdomain.GetVisitorEventsOpts{
			Offset: offset,
			Limit:  limit,
			Type:   tdomain.EventType(in.Type),
		})
		if err != nil {
			return nil, nil, err
		}
		return nil, trackingEventsOut{
			Items:    events,
			pageMeta: pageMeta{Total: total, Page: page, Limit: limit, HasMore: int64(page*limit) < total},
		}, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "tracking_visitor_get",
		Description: "Get a visitor profile with their most recent events.",
		InputSchema: json.RawMessage(schemaTrackingVisitorGet),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in trackingVisitorGetIn) (*mcp.CallToolResult, any, error) {
		detail, err := tuc.GetVisitorDetail(d.TrackingRepo, contextID, in.VisitorID, in.Limit)
		if err != nil {
			return nil, nil, err
		}
		return nil, detail, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "tracking_track",
		Description: "Record a tracking event (pageview or event) for a visitor in this context.",
		InputSchema: json.RawMessage(schemaTrackingTrack),
	}, func(_ context.Context, _ *mcp.CallToolRequest, in trackingTrackIn) (*mcp.CallToolResult, any, error) {
		event := &tdomain.Event{
			ID:         uuid.New().String(),
			VisitorID:  in.VisitorID,
			Type:       tdomain.EventType(in.Type),
			URL:        in.URL,
			Title:      in.Title,
			Referrer:   in.Referrer,
			EventName:  in.EventName,
			Properties: in.Properties,
			UserAgent:  in.UserAgent,
			IP:         in.IP,
			CreatedAt:  time.Now(),
		}
		if err := tuc.TrackEvent(d.ContactRepo, nil, d.TrackingRepo, contextID, event); err != nil {
			return nil, trackingTrackedOut{}, err
		}
		return nil, trackingTrackedOut{Tracked: true, EventID: event.ID}, nil
	})
}

// --- In/Out types ---

type dashboardIn struct {
	Start     string `json:"start"`
	End       string `json:"end"`
	ActiveMin int    `json:"active_min"`
	TopN      int    `json:"top_n"`
}

type trackingEventsIn struct {
	Type      string `json:"type"`
	EventName string `json:"event_name"`
	URL       string `json:"url"`
	Page      int    `json:"page"`
	Limit     int    `json:"limit"`
}

type trackingEventsOut struct {
	Items    []*tdomain.Event `json:"items"`
	pageMeta
}

type trackingVisitorsIn struct {
	Sort  string `json:"sort"`
	Page  int    `json:"page"`
	Limit int    `json:"limit"`
}

type trackingVisitorsOut struct {
	Items    []*tdomain.Visitor `json:"items"`
	pageMeta
}

type trackingVisitorEventsIn struct {
	VisitorID string `json:"visitor_id"`
	Type      string `json:"type"`
	Page      int    `json:"page"`
	Limit     int    `json:"limit"`
}

type trackingVisitorGetIn struct {
	VisitorID string `json:"visitor_id"`
	Limit     int    `json:"limit"`
}

type trackingTrackIn struct {
	Type       string                 `json:"type"`
	VisitorID  string                 `json:"visitor_id"`
	URL        string                 `json:"url"`
	Title      string                 `json:"title"`
	Referrer   string                 `json:"referrer"`
	EventName  string                 `json:"event_name"`
	Properties map[string]interface{} `json:"properties"`
	UserAgent  string                 `json:"user_agent"`
	IP         string                 `json:"ip"`
}

type trackingTrackedOut struct {
	Tracked bool   `json:"tracked"`
	EventID string `json:"event_id"`
}