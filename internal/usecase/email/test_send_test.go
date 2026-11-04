package email

import (
	"testing"

	dmemail "github.com/h2hsecure/automata/internal/domain/email"
)

// fakeTestSendTemplateRepo returns one template for a known key.
type fakeTestSendTemplateRepo struct{}

func (r *fakeTestSendTemplateRepo) Create(t *dmemail.EmailTemplate) error                      { return nil }
func (r *fakeTestSendTemplateRepo) GetByID(id string) (*dmemail.EmailTemplate, error)          { return nil, nil }
func (r *fakeTestSendTemplateRepo) GetByKey(c, k string) (*dmemail.EmailTemplate, error) {
	if k == "test_template" {
		return &dmemail.EmailTemplate{
			ID: "t-test", ContextID: c, Key: "test_template",
			Subject: "Test {{ .name }}", BodyText: "Hello {{ .name }}",
		}, nil
	}
	return nil, dmemail.ErrNotFound
}
func (r *fakeTestSendTemplateRepo) List(c string) ([]*dmemail.EmailTemplate, error)              { return nil, nil }
func (r *fakeTestSendTemplateRepo) ListByForm(c, f string) ([]*dmemail.EmailTemplate, error)     { return nil, nil }
func (r *fakeTestSendTemplateRepo) Update(t *dmemail.EmailTemplate) error                        { return nil }
func (r *fakeTestSendTemplateRepo) Delete(id string) error                                       { return nil }
func (r *fakeTestSendTemplateRepo) SeedDefaults(c string) error                                  { return nil }

// TestTestSend_EmptyToRejected verifies that an empty recipient is rejected
// before anything is enqueued.
func TestTestSend_EmptyToRejected(t *testing.T) {
	q := &fakeQueue{}
	u := NewEmailUsecase(&fakeTestSendTemplateRepo{}, q)

	_, _, _, err := u.TestSend("ctx-1", dmemail.TestSendRequest{
		To:       "",
		Template: "test_template",
	})
	if err == nil {
		t.Fatal("TestSend() with empty To must be rejected")
	}
	if len(q.jobs) != 0 {
		t.Errorf("expected no jobs enqueued, got %d", len(q.jobs))
	}
}

// TestTestSend_InvalidToRejected verifies that a malformed recipient is
// rejected before anything is enqueued.
func TestTestSend_InvalidToRejected(t *testing.T) {
	q := &fakeQueue{}
	u := NewEmailUsecase(&fakeTestSendTemplateRepo{}, q)

	_, _, _, err := u.TestSend("ctx-1", dmemail.TestSendRequest{
		To:       "not-an-email",
		Template: "test_template",
	})
	if err == nil {
		t.Fatal("TestSend() with invalid To must be rejected")
	}
	if len(q.jobs) != 0 {
		t.Errorf("expected no jobs enqueued, got %d", len(q.jobs))
	}
}

// TestTestSend_ValidToEnqueued is the sanity case: a valid recipient
// produces exactly one enqueued job.
func TestTestSend_ValidToEnqueued(t *testing.T) {
	q := &fakeQueue{}
	u := NewEmailUsecase(&fakeTestSendTemplateRepo{}, q)

	_, _, _, err := u.TestSend("ctx-1", dmemail.TestSendRequest{
		To:        "test@example.com",
		Template:  "test_template",
		Variables: map[string]string{"name": "Ann"},
	})
	if err != nil {
		t.Fatalf("TestSend() error: %v", err)
	}
	if len(q.jobs) != 1 {
		t.Fatalf("expected 1 job enqueued, got %d", len(q.jobs))
	}
	if len(q.jobs[0].To) != 1 || q.jobs[0].To[0] != "test@example.com" {
		t.Errorf("job.To = %v, want [test@example.com]", q.jobs[0].To)
	}
}