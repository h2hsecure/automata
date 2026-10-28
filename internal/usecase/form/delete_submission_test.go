package form

import (
	"errors"
	"testing"

	"github.com/h2hsecure/automata/internal/domain/form"
)

// deleteTestRepo is an in-memory form.Repository for DeleteSubmission tests.
type deleteTestRepo struct {
	submission  *form.Submission
	getErr      error
	deleteErr   error
	deletedID   string
	deleteCalls int
}

func (r *deleteTestRepo) Create(f *form.Form) error { return nil }
func (r *deleteTestRepo) GetByID(id string) (*form.Form, error) {
	return nil, errors.New("not implemented")
}
func (r *deleteTestRepo) GetBySlug(contextID, slug string) (*form.Form, error) {
	return nil, errors.New("not implemented")
}
func (r *deleteTestRepo) List(contextID string) ([]*form.Form, error) {
	return nil, nil
}
func (r *deleteTestRepo) Update(f *form.Form) error { return nil }
func (r *deleteTestRepo) Delete(id string) error    { return nil }
func (r *deleteTestRepo) CreateSubmission(s *form.Submission) error {
	return nil
}
func (r *deleteTestRepo) GetSubmission(id string) (*form.Submission, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.submission != nil && r.submission.ID == id {
		return r.submission, nil
	}
	return nil, errors.New("submission not found")
}
func (r *deleteTestRepo) ListSubmissions(formID string, opts form.ListSubmissionsOptions) ([]*form.Submission, int64, error) {
	return nil, 0, nil
}
func (r *deleteTestRepo) DeleteSubmission(id string) error {
	r.deleteCalls++
	r.deletedID = id
	return r.deleteErr
}

func TestDeleteSubmission_EmptyID(t *testing.T) {
	repo := &deleteTestRepo{}
	if err := DeleteSubmission(repo, "ctx-1", ""); err == nil {
		t.Fatal("expected error for empty ID")
	}
	if repo.deleteCalls != 0 {
		t.Fatal("repo delete must not be called for empty ID")
	}
}

func TestDeleteSubmission_MissingSubmission(t *testing.T) {
	repo := &deleteTestRepo{getErr: errors.New("submission not found")}
	if err := DeleteSubmission(repo, "ctx-1", "sub-1"); err == nil {
		t.Fatal("expected error when submission missing")
	}
	if repo.deleteCalls != 0 {
		t.Fatal("repo delete must not be called when submission missing")
	}
}

func TestDeleteSubmission_WrongContext(t *testing.T) {
	repo := &deleteTestRepo{submission: &form.Submission{ID: "sub-1", ContextID: "other-ctx"}}
	if err := DeleteSubmission(repo, "ctx-1", "sub-1"); err == nil {
		t.Fatal("expected error for foreign-context submission")
	}
	if repo.deleteCalls != 0 {
		t.Fatal("repo delete must not delete a foreign-context submission")
	}
}

func TestDeleteSubmission_Success(t *testing.T) {
	repo := &deleteTestRepo{submission: &form.Submission{ID: "sub-1", ContextID: "ctx-1"}}
	if err := DeleteSubmission(repo, "ctx-1", "sub-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.deletedID != "sub-1" || repo.deleteCalls != 1 {
		t.Fatalf("expected one delete of sub-1, got %q (%d calls)", repo.deletedID, repo.deleteCalls)
	}
}

func TestDeleteSubmission_DeleteErrorWrapped(t *testing.T) {
	repo := &deleteTestRepo{
		submission: &form.Submission{ID: "sub-1", ContextID: "ctx-1"},
		deleteErr:  errors.New("db down"),
	}
	err := DeleteSubmission(repo, "ctx-1", "sub-1")
	if err == nil {
		t.Fatal("expected error when repo delete fails")
	}
	if err.Error() == "db down" {
		t.Fatal("expected wrapped error, got bare repo error")
	}
}
