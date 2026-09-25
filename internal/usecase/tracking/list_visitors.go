package tracking

import (
	"fmt"

	"github.com/hekemen/automata/internal/domain/tracking"
)

// ListVisitors retrieves a paginated list of visitors for a context.
func ListVisitors(repo tracking.Repository, contextID string, opts tracking.ListVisitorsOpts) ([]*tracking.Visitor, int64, error) {
	if contextID == "" {
		return nil, 0, fmt.Errorf("contextID required")
	}

	// Validate sort field
	validSorts := map[string]bool{"first_seen": true, "last_seen": true, "page_views": true}
	if opts.Sort != "" && !validSorts[opts.Sort] {
		return nil, 0, fmt.Errorf("invalid sort field: %s", opts.Sort)
	}

	// Validate limit
	if opts.Limit < 1 || opts.Limit > 100 {
		return nil, 0, fmt.Errorf("limit must be between 1 and 100")
	}

	visitors, total, err := repo.ListVisitors(nil, contextID, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("list visitors: %w", err)
	}

	return visitors, total, nil
}
