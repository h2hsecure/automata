package tracking

import (
	"fmt"
	"time"

	"github.com/hekemen/automata/internal/domain/tracking"
)

// TrackEvent tracks a visitor event and updates visitor metrics.
func TrackEvent(repo tracking.Repository, tenantID string, event *tracking.Event) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID required")
	}

	event.TenantID = tenantID

	if err := repo.CreateEvent(event); err != nil {
		return fmt.Errorf("create event: %w", err)
	}

	visitor := &tracking.Visitor{
		TenantID:    tenantID,
		CookieValue: event.VisitorID,
		PageViews:   1,
		FirstSeen:   event.CreatedAt,
		LastSeen:    event.CreatedAt,
	}

	if err := repo.UpsertVisitor(visitor); err != nil {
		return fmt.Errorf("upsert visitor: %w", err)
	}

	return nil
}

// TrackEventWithTime is like TrackEvent but uses the provided time for event creation.
func TrackEventWithTime(repo tracking.Repository, tenantID string, event *tracking.Event, now time.Time) error {
	if tenantID == "" {
		return fmt.Errorf("tenantID required")
	}

	event.TenantID = tenantID
	event.CreatedAt = now

	if err := repo.CreateEvent(event); err != nil {
		return fmt.Errorf("create event: %w", err)
	}

	visitor := &tracking.Visitor{
		TenantID:    tenantID,
		CookieValue: event.VisitorID,
		PageViews:   1,
		FirstSeen:   now,
		LastSeen:    now,
	}

	if err := repo.UpsertVisitor(visitor); err != nil {
		return fmt.Errorf("upsert visitor: %w", err)
	}

	return nil
}
