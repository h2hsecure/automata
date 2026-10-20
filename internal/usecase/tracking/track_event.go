package tracking

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	ct "github.com/hekemen/automata/internal/domain/contact"
	"github.com/hekemen/automata/internal/domain/tracking"
	"github.com/rs/zerolog/log"
)

// TrackEvent tracks a visitor event and updates visitor metrics. For
// pageview events carrying a visitor ID it also ensures a hidden contact
// exists for the visitor and records a page_visit activity.
func TrackEvent(contactRepo ct.Repository, trackingRepo tracking.Repository, contextID string, event *tracking.Event) error {
	if contextID == "" {
		return fmt.Errorf("contextID required")
	}

	event.ContextID = contextID

	// Normalize the snippet's "page_view" sentinel to the canonical event type.
	if event.Type == "page_view" {
		event.Type = tracking.EventTypePageView
	}

	if err := trackingRepo.CreateEvent(event); err != nil {
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

	if err := trackingRepo.UpsertVisitor(visitor); err != nil {
		return fmt.Errorf("upsert visitor: %w", err)
	}

	trackPageVisit(contactRepo, contextID, event)

	return nil
}

// TrackEventWithTime is like TrackEvent but uses the provided time for event creation.
func TrackEventWithTime(contactRepo ct.Repository, trackingRepo tracking.Repository, contextID string, event *tracking.Event, now time.Time) error {
	event.CreatedAt = now
	return TrackEvent(contactRepo, trackingRepo, contextID, event)
}

// trackPageVisit ensures a hidden contact exists for the visitor and records
// a page_visit activity on it. Failures are logged and never fail tracking.
func trackPageVisit(contactRepo ct.Repository, contextID string, event *tracking.Event) {
	if contactRepo == nil || event.Type != tracking.EventTypePageView || event.VisitorID == "" {
		return
	}

	contact, err := contactRepo.FindBySourceID(contextID, "visitor", event.VisitorID)
	if err != nil {
		log.Warn().Err(err).Str("visitor_id", event.VisitorID).Msg("track event: find visitor contact")
		return
	}
	if contact == nil {
		contact = &ct.Contact{
			ID:       uuid.New().String(),
			ContextID: contextID,
			Source:   "visitor",
			SourceID: event.VisitorID,
			Hidden:   true,
		}
		if err := contactRepo.Create(contact); err != nil {
			log.Warn().Err(err).Str("visitor_id", event.VisitorID).Msg("track event: create hidden contact")
			return
		}
	}

	if err := contactRepo.CreateActivity(&ct.Activity{
		ID:        uuid.New().String(),
		ContactID: contact.ID,
		ContextID: contextID,
		Type:      ct.ActivityPageVisit,
		Data: map[string]interface{}{
			"url":        event.URL,
			"title":      event.Title,
			"referrer":   event.Referrer,
			"ip":         event.IP,
			"user_agent": event.UserAgent,
			"visitor_id": event.VisitorID,
		},
	}); err != nil {
		log.Warn().Err(err).Str("contact_id", contact.ID).Msg("track event: create page_visit activity")
	}
}