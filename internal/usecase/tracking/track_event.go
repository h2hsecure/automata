package tracking

import (
	"fmt"
	"time"

	"github.com/hekemen/automata/internal/domain/tracking"
)

// TrackEvent tracks a visitor event and updates visitor metrics.
func TrackEvent(repo tracking.Repository, contextID string, event *tracking.Event) error {
	if contextID == "" {
		return fmt.Errorf("contextID required")
	}

	event.ContextID = contextID

	// Normalize the snippet's "page_view" sentinel to the canonical event type.
	if event.Type == "page_view" {
		event.Type = tracking.EventTypePageView
	}

	if err := repo.CreateEvent(event); err != nil {
		return fmt.Errorf("create event: %w", err)
	}

	visitor := &tracking.Visitor{
		ContextID:    contextID,
		CookieValue: event.VisitorID,
		PageViews:   1,
		FirstSeen:   event.CreatedAt,
		LastSeen:    event.CreatedAt,
		LastIP:      event.IP,
	}

	if err := repo.UpsertVisitor(visitor); err != nil {
		return fmt.Errorf("upsert visitor: %w", err)
	}

	return nil
}

// TrackEventWithTime is like TrackEvent but uses the provided time for event creation.
func TrackEventWithTime(repo tracking.Repository, contextID string, event *tracking.Event, now time.Time) error {
	if contextID == "" {
		return fmt.Errorf("contextID required")
	}

	event.ContextID = contextID
	event.CreatedAt = now

	// Normalize the snippet's "page_view" sentinel to the canonical event type.
	if event.Type == "page_view" {
		event.Type = tracking.EventTypePageView
	}

	if err := repo.CreateEvent(event); err != nil {
		return fmt.Errorf("create event: %w", err)
	}

	visitor := &tracking.Visitor{
		ContextID:    contextID,
		CookieValue: event.VisitorID,
		PageViews:   1,
		FirstSeen:   now,
		LastSeen:    now,
		LastIP:      event.IP,
	}

	if err := repo.UpsertVisitor(visitor); err != nil {
		return fmt.Errorf("upsert visitor: %w", err)
	}

	return nil
}
