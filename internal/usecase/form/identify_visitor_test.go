package form

import (
	"context"
	"fmt"
	"testing"

	ct "github.com/h2hsecure/automata/internal/domain/contact"
	tracking "github.com/h2hsecure/automata/internal/domain/tracking"
)

type fakeContactRepo struct {
	contacts    []*ct.Contact
	updateCalls int
	mergeCalls  [][2]string
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

func (f *fakeContactRepo) Update(c *ct.Contact) error {
	f.updateCalls++
	return nil
}

func (f *fakeContactRepo) Delete(id string) error { return nil }

func (f *fakeContactRepo) FindByEmail(contextID, email string) (*ct.Contact, error) {
	for _, c := range f.contacts {
		if c.ContextID == contextID && c.Email != nil && *c.Email == email {
			return c, nil
		}
	}
	// Mirror the real repo: CreateContact relies on a "contact not found"
	// error (not a nil result) to distinguish miss from duplicate.
	return nil, fmt.Errorf("contact not found")
}

func (f *fakeContactRepo) Merge(keepID, mergeIntoID string) error {
	f.mergeCalls = append(f.mergeCalls, [2]string{keepID, mergeIntoID})
	return nil
}

func (f *fakeContactRepo) CreateActivity(a *ct.Activity) error { return nil }

func (f *fakeContactRepo) GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]ct.Activity, int64, error) {
	return nil, 0, nil
}

func (f *fakeContactRepo) GetActivityCount(contactID string) (int64, error) {
	return 0, nil
}

func (f *fakeContactRepo) CountByContext(contextID string) (int64, error) {
	return int64(len(f.contacts)), nil
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

type fakeTrackRepo struct {
	setCalls      [][3]string // contextID, visitorID, contactID
	reassignCalls [][3]string // contextID, oldContactID, newContactID
}

func (f *fakeTrackRepo) CreateEvent(e *tracking.Event) error { return nil }

func (f *fakeTrackRepo) CreateEventsBatch(events []*tracking.Event) error { return nil }

func (f *fakeTrackRepo) GetVisitor(contextID, visitorID string) (*tracking.Visitor, error) {
	return nil, nil
}

func (f *fakeTrackRepo) UpsertVisitor(v *tracking.Visitor) error { return nil }

func (f *fakeTrackRepo) GetDashboardMetrics(contextID string, dateRange tracking.DateRange) (*tracking.DashboardMetrics, error) {
	return nil, nil
}

func (f *fakeTrackRepo) GetEvents(contextID string, opts tracking.EventFilter) ([]*tracking.Event, int64, error) {
	return nil, 0, nil
}

func (f *fakeTrackRepo) GetTopPages(contextID string, dateRange tracking.DateRange, limit int) ([]tracking.PageViewCount, error) {
	return nil, nil
}

func (f *fakeTrackRepo) GetTopReferrers(contextID string, dateRange tracking.DateRange, limit int) ([]tracking.ReferrerCount, error) {
	return nil, nil
}

func (f *fakeTrackRepo) GetUAGroups(contextID string, dateRange tracking.DateRange) ([]tracking.UACount, error) {
	return nil, nil
}

func (f *fakeTrackRepo) GetActiveVisitors(contextID string, minutes int) (int64, error) {
	return 0, nil
}

func (f *fakeTrackRepo) PurgeOldEvents(contextID string, retentionDays int) (int64, error) {
	return 0, nil
}

func (f *fakeTrackRepo) ListVisitors(ctx context.Context, contextID string, opts tracking.ListVisitorsOpts) ([]*tracking.Visitor, int64, error) {
	return nil, 0, nil
}

func (f *fakeTrackRepo) GetVisitorByID(ctx context.Context, contextID, visitorID string) (*tracking.Visitor, error) {
	return nil, nil
}

func (f *fakeTrackRepo) GetVisitorEvents(ctx context.Context, contextID, visitorID string, opts tracking.GetVisitorEventsOpts) ([]*tracking.Event, int64, error) {
	return nil, 0, nil
}

func (f *fakeTrackRepo) SetVisitorContactID(ctx context.Context, contextID, visitorID, contactID string) error {
	f.setCalls = append(f.setCalls, [3]string{contextID, visitorID, contactID})
	return nil
}

func (f *fakeTrackRepo) ReassignVisitorContactID(contextID, oldContactID, newContactID string) error {
	f.reassignCalls = append(f.reassignCalls, [3]string{contextID, oldContactID, newContactID})
	return nil
}

func TestIdentifyVisitor_UpliftsHiddenContact(t *testing.T) {
	email := "ann@example.com"
	hidden := &ct.Contact{
		ID: "c-hidden", ContextID: "ctx-1",
		Source: "visitor", SourceID: "vid-1", Hidden: true,
	}
	cr := &fakeContactRepo{contacts: []*ct.Contact{hidden}}
	tr := &fakeTrackRepo{}

	c, err := IdentifyVisitor(cr, tr, "ctx-1", "signup", "vid-1", map[string]interface{}{
		"email":      email,
		"first_name": "Ann",
		"last_name":  "Lee",
		"company":    "ACME",
	})
	if err != nil {
		t.Fatalf("IdentifyVisitor: %v", err)
	}
	if c.ID != "c-hidden" {
		t.Fatalf("returned contact = %q, want the uplifted hidden contact c-hidden", c.ID)
	}
	if c.Email == nil || *c.Email != email {
		t.Errorf("email = %v, want %q", c.Email, email)
	}
	if c.Hidden {
		t.Errorf("Hidden = true, want false after uplift")
	}
	if c.Source != "form:signup" || c.SourceID != "vid-1" {
		t.Errorf("source = %q/%q, want form:signup/vid-1", c.Source, c.SourceID)
	}
	if c.FirstName != "Ann" || c.LastName != "Lee" || c.Company != "ACME" {
		t.Errorf("fields = (%q, %q, %q), want (Ann, Lee, ACME)", c.FirstName, c.LastName, c.Company)
	}
	if cr.updateCalls != 1 {
		t.Errorf("update calls = %d, want 1", cr.updateCalls)
	}
	if len(tr.setCalls) != 0 || len(tr.reassignCalls) != 0 {
		t.Errorf("tracking link calls = set %d / reassign %d, want 0/0", len(tr.setCalls), len(tr.reassignCalls))
	}
}

func TestIdentifyVisitor_UpliftCollisionMerges(t *testing.T) {
	email := "ann@example.com"
	existing := &ct.Contact{ID: "c-existing", ContextID: "ctx-1", Email: &email}
	hidden := &ct.Contact{
		ID: "c-hidden", ContextID: "ctx-1",
		Source: "visitor", SourceID: "vid-1", Hidden: true,
	}
	cr := &fakeContactRepo{contacts: []*ct.Contact{existing, hidden}}
	tr := &fakeTrackRepo{}

	c, err := IdentifyVisitor(cr, tr, "ctx-1", "signup", "vid-1", map[string]interface{}{
		"email": email,
	})
	if err != nil {
		t.Fatalf("IdentifyVisitor: %v", err)
	}
	if c.ID != "c-existing" {
		t.Fatalf("returned contact = %q, want existing contact c-existing", c.ID)
	}
	if len(cr.mergeCalls) != 1 || cr.mergeCalls[0] != [2]string{"c-existing", "c-hidden"} {
		t.Errorf("merge calls = %v, want [[c-existing c-hidden]]", cr.mergeCalls)
	}
	if len(tr.reassignCalls) != 1 || tr.reassignCalls[0] != [3]string{"ctx-1", "c-hidden", "c-existing"} {
		t.Errorf("reassign calls = %v, want [[ctx-1 c-hidden c-existing]]", tr.reassignCalls)
	}
	if cr.updateCalls != 0 {
		t.Errorf("update calls = %d, want 0 (collision path must not update)", cr.updateCalls)
	}
}

func TestIdentifyVisitor_NoHiddenContactUsesLegacyPath(t *testing.T) {
	email := "ann@example.com"
	cr := &fakeContactRepo{}
	tr := &fakeTrackRepo{}

	c, err := IdentifyVisitor(cr, tr, "ctx-1", "signup", "vid-1", map[string]interface{}{
		"email": email,
	})
	if err != nil {
		t.Fatalf("IdentifyVisitor: %v", err)
	}
	if c == nil || c.ID == "" {
		t.Fatalf("contact = %+v, want a created contact", c)
	}
	if c.Email == nil || *c.Email != email {
		t.Errorf("email = %v, want %q", c.Email, email)
	}
	if len(cr.contacts) != 1 {
		t.Errorf("created contacts = %d, want 1", len(cr.contacts))
	}
	if len(tr.setCalls) != 1 || tr.setCalls[0] != [3]string{"ctx-1", "vid-1", c.ID} {
		t.Errorf("set calls = %v, want [[ctx-1 vid-1 %s]]", tr.setCalls, c.ID)
	}
	if len(tr.reassignCalls) != 0 {
		t.Errorf("reassign calls = %d, want 0", len(tr.reassignCalls))
	}
}

