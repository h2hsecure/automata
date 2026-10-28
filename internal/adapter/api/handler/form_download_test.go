package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/h2hsecure/automata/internal/domain/form"
	"github.com/h2hsecure/automata/internal/infrastructure/storage"
)

// downloadFakeRepo returns a fixed submission from GetSubmission.
type downloadFakeRepo struct {
	submission *form.Submission
}

func (r *downloadFakeRepo) Create(f *form.Form) error { return nil }

func (r *downloadFakeRepo) GetByID(id string) (*form.Form, error) {
	return nil, fmt.Errorf("not found")
}

func (r *downloadFakeRepo) GetBySlug(contextID, slug string) (*form.Form, error) {
	return nil, fmt.Errorf("not found")
}

func (r *downloadFakeRepo) List(contextID string) ([]*form.Form, error) { return nil, nil }

func (r *downloadFakeRepo) Update(f *form.Form) error { return nil }

func (r *downloadFakeRepo) Delete(id string) error { return nil }

func (r *downloadFakeRepo) CreateSubmission(s *form.Submission) error { return nil }

func (r *downloadFakeRepo) GetSubmission(id string) (*form.Submission, error) {
	if r.submission == nil || r.submission.ID != id {
		return nil, fmt.Errorf("not found")
	}
	return r.submission, nil
}

func (r *downloadFakeRepo) ListSubmissions(formID string, opts form.ListSubmissionsOptions) ([]*form.Submission, int64, error) {
	return nil, 0, nil
}

func (r *downloadFakeRepo) DeleteSubmission(id string) error { return nil }

func newDownloadContext(t *testing.T, sid, idx string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/context/forms/form-1/submissions/"+sid+"/files/"+idx, nil)
	c.Params = gin.Params{
		{Key: "id", Value: "form-1"},
		{Key: "sid", Value: sid},
		{Key: "idx", Value: idx},
	}
	c.Set("context_id", "ctx-1")
	return c, w
}

func TestDownloadSubmissionFile_ServesStoredContent(t *testing.T) {
	store, err := storage.New(t.TempDir())
	if err != nil {
		t.Fatalf("storage.New: %v", err)
	}
	token, err := store.StoreMeta("ctx-1", ".png", "report.png", "image/png", []byte("PNGDATA"))
	if err != nil {
		t.Fatalf("StoreMeta: %v", err)
	}
	repo := &downloadFakeRepo{submission: &form.Submission{
		ID:        "sub-1",
		FormID:    "form-1",
		ContextID: "ctx-1",
		Files:     []form.FileUpload{{Field: "attachment", Name: "report.png", URL: token}},
	}}

	c, w := newDownloadContext(t, "sub-1", "0")
	h := NewFormHandler(repo).WithFileStore(store)
	h.DownloadSubmissionFile(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != "PNGDATA" {
		t.Fatalf("expected PNGDATA, got %q", got)
	}
	if ct := w.Header().Get("Content-Disposition"); ct == "" {
		t.Fatalf("expected Content-Disposition header")
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("expected image/png, got %q", ct)
	}
}

func TestDownloadSubmissionFile_WrongContext_NotFound(t *testing.T) {
	store, _ := storage.New(t.TempDir())
	token, _ := store.StoreMeta("ctx-1", ".txt", "a.txt", "text/plain", []byte("x"))
	repo := &downloadFakeRepo{submission: &form.Submission{
		ID:        "sub-1",
		FormID:    "form-1",
		ContextID: "ctx-2", // foreign context
		Files:     []form.FileUpload{{URL: token}},
	}}

	c, w := newDownloadContext(t, "sub-1", "0")
	h := NewFormHandler(repo).WithFileStore(store)
	h.DownloadSubmissionFile(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDownloadSubmissionFile_SubmissionNotFound(t *testing.T) {
	store, _ := storage.New(t.TempDir())
	repo := &downloadFakeRepo{}

	c, w := newDownloadContext(t, "missing", "0")
	h := NewFormHandler(repo).WithFileStore(store)
	h.DownloadSubmissionFile(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDownloadSubmissionFile_IndexOutOfRange(t *testing.T) {
	store, _ := storage.New(t.TempDir())
	repo := &downloadFakeRepo{submission: &form.Submission{
		ID: "sub-1", FormID: "form-1", ContextID: "ctx-1",
	}}

	c, w := newDownloadContext(t, "sub-1", "3")
	h := NewFormHandler(repo).WithFileStore(store)
	h.DownloadSubmissionFile(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDownloadSubmissionFile_InvalidIndex(t *testing.T) {
	store, _ := storage.New(t.TempDir())
	repo := &downloadFakeRepo{submission: &form.Submission{
		ID: "sub-1", FormID: "form-1", ContextID: "ctx-1",
	}}

	c, w := newDownloadContext(t, "sub-1", "abc")
	h := NewFormHandler(repo).WithFileStore(store)
	h.DownloadSubmissionFile(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDownloadSubmissionFile_FileMissingFromStore(t *testing.T) {
	store, _ := storage.New(t.TempDir())
	repo := &downloadFakeRepo{submission: &form.Submission{
		ID:        "sub-1",
		FormID:    "form-1",
		ContextID: "ctx-1",
		Files:     []form.FileUpload{{URL: "ctx-1/deadbeef.txt"}},
	}}

	c, w := newDownloadContext(t, "sub-1", "0")
	h := NewFormHandler(repo).WithFileStore(store)
	h.DownloadSubmissionFile(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDownloadSubmissionFile_NilStore_NotFound(t *testing.T) {
	repo := &downloadFakeRepo{submission: &form.Submission{
		ID: "sub-1", FormID: "form-1", ContextID: "ctx-1",
		Files: []form.FileUpload{{URL: "ctx-1/aaaa.txt"}},
	}}

	c, w := newDownloadContext(t, "sub-1", "0")
	h := NewFormHandler(repo)
	h.DownloadSubmissionFile(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}