package tracking

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/tracking"
)

// GetEvents retrieves events for a context with the given filter options.
func GetEvents(repo tracking.Repository, contextID string, opts tracking.EventFilter) ([]*tracking.Event, int64, error) {
	if contextID == "" {
		return nil, 0, fmt.Errorf("contextID required")
	}

	events, total, err := repo.GetEvents(contextID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("get events: %w", err)
	}

	return events, total, nil
}
