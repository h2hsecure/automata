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
	"github.com/h2hsecure/automata/internal/domain/form"
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
