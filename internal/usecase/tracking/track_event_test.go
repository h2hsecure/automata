package tracking

import (
	"testing"
	"time"

	ct "github.com/hekemen/automata/internal/domain/contact"
	"github.com/hekemen/automata/internal/domain/tracking"
)

// fakeContactRepo records created contacts and activities.
type fakeContactRepo struct {
	contacts   []*ct.Contact
	activities []ct.Activity
}

func (f *fakeContactRepo) Create(c *ct.Contact) error {
	f.contacts = append(f.contacts, c)
	return nil
}

func (f *fakeContactRepo) GetByID(id string) (*ct.Contact, error) {
	for _, c := range f.contacts {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}

func (f *fakeContactRepo) List(offset, limit int, filters ct.FilterOptions) ([]*ct.Contact, error) {
	return f.contacts, nil
}

func (f *fakeContactRepo) Update(c *ct.Contact) error { return nil }

func (f *fakeContactRepo) Delete(id string) error { return nil }

func (f *fakeContactRepo) FindByEmail(contextID, email string) (*ct.Contact, error) {
	return nil, nil
}

func (f *fakeContactRepo) Merge(keepID, mergeIntoID string) error { return nil }

func (f *fakeContactRepo) CreateActivity(a *ct.Activity) error {
	f.activities = append(f.activities, *a)
	return nil
}

func (f *fakeContactRepo) GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]ct.Activity, int64, error) {
	return f.activities, int64(len(f.activities)), nil
}

func (f *fakeContactRepo) GetActivityCount(contactID string) (int64, error) {
	return int64(len(f.activities)), nil
}

func (f *fakeContactRepo) CountByContext(contextID string) (int64, error) {
	return int64(len(f.contacts)), nil
}

func (f *fakeContactRepo) CountDistinctContactsByActivity(contextID, activityType string, since *time.Time) (int, error) {
	return 0, nil
}

func (f *fakeContactRepo) FindBySourceID(contextID, source, sourceID string) (*ct.Contact, error) {
	for _, c := range f.contacts {
		if c.ContextID == contextID && c.Source == source && c.SourceID == sourceID {
			return c, nil
		}
	}
	return nil, nil
}

func (f *fakeContactRepo) Count(contextID string, filters ct.FilterOptions) (int64, error) {
	return int64(len(f.contacts)), nil
}

func newPageViewEvent(visitorID string) *tracking.Event {
	return &tracking.Event{
		VisitorID: visitorID,
		Type:      tracking.EventTypePageView,
		URL:       "/home",
		Title:     "Home",
		Referrer:  "google.com",
		UserAgent: "Mozilla/5.0 test",
		IP:        "1.2.3.4",
		CreatedAt: time.Now(),
	}
}

func TestTrackEvent_HiddenContactAndActivity(t *testing.T) {
	cr := &fakeContactRepo{}
	tr := &fakeDashRepo{}

	if err := TrackEvent(cr, tr, "ctx-1", newPageViewEvent("vid-1")); err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}
	if len(tr.events) != 1 || len(tr.visitors) != 1 {
		t.Fatalf("event/visitor recorded = (%d, %d), want (1, 1)", len(tr.events), len(tr.visitors))
	}
	if len(cr.contacts) != 1 {
		t.Fatalf("contacts = %d, want 1", len(cr.contacts))
	}
	c := cr.contacts[0]
	if !c.Hidden || c.Source != "visitor" || c.SourceID != "vid-1" || c.ContextID != "ctx-1" {
		t.Errorf("contact = %+v, want hidden visitor contact for vid-1", c)
	}
	if len(cr.activities) != 1 {
		t.Fatalf("activities = %d, want 1", len(cr.activities))
	}
	a := cr.activities[0]
	if a.Type != ct.ActivityPageVisit || a.ContactID != c.ID || a.ContextID != "ctx-1" {
		t.Errorf("activity = %+v, want page_visit on the hidden contact", a)
	}
	wantData := map[string]interface{}{
		"url":        "/home",
		"title":      "Home",
		"referrer":   "google.com",
		"ip":         "1.2.3.4",
		"user_agent": "Mozilla/5.0 test",
		"visitor_id": "vid-1",
	}
	for k, v := range wantData {
		if a.Data[k] != v {
			t.Errorf("activity data[%q] = %v, want %v", k, a.Data[k], v)
		}
	}

	// Second pageview for the same visitor: dedupe contact, add a second activity.
	if err := TrackEvent(cr, tr, "ctx-1", newPageViewEvent("vid-1")); err != nil {
		t.Fatalf("TrackEvent (2nd): %v", err)
	}
	if len(cr.contacts) != 1 {
		t.Errorf("contacts after 2nd call = %d, want 1", len(cr.contacts))
	}
	if len(cr.activities) != 2 {
		t.Errorf("activities after 2nd call = %d, want 2", len(cr.activities))
	}
}

func TestTrackEvent_NormalizesPageViewSentinel(t *testing.T) {
	cr := &fakeContactRepo{}
	tr := &fakeDashRepo{}

	event := newPageViewEvent("vid-2")
	event.Type = "page_view"
	if err := TrackEvent(cr, tr, "ctx-1", event); err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}
	if event.Type != tracking.EventTypePageView {
		t.Errorf("event.Type = %q, want %q", event.Type, tracking.EventTypePageView)
	}
	if len(cr.contacts) != 1 || len(cr.activities) != 1 {
		t.Errorf("contacts/activities = (%d, %d), want (1, 1)", len(cr.contacts), len(cr.activities))
	}
}

func TestTrackEvent_NormalizesCompactPVSentinel(t *testing.T) {
	cr := &fakeContactRepo{}
	tr := &fakeDashRepo{}

	event := newPageViewEvent("vid-5")
	event.Type = "pv"
	if err := TrackEvent(cr, tr, "ctx-1", event); err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}
	if event.Type != tracking.EventTypePageView {
		t.Errorf("event.Type = %q, want %q", event.Type, tracking.EventTypePageView)
	}
	if len(cr.contacts) != 1 || len(cr.activities) != 1 {
		t.Errorf("contacts/activities = (%d, %d), want (1, 1)", len(cr.contacts), len(cr.activities))
	}
}

func TestTrackEvent_NoContactWithoutVisitorID(t *testing.T) {
	cr := &fakeContactRepo{}
	tr := &fakeDashRepo{}

	if err := TrackEvent(cr, tr, "ctx-1", newPageViewEvent("")); err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}
	if len(cr.contacts) != 0 || len(cr.activities) != 0 {
		t.Errorf("contacts/activities = (%d, %d), want (0, 0)", len(cr.contacts), len(cr.activities))
	}
	if len(tr.events) != 1 {
		t.Errorf("events = %d, want 1 (tracking itself must succeed)", len(tr.events))
	}
}

func TestTrackEvent_NoContactForNonPageView(t *testing.T) {
	cr := &fakeContactRepo{}
	tr := &fakeDashRepo{}

	event := &tracking.Event{
		VisitorID: "vid-3",
		Type:      tracking.EventTypeEvent,
		EventName: "cta_click",
		CreatedAt: time.Now(),
	}
	if err := TrackEvent(cr, tr, "ctx-1", event); err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}
	if len(cr.contacts) != 0 || len(cr.activities) != 0 {
		t.Errorf("contacts/activities = (%d, %d), want (0, 0)", len(cr.contacts), len(cr.activities))
	}
}

func TestTrackEvent_NilContactRepoSkipsContactPath(t *testing.T) {
	tr := &fakeDashRepo{}

	if err := TrackEvent(nil, tr, "ctx-1", newPageViewEvent("vid-4")); err != nil {
		t.Fatalf("TrackEvent: %v", err)
	}
	if len(tr.events) != 1 {
		t.Errorf("events = %d, want 1", len(tr.events))
	}
}