package email

import (
	"context"
	"testing"
	"time"

	"github.com/h2hsecure/automata/internal/domain/contact"
	dmemail "github.com/h2hsecure/automata/internal/domain/email"
	fwform "github.com/h2hsecure/automata/internal/domain/form"
	"github.com/h2hsecure/automata/internal/infrastructure/queue"
)

// --- fakes ---

type fakeSubmissionTemplateRepo struct {
	templates []*dmemail.EmailTemplate
}

func (r *fakeSubmissionTemplateRepo) ListByForm(contextID, formID string) ([]*dmemail.EmailTemplate, error) {
	var out []*dmemail.EmailTemplate
	for _, t := range r.templates {
		if t.ContextID == contextID && t.FormID != nil && *t.FormID == formID {
			out = append(out, t)
		}
	}
	return out, nil
}
func (r *fakeSubmissionTemplateRepo) Create(t *dmemail.EmailTemplate) error                        { return nil }
func (r *fakeSubmissionTemplateRepo) GetByID(id string) (*dmemail.EmailTemplate, error)             { return nil, nil }
func (r *fakeSubmissionTemplateRepo) GetByKey(c, k string) (*dmemail.EmailTemplate, error)         { return nil, nil }
func (r *fakeSubmissionTemplateRepo) List(c string) ([]*dmemail.EmailTemplate, error)              { return nil, nil }
func (r *fakeSubmissionTemplateRepo) Update(t *dmemail.EmailTemplate) error                        { return nil }
func (r *fakeSubmissionTemplateRepo) Delete(id string) error                                       { return nil }
func (r *fakeSubmissionTemplateRepo) SeedDefaults(c string) error                                  { return nil }

type fakeQueue struct {
	jobs []*queue.EmailJob
}

func (q *fakeQueue) Enqueue(job *queue.EmailJob) error {
	q.jobs = append(q.jobs, job)
	return nil
}
func (q *fakeQueue) StartWorker(ctx context.Context, m queue.Mailer) {}
func (q *fakeQueue) ListPending(c string, limit int) ([]*queue.EmailJob, error)          { return nil, nil }
func (q *fakeQueue) ListHistory(c, s string, limit int) ([]*queue.EmailHistory, error)   { return nil, nil }
func (q *fakeQueue) Retry(c, jobID string) error                                          { return nil }
func (q *fakeQueue) Cancel(c, jobID string) error                                         { return nil }
func (q *fakeQueue) PruneHistory(olderThan time.Time) (int64, error)                      { return 0, nil }
func (q *fakeQueue) CountPending(c string) (int, error)                                   { return 0, nil }
func (q *fakeQueue) CountHistory(c, s string, since *time.Time) (int, error)              { return 0, nil }

// --- helpers ---

func submissionForm(settings map[string]interface{}) *fwform.Form {
	return &fwform.Form{
		ID:        "form-1",
		ContextID: "ctx-1",
		Name:      "Contact Form",
		Slug:      "contact",
		Fields: []fwform.FormField{
			{Slug: "email", Type: "email", Label: "Email", Required: true},
			{Slug: "name", Type: "text", Label: "Name", Required: true},
		},
		Settings: settings,
	}
}

func submissionWith(email string) *fwform.Submission {
	data := map[string]interface{}{"name": "Ann"}
	if email != "" {
		data["email"] = email
	}
	return &fwform.Submission{
		ID:        "sub-1",
		FormID:    "form-1",
		ContextID: "ctx-1",
		Data:      data,
		CreatedAt: time.Now(),
	}
}

func adminTemplate() *dmemail.EmailTemplate {
	formID := "form-1"
	return &dmemail.EmailTemplate{
		ID: "t-admin", ContextID: "ctx-1", Key: "form_submission_admin",
		Subject: "New submission", BodyText: "{{ .form_name }}",
		Kind: dmemail.TemplateKindForm, FormID: &formID, Audience: "admin",
	}
}

func submitterTemplate() *dmemail.EmailTemplate {
	formID := "form-1"
	return &dmemail.EmailTemplate{
		ID: "t-sub", ContextID: "ctx-1", Key: "form_submission",
		Subject: "Thanks", BodyText: "Hi",
		Kind: dmemail.TemplateKindForm, FormID: &formID, Audience: "submitter",
	}
}

// fakeContactRepoForSub provides the minimal contact repo surface SendForSubmission
// uses and captures the activities recorded on it.
type fakeContactRepoForSub struct {
	contacts   map[string]*contact.Contact
	activities []*contact.Activity
}

func (r *fakeContactRepoForSub) FindByEmail(contextID, email string) (*contact.Contact, error) {
	c, ok := r.contacts[email]
	if !ok {
		return nil, dmemail.ErrNotFound
	}
	return c, nil
}
func (r *fakeContactRepoForSub) Create(c *contact.Contact) error                                          { return nil }
func (r *fakeContactRepoForSub) GetByID(id string) (*contact.Contact, error)                               { return nil, nil }
func (r *fakeContactRepoForSub) List(offset, limit int, filters contact.FilterOptions) ([]*contact.Contact, error) {
	return nil, nil
}
func (r *fakeContactRepoForSub) Update(c *contact.Contact) error                                           { return nil }
func (r *fakeContactRepoForSub) Delete(id string) error                                                    { return nil }
func (r *fakeContactRepoForSub) Merge(keepID, mergeIntoID string) error                                    { return nil }
func (r *fakeContactRepoForSub) CreateActivity(a *contact.Activity) error {
	r.activities = append(r.activities, a)
	return nil
}
func (r *fakeContactRepoForSub) GetActivity(contactID, contextID string, offset, limit int, activityType *string) ([]contact.Activity, int64, error) {
	return nil, 0, nil
}
func (r *fakeContactRepoForSub) GetActivityCount(contactID string) (int64, error)                          { return 0, nil }
func (r *fakeContactRepoForSub) CountByContext(contextID string) (int64, error)                            { return 0, nil }
func (r *fakeContactRepoForSub) CountDistinctContactsByActivity(contextID, activityType string, since *time.Time) (int, error) {
	return 0, nil
}
func (r *fakeContactRepoForSub) FindBySourceID(contextID, source, sourceID string) (*contact.Contact, error) {
	return nil, nil
}
func (r *fakeContactRepoForSub) Count(contextID string, filters contact.FilterOptions) (int64, error)      { return 0, nil }

// --- tests ---

func TestSendForSubmission_SendsAdminAndSubmitter(t *testing.T) {
	q := &fakeQueue{}
	u := NewEmailUsecaseWithContact(&fakeSubmissionTemplateRepo{templates: []*dmemail.EmailTemplate{adminTemplate(), submitterTemplate()}}, q, nil)
	f := submissionForm(map[string]interface{}{
		"notifications": map[string]interface{}{
			"admin":     map[string]interface{}{"enabled": true, "to": []interface{}{"admin@example.com"}},
			"submitter": map[string]interface{}{"enabled": true},
		},
	})

	u.SendForSubmission(f, submissionWith("user@example.com"))

	if len(q.jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(q.jobs))
	}
	seen := map[string]bool{}
	for _, j := range q.jobs {
		for _, to := range j.To {
			seen[to] = true
		}
	}
	if !seen["admin@example.com"] {
		t.Errorf("admin email not sent: %v", q.jobs)
	}
	if !seen["user@example.com"] {
		t.Errorf("submitter email not sent: %v", q.jobs)
	}
}

func TestSendForSubmission_AdminDisabled_SendsSubmitterOnly(t *testing.T) {
	q := &fakeQueue{}
	u := NewEmailUsecaseWithContact(&fakeSubmissionTemplateRepo{templates: []*dmemail.EmailTemplate{adminTemplate(), submitterTemplate()}}, q, nil)
	f := submissionForm(map[string]interface{}{
		"notifications": map[string]interface{}{
			"admin":     map[string]interface{}{"enabled": false, "to": []interface{}{"admin@example.com"}},
			"submitter": map[string]interface{}{"enabled": true},
		},
	})

	u.SendForSubmission(f, submissionWith("user@example.com"))

	if len(q.jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(q.jobs))
	}
	if len(q.jobs[0].To) != 1 || q.jobs[0].To[0] != "user@example.com" {
		t.Errorf("expected only submitter job, got %v", q.jobs[0].To)
	}
}

func TestSendForSubmission_SubmitterDisabled_SendsAdminOnly(t *testing.T) {
	q := &fakeQueue{}
	u := NewEmailUsecaseWithContact(&fakeSubmissionTemplateRepo{templates: []*dmemail.EmailTemplate{adminTemplate(), submitterTemplate()}}, q, nil)
	f := submissionForm(map[string]interface{}{
		"notifications": map[string]interface{}{
			"admin":     map[string]interface{}{"enabled": true, "to": []interface{}{"admin@example.com"}},
			"submitter": map[string]interface{}{"enabled": false},
		},
	})

	u.SendForSubmission(f, submissionWith("user@example.com"))

	if len(q.jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(q.jobs))
	}
	if len(q.jobs[0].To) != 1 || q.jobs[0].To[0] != "admin@example.com" {
		t.Errorf("expected only admin job, got %v", q.jobs[0].To)
	}
}

func TestSendForSubmission_NoAdminTo_SendsSubmitterOnly(t *testing.T) {
	q := &fakeQueue{}
	u := NewEmailUsecaseWithContact(&fakeSubmissionTemplateRepo{templates: []*dmemail.EmailTemplate{adminTemplate(), submitterTemplate()}}, q, nil)
	// No settings at all: admin has no configured recipients, submitter defaults on.
	f := submissionForm(nil)

	u.SendForSubmission(f, submissionWith("user@example.com"))

	if len(q.jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(q.jobs))
	}
	if q.jobs[0].To[0] != "user@example.com" {
		t.Errorf("expected submitter job, got %v", q.jobs[0].To)
	}
}

func TestSendForSubmission_NoEmailInSubmission_AdminStillSent(t *testing.T) {
	q := &fakeQueue{}
	u := NewEmailUsecaseWithContact(&fakeSubmissionTemplateRepo{templates: []*dmemail.EmailTemplate{adminTemplate(), submitterTemplate()}}, q, nil)
	f := submissionForm(map[string]interface{}{
		"notifications": map[string]interface{}{
			"admin": map[string]interface{}{"enabled": true, "to": []interface{}{"admin@example.com"}},
		},
	})

	u.SendForSubmission(f, submissionWith(""))

	if len(q.jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(q.jobs))
	}
	if q.jobs[0].To[0] != "admin@example.com" {
		t.Errorf("expected admin job, got %v", q.jobs[0].To)
	}
}

func TestSendForSubmission_MultipleAdminRecipients(t *testing.T) {
	q := &fakeQueue{}
	u := NewEmailUsecaseWithContact(&fakeSubmissionTemplateRepo{templates: []*dmemail.EmailTemplate{adminTemplate()}}, q, nil)
	f := submissionForm(map[string]interface{}{
		"notifications": map[string]interface{}{
			"admin": map[string]interface{}{"enabled": true, "to": []interface{}{"a@example.com", "b@example.com"}},
		},
	})

	u.SendForSubmission(f, submissionWith("user@example.com"))

	seen := map[string]bool{}
	for _, j := range q.jobs {
		for _, to := range j.To {
			seen[to] = true
		}
	}
	if !seen["a@example.com"] || !seen["b@example.com"] {
		t.Errorf("expected both admin recipients, got %v", seen)
	}
}

func TestSendForSubmission_ContactActivityStillRecorded(t *testing.T) {
	q := &fakeQueue{}
	email := "user@example.com"
	contactRepo := &fakeContactRepoForSub{contacts: map[string]*contact.Contact{email: {ID: "c-1", ContextID: "ctx-1", Email: &email}}}
	u := NewEmailUsecaseWithContact(&fakeSubmissionTemplateRepo{templates: []*dmemail.EmailTemplate{submitterTemplate()}}, q, contactRepo)
	f := submissionForm(nil)

	u.SendForSubmission(f, submissionWith(email))

	if len(contactRepo.activities) != 1 {
		t.Fatalf("expected 1 activity, got %d", len(contactRepo.activities))
	}
	if contactRepo.activities[0].Type != contact.ActivityEmailSent {
		t.Errorf("activity type = %v", contactRepo.activities[0].Type)
	}
}