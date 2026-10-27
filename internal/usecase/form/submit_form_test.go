package form

import (
	"errors"
	"testing"
	"time"

	domain "github.com/hekemen/automata/internal/domain/form"
)

type submitTestRepo struct {
	form        *domain.Form
	submissions []*domain.Submission
}

func (r *submitTestRepo) Create(f *domain.Form) error { return nil }
func (r *submitTestRepo) GetByID(id string) (*domain.Form, error) {
	return r.form, nil
}
func (r *submitTestRepo) GetBySlug(contextID, slug string) (*domain.Form, error) {
	return r.form, nil
}
func (r *submitTestRepo) List(contextID string) ([]*domain.Form, error) {
	return nil, nil
}
func (r *submitTestRepo) Update(f *domain.Form) error { return nil }
func (r *submitTestRepo) Delete(id string) error      { return nil }
func (r *submitTestRepo) CreateSubmission(s *domain.Submission) error {
	r.submissions = append(r.submissions, s)
	return nil
}
func (r *submitTestRepo) ListSubmissions(formID string, opts domain.ListSubmissionsOptions) ([]*domain.Submission, int64, error) {
	return nil, 0, nil
}
func (r *submitTestRepo) GetSubmission(id string) (*domain.Submission, error) {
	return nil, nil
}

type fakeResolver struct {
	calls  []string
	upload domain.FileUpload
	err    error
}

func (r *fakeResolver) Resolve(contextID, token string) (domain.FileUpload, error) {
	r.calls = append(r.calls, contextID+"/"+token)
	return r.upload, r.err
}

func submitTestForm() *domain.Form {
	return &domain.Form{
		ID:        "f1",
		ContextID: "ctx1",
		Slug:      "contact",
		Name:      "Contact",
		Fields: []domain.FormField{
			{Slug: "name", Type: "text", Label: "Name", Required: true},
			{Slug: "resume", Type: "file", Label: "Resume"},
			{Slug: "note", Type: "text", Label: "Note"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestSubmitForm_ResolvesFileFieldTokens(t *testing.T) {
	repo := &submitTestRepo{form: submitTestForm()}
	resolver := &fakeResolver{upload: domain.FileUpload{Name: "resume.pdf", Size: 10, Type: "application/pdf", URL: "ctx1/tok1"}}

	sub, err := SubmitForm(repo, "ctx1", SubmitFormInput{
		Slug: "contact",
		Data: map[string]interface{}{"name": "Ann", "resume": "ctx1/tok1"},
	}, resolver)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if len(sub.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(sub.Files))
	}
	if sub.Files[0].Field != "resume" {
		t.Fatalf("expected field resume, got %q", sub.Files[0].Field)
	}
	if sub.Files[0].Name != "resume.pdf" {
		t.Fatalf("unexpected name %q", sub.Files[0].Name)
	}
	if len(resolver.calls) != 1 || resolver.calls[0] != "ctx1/ctx1/tok1" {
		t.Fatalf("unexpected resolver calls %v", resolver.calls)
	}
}

func TestSubmitForm_NoTokenNoResolverCall(t *testing.T) {
	repo := &submitTestRepo{form: submitTestForm()}
	resolver := &fakeResolver{}

	sub, err := SubmitForm(repo, "ctx1", SubmitFormInput{
		Slug: "contact",
		Data: map[string]interface{}{"name": "Ann", "note": "hi"},
	}, resolver)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if len(sub.Files) != 0 {
		t.Fatalf("expected no files, got %d", len(sub.Files))
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("expected no resolver calls, got %v", resolver.calls)
	}
}

func TestSubmitForm_NonFileFieldTokenNotResolved(t *testing.T) {
	repo := &submitTestRepo{form: submitTestForm()}
	resolver := &fakeResolver{}

	sub, err := SubmitForm(repo, "ctx1", SubmitFormInput{
		Slug: "contact",
		Data: map[string]interface{}{"name": "Ann", "note": "some token"},
	}, resolver)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if len(sub.Files) != 0 {
		t.Fatalf("expected no files, got %d", len(sub.Files))
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("expected no resolver calls, got %v", resolver.calls)
	}
}

func TestSubmitForm_ResolverErrorFailsSubmission(t *testing.T) {
	repo := &submitTestRepo{form: submitTestForm()}
	resolver := &fakeResolver{err: errors.New("unknown token")}

	_, err := SubmitForm(repo, "ctx1", SubmitFormInput{
		Slug: "contact",
		Data: map[string]interface{}{"name": "Ann", "resume": "ctx1/bogus"},
	}, resolver)
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
	if len(repo.submissions) != 0 {
		t.Fatal("expected no submission to be persisted")
	}
}

func TestSubmitForm_NilResolverKeepsLegacyBehavior(t *testing.T) {
	repo := &submitTestRepo{form: submitTestForm()}

	sub, err := SubmitForm(repo, "ctx1", SubmitFormInput{
		Slug: "contact",
		Data:  map[string]interface{}{"name": "Ann"},
		Files: []domain.FileUpload{{Field: "resume", Name: "x.pdf"}},
	}, nil)
	if err != nil {
		t.Fatalf("SubmitForm: %v", err)
	}
	if len(sub.Files) != 1 || sub.Files[0].Name != "x.pdf" {
		t.Fatalf("expected legacy files to pass through, got %+v", sub.Files)
	}
}