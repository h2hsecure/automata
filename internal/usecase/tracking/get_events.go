package tracking

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/tracking"
)

// GetEvents retrieves events for a tenant with the given filter options.
func GetEvents(repo tracking.Repository, tenantID string, opts tracking.EventFilter) ([]*tracking.Event, int64, error) {
	if tenantID == "" {
		return nil, 0, fmt.Errorf("tenantID required")
	}

	events, total, err := repo.GetEvents(tenantID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("get events: %w", err)
	}

	return events, total, nil
}
