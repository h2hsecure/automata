package tracking

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	ct "github.com/hekemen/automata/internal/domain/contact"
	geodomain "github.com/hekemen/automata/internal/domain/geo"
	"github.com/hekemen/automata/internal/domain/tracking"
	"github.com/hekemen/automata/pkg/ua"
	"github.com/rs/zerolog/log"
)

// TrackEvent tracks a visitor event and updates visitor metrics. For
// pageview events carrying a visitor ID it also ensures a hidden contact
// exists for the visitor and records a page_visit activity. geoLookup is
// used to resolve the visitor's location offline; a nil lookup disables
// geolocation.
func TrackEvent(contactRepo ct.Repository, geoLookup geodomain.Lookup, trackingRepo tracking.Repository, contextID string, event *tracking.Event) error {
	if contextID == "" {
		return fmt.Errorf("contextID required")
	}

	event.ContextID = contextID

	// Normalize the snippet's "page_view" / compact "pv" sentinels to the
	// canonical event type.
	switch event.Type {
	case "page_view", "pv":
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

	trackPageVisit(contactRepo, geoLookup, contextID, event)

	return nil
}

// TrackEventWithTime is like TrackEvent but uses the provided time for event creation.
func TrackEventWithTime(contactRepo ct.Repository, geoLookup geodomain.Lookup, trackingRepo tracking.Repository, contextID string, event *tracking.Event, now time.Time) error {
	event.CreatedAt = now
	return TrackEvent(contactRepo, geoLookup, trackingRepo, contextID, event)
}

// trackPageVisit ensures a hidden contact exists for the visitor, records a
// page_visit activity on it, and enriches the contact with the visitor's
// user-agent, device, browser, IP, and offline geolocation. The contact row
// is refreshed with last-seen client data on later pageviews; geolocation is
// re-resolved only when the IP changes. Failures are logged and never fail
// tracking.
func trackPageVisit(contactRepo ct.Repository, geoLookup geodomain.Lookup, contextID string, event *tracking.Event) {
	if contactRepo == nil || event.Type != tracking.EventTypePageView || event.VisitorID == "" {
		return
	}

	device, browser := ua.Parse(event.UserAgent)

	contact, err := contactRepo.FindBySourceID(contextID, "visitor", event.VisitorID)
	if err != nil {
		log.Warn().Err(err).Str("visitor_id", event.VisitorID).Msg("track event: find visitor contact")
		return
	}
	if contact == nil {
		contact = &ct.Contact{
			ID:         uuid.New().String(),
			ContextID:  contextID,
			Source:     "visitor",
			SourceID:   event.VisitorID,
			Hidden:     true,
			UserAgent:  event.UserAgent,
			DeviceType: string(device),
			Browser:    string(browser),
			IPAddress:  event.IP,
		}
		applyGeo(contact, geoLookup, event.IP)
		if err := contactRepo.Create(contact); err != nil {
			log.Warn().Err(err).Str("visitor_id", event.VisitorID).Msg("track event: create hidden contact")
			return
		}
	} else if changed := refreshClientInfo(contact, geoLookup, event); changed {
		if err := contactRepo.Update(contact); err != nil {
			log.Warn().Err(err).Str("contact_id", contact.ID).Msg("track event: refresh contact client info")
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

// refreshClientInfo updates a contact's last-seen client fields from an
// event and re-resolves its geolocation only when the IP changed. It
// returns true when any field changed.
func refreshClientInfo(c *ct.Contact, geoLookup geodomain.Lookup, event *tracking.Event) bool {
	device, browser := ua.Parse(event.UserAgent)
	changed := false
	if c.UserAgent != event.UserAgent {
		c.UserAgent = event.UserAgent
		changed = true
	}
	if c.DeviceType != string(device) {
		c.DeviceType = string(device)
		changed = true
	}
	if c.Browser != string(browser) {
		c.Browser = string(browser)
		changed = true
	}
	if c.IPAddress != event.IP {
		c.IPAddress = event.IP
		changed = true
		applyGeo(c, geoLookup, event.IP)
	}
	return changed
}

// applyGeo fills the contact's geo fields from the lookup. A miss (no data
// for the IP, nil lookup, or empty IP) clears the fields and logs a warning
// only when the IP is valid — geolocation is never fatal to tracking.
func applyGeo(c *ct.Contact, geoLookup geodomain.Lookup, ip string) {
	c.GeoCountry = ""
	c.GeoCountryCode = ""
	c.GeoCity = ""
	c.GeoRegion = ""
	if ip == "" || geoLookup == nil {
		return
	}
	if loc, ok := geoLookup.Locate(ip); ok {
		c.GeoCountry = loc.Country
		c.GeoCountryCode = loc.CountryCode
		c.GeoCity = loc.City
		c.GeoRegion = loc.Region
	} else {
		log.Warn().Str("ip", ip).Msg("track event: geo lookup has no data for ip")
	}
}