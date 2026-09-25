package tracking

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/tracking"
)

// VisitorDetailResponse holds a visitor's profile with recent events.
type VisitorDetailResponse struct {
	Visitor      *tracking.Visitor `json:"visitor"`
	RecentEvents []*tracking.Event `json:"recent_events"`
}

// GetVisitorDetail fetches a visitor by ID and their most recent events.
func GetVisitorDetail(repo tracking.Repository, contextID, visitorID string, limit int) (*VisitorDetailResponse, error) {
	if contextID == "" {
		return nil, fmt.Errorf("contextID required")
	}
	if visitorID == "" {
		return nil, fmt.Errorf("visitorID required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	visitor, err := repo.GetVisitorByID(nil, contextID, visitorID)
	if err != nil {
		return nil, fmt.Errorf("get visitor: %w", err)
	}

	// Fetch recent events
	events, _, err := repo.GetVisitorEvents(nil, contextID, visitorID, tracking.GetVisitorEventsOpts{
		Limit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("get visitor events: %w", err)
	}

	return &VisitorDetailResponse{
		Visitor:      visitor,
		RecentEvents: events,
	}, nil
}
