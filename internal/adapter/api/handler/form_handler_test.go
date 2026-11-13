package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	ct "github.com/h2hsecure/automata/internal/domain/contact"
	"github.com/h2hsecure/automata/internal/domain/form"
	trackingdomain "github.com/h2hsecure/automata/internal/domain/tracking"
	uc "github.com/h2hsecure/automata/internal/usecase/form"
)

// fakeFormRepo is an in-memory form.Repository for handler tests.
type fakeFormRepo struct {
	bySlug      *form.Form
	submissions []*form.Submission
	sub         *form.Submission
	deletedIDs  []string
}

func (r *fakeFormRepo) Create(f *form.Form) error { return nil }

func (r *fakeFormRepo) GetByID(id string) (*form.Form, error) { return r.bySlug, nil }

func (r *fakeFormRepo) GetBySlug(contextID, slug string) (*form.Form, error) {
	return r.bySlug, nil
}

func (r *fakeFormRepo) List(contextID string) ([]*form.Form, error) { return nil, nil }

func (r *fakeFormRepo) Update(f *form.Form) error { return nil }

func (r *fakeFormRepo) Delete(id string) error { return nil }

func (r *fakeFormRepo) CreateSubmission(s *form.Submission) error {
	r.submissions = append(r.submissions, s)
	return nil
}

func (r *fakeFormRepo) GetSubmission(id string) (*form.Submission, error) {
	if r.sub == nil || r.sub.ID != id {
		return nil, fmt.Errorf("submission not found")
	}
	return r.sub, nil
}

func (r *fakeFormRepo) ListSubmissions(formID string, opts form.ListSubmissionsOptions) ([]*form.Submission, int64, error) {
	return nil, 0, nil
}

func (r *fakeFormRepo) DeleteSubmission(id string) error {
	r.deletedIDs = append(r.deletedIDs, id)
	return nil
}

// fakeFormContactRepo is an in-memory contact repository for form handler
// tests; it records created contacts and activities.
type fakeFormContactRepo struct {
	contacts   []*ct.Contact
	activities []*ct.Activity
}

func (r *fakeFormContactRepo) Create(c *ct.Contact) error {
	r.contacts = append(r.contacts, c)
	return nil
}
func (r *fakeFormContactRepo) GetByID(id string) (*ct.Contact, error) {
	for _, c := range r.contacts {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, nil
}
func (r *fakeFormContactRepo) List(offset, limit int, filters ct.FilterOptions) ([]*ct.Contact, error) {
	return r.contacts, nil
}
func (r *fakeFormContactRepo) Update(c *ct.Contact) error { return nil }
func (r *fakeFormContactRepo) Delete(id string) error    { return nil }
func (r *fakeFormContactRepo) FindByEmail(contextID, email string) (*ct.Contact, error) {
	for _, c := range r.contacts {
		if c.ContextID == contextID && c.Email != nil && *c.Email == email {
			return c, nil
		}
	}
	// Mirror the real repo: CreateContact relies on a "contact not found"
	// error (not a nil result) to distinguish miss from duplicate.
	return nil, fmt.Errorf("contact not found")
}
func (r *fakeFormContactRepo) Merge(keepID, mergeIntoID string) error { return nil }
func (r *fakeFormContactRepo) CreateActivity(a *ct.Activity) error {
	r.activities = append(r.activities, a)
	return nil
}
func (r *fakeFormContactRepo) GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]ct.Activity, int64, error) {
	return nil, 0, nil
}
func (r *fakeFormContactRepo) GetActivityCount(contactID string) (int64, error) { return 0, nil }
func (r *fakeFormContactRepo) CountByContext(contextID string) (int64, error) {
	return int64(len(r.contacts)), nil
}
func (r *fakeFormContactRepo) FindBySourceID(contextID, source, sourceID string) (*ct.Contact, error) {
	for _, c := range r.contacts {
		if c.ContextID == contextID && c.Source == source && c.SourceID == sourceID {
			return c, nil
		}
	}
	return nil, nil
}
func (r *fakeFormContactRepo) Count(contextID string, filters ct.FilterOptions) (int64, error) {
	return int64(len(r.contacts)), nil
}

// fakeFormTrackingRepo is a no-op tracking repository for form handler tests.
type fakeFormTrackingRepo struct{}

func (r *fakeFormTrackingRepo) CreateEvent(e *trackingdomain.Event) error { return nil }
func (r *fakeFormTrackingRepo) CreateEventsBatch(events []*trackingdomain.Event) error {
	return nil
}
func (r *fakeFormTrackingRepo) GetVisitor(contextID, visitorID string) (*trackingdomain.Visitor, error) {
	return nil, nil
}
func (r *fakeFormTrackingRepo) UpsertVisitor(v *trackingdomain.Visitor) error { return nil }
func (r *fakeFormTrackingRepo) GetDashboardMetrics(contextID string, dateRange trackingdomain.DateRange) (*trackingdomain.DashboardMetrics, error) {
	return nil, nil
}
func (r *fakeFormTrackingRepo) GetEvents(contextID string, opts trackingdomain.EventFilter) ([]*trackingdomain.Event, int64, error) {
	return nil, 0, nil
}
func (r *fakeFormTrackingRepo) GetTopPages(contextID string, dateRange trackingdomain.DateRange, limit int) ([]trackingdomain.PageViewCount, error) {
	return nil, nil
}
func (r *fakeFormTrackingRepo) GetTopReferrers(contextID string, dateRange trackingdomain.DateRange, limit int) ([]trackingdomain.ReferrerCount, error) {
	return nil, nil
}
func (r *fakeFormTrackingRepo) GetUAGroups(contextID string, dateRange trackingdomain.DateRange) ([]trackingdomain.UACount, error) {
	return nil, nil
}
func (r *fakeFormTrackingRepo) GetActiveVisitors(contextID string, minutes int) (int64, error) {
	return 0, nil
}
func (r *fakeFormTrackingRepo) PurgeOldEvents(contextID string, retentionDays int) (int64, error) {
	return 0, nil
}
func (r *fakeFormTrackingRepo) ListVisitors(ctx context.Context, contextID string, opts trackingdomain.ListVisitorsOpts) ([]*trackingdomain.Visitor, int64, error) {
	return nil, 0, nil
}
func (r *fakeFormTrackingRepo) GetVisitorByID(ctx context.Context, contextID, visitorID string) (*trackingdomain.Visitor, error) {
	return nil, nil
}
func (r *fakeFormTrackingRepo) GetVisitorEvents(ctx context.Context, contextID, visitorID string, opts trackingdomain.GetVisitorEventsOpts) ([]*trackingdomain.Event, int64, error) {
	return nil, 0, nil
}
func (r *fakeFormTrackingRepo) SetVisitorContactID(ctx context.Context, contextID, visitorID, contactID string) error {
	return nil
}
func (r *fakeFormTrackingRepo) ReassignVisitorContactID(contextID, oldContactID, newContactID string) error {
	return nil
}

// fakeWebhookNotifier records DeliverEvent calls.
type fakeWebhookNotifier struct {
	calls []webhookNotifierCall
	err   error
}

type webhookNotifierCall struct {
	contextID string
	event     string
	formID    string
	data      map[string]interface{}
}

func (f *fakeWebhookNotifier) DeliverEvent(_ context.Context, contextID, event, formID string, data map[string]interface{}) error {
	f.calls = append(f.calls, webhookNotifierCall{contextID: contextID, event: event, formID: formID, data: data})
	return f.err
}

func newTestFormHandler(repo *fakeFormRepo, notifier uc.WebhookNotifier) *FormHandler {
	return NewFormHandlerWithEmailAndWebhooks(repo, nil, nil, nil, notifier)
}

func postFormSubmit(t *testing.T, h *FormHandler) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/form/submit/test-form",
		bytes.NewBufferString(`{"data": {"email": "user@example.com"}}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "slug", Value: "test-form"}}
	c.Set("context_id", "ctx-1")

	h.SubmitForm(c)
	return w
}

func testForm() *form.Form {
	return &form.Form{
		ID:        "form-1",
		ContextID: "ctx-1",
		Slug:      "test-form",
		Name:      "Test Form",
		Fields:    []form.FormField{{Slug: "email", Type: "email", Required: true}},
	}
}

func TestSubmitForm_CallsWebhookNotifier(t *testing.T) {
	notifier := &fakeWebhookNotifier{}
	h := newTestFormHandler(&fakeFormRepo{bySlug: testForm()}, notifier)

	w := postFormSubmit(t, h)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("expected 1 notifier call, got %d", len(notifier.calls))
	}
	call := notifier.calls[0]
	if call.contextID != "ctx-1" || call.event != "form.submitted" || call.formID != "form-1" {
		t.Fatalf("unexpected notifier call: %+v", call)
	}
	if call.data["email"] != "user@example.com" {
		t.Fatalf("unexpected notifier data: %v", call.data)
	}
}

func TestSubmitForm_WebhookFailure_IsNonFatal(t *testing.T) {
	notifier := &fakeWebhookNotifier{err: errors.New("webhook down")}
	repo := &fakeFormRepo{bySlug: testForm()}
	h := newTestFormHandler(repo, notifier)

	w := postFormSubmit(t, h)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 despite webhook failure, got %d", w.Code)
	}
	if len(repo.submissions) != 1 {
		t.Fatalf("expected submission to be created, got %d", len(repo.submissions))
	}
}

func TestSubmitForm_NilNotifier_StillWorks(t *testing.T) {
	h := NewFormHandler(&fakeFormRepo{bySlug: testForm()})

	w := postFormSubmit(t, h)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func postFormSubmitBody(t *testing.T, h *FormHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/form/submit/test-form",
		bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "slug", Value: "test-form"}}
	c.Set("context_id", "ctx-1")

	h.SubmitForm(c)
	return w
}

func TestSubmitForm_CreatesFormSubmissionActivity(t *testing.T) {
	cr := &fakeFormContactRepo{}
	tr := &fakeFormTrackingRepo{}
	h := NewFormHandlerWithEmailAndWebhooks(&fakeFormRepo{bySlug: testForm()}, cr, tr, nil, nil)

	w := postFormSubmitBody(t, h, `{"data": {"email": "user@example.com"}, "visitor_id": "v1"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if len(cr.contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(cr.contacts))
	}

	var activity *ct.Activity
	for _, a := range cr.activities {
		if a.Type == ct.ActivityFormSubmission {
			activity = a
		}
	}
	if activity == nil {
		t.Fatalf("expected a form_submission activity, got %+v", cr.activities)
	}
	if activity.ContactID != cr.contacts[0].ID {
		t.Errorf("activity contact = %q, want %q", activity.ContactID, cr.contacts[0].ID)
	}
	if activity.ContextID != "ctx-1" {
		t.Errorf("activity context = %q, want ctx-1", activity.ContextID)
	}
	if activity.Data["form_name"] != "Test Form" || activity.Data["form_slug"] != "test-form" {
		t.Errorf("activity data = %v, want form_name Test Form / form_slug test-form", activity.Data)
	}
	if activity.Data["submission_id"] == "" {
		t.Errorf("activity data missing submission_id: %v", activity.Data)
	}
}

func TestSubmitForm_NoEmail_NoActivity(t *testing.T) {
	cr := &fakeFormContactRepo{}
	tr := &fakeFormTrackingRepo{}
	// No required email field, so the submission validates without an email.
	f := testForm()
	f.Fields = []form.FormField{{Slug: "name", Type: "text", Required: true}}
	h := NewFormHandlerWithEmailAndWebhooks(&fakeFormRepo{bySlug: f}, cr, tr, nil, nil)

	w := postFormSubmitBody(t, h, `{"data": {"name": "No Email"}, "visitor_id": "v1"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if len(cr.activities) != 0 {
		t.Fatalf("expected no activities without an email, got %+v", cr.activities)
	}
}

func newDeleteSubmissionContext(t *testing.T, h *FormHandler, contextID, sid string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("DELETE", "/api/context/forms/form-1/submissions/"+sid, nil)
	c.Params = gin.Params{
		{Key: "id", Value: "form-1"},
		{Key: "sid", Value: sid},
	}
	if contextID != "" {
		c.Set("context_id", contextID)
	}
	h.DeleteSubmission(c)
	// Gin's engine flushes a deferred status (e.g. 204) at the end of the
	// request cycle; mirror that since we call the handler directly.
	c.Writer.WriteHeaderNow()
	return w
}

func TestDeleteSubmission_NoContext_401(t *testing.T) {
	repo := &fakeFormRepo{sub: &form.Submission{ID: "sub-1", ContextID: "ctx-1"}}
	h := NewFormHandler(repo)

	w := newDeleteSubmissionContext(t, h, "", "sub-1")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if len(repo.deletedIDs) != 0 {
		t.Fatalf("must not delete without context, got %v", repo.deletedIDs)
	}
}

func TestDeleteSubmission_Missing_404(t *testing.T) {
	repo := &fakeFormRepo{}
	h := NewFormHandler(repo)

	w := newDeleteSubmissionContext(t, h, "ctx-1", "sub-missing")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if len(repo.deletedIDs) != 0 {
		t.Fatalf("must not delete a missing submission, got %v", repo.deletedIDs)
	}
}

func TestDeleteSubmission_WrongContext_404(t *testing.T) {
	repo := &fakeFormRepo{sub: &form.Submission{ID: "sub-1", ContextID: "other-ctx"}}
	h := NewFormHandler(repo)

	w := newDeleteSubmissionContext(t, h, "ctx-1", "sub-1")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for foreign context, got %d", w.Code)
	}
	if len(repo.deletedIDs) != 0 {
		t.Fatalf("must not delete a foreign-context submission, got %v", repo.deletedIDs)
	}
}

func TestDeleteSubmission_Success_204(t *testing.T) {
	repo := &fakeFormRepo{sub: &form.Submission{ID: "sub-1", ContextID: "ctx-1"}}
	h := NewFormHandler(repo)

	w := newDeleteSubmissionContext(t, h, "ctx-1", "sub-1")
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if len(repo.deletedIDs) != 1 || repo.deletedIDs[0] != "sub-1" {
		t.Fatalf("expected sub-1 deleted, got %v", repo.deletedIDs)
	}
}
