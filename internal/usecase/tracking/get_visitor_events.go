package tracking

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/tracking"
)

// GetVisitorEvents retrieves a paginated list of events for a specific visitor.
func GetVisitorEvents(repo tracking.Repository, contextID, visitorID string, opts tracking.GetVisitorEventsOpts) ([]*tracking.Event, int64, error) {
	if contextID == "" {
		return nil, 0, fmt.Errorf("contextID required")
	}
	if visitorID == "" {
		return nil, 0, fmt.Errorf("visitorID required")
	}

	// Validate type filter
	if opts.Type != "" && opts.Type != tracking.EventTypePageView && opts.Type != tracking.EventTypeEvent {
		return nil, 0, fmt.Errorf("invalid type value: %s", opts.Type)
	}

	// Validate limit
	if opts.Limit < 1 || opts.Limit > 100 {
		return nil, 0, fmt.Errorf("limit must be between 1 and 100")
	}

	events, total, err := repo.GetVisitorEvents(nil, contextID, visitorID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("get visitor events: %w", err)
	}

	return events, total, nil
}
