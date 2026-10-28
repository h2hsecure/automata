package tracking

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hekemen/automata/internal/domain/form"
	"github.com/hekemen/automata/internal/infrastructure/storage"
)

type uploadTestRepo struct {
	bySlug map[string]*form.Form
}

func (r *uploadTestRepo) Create(f *form.Form) error { return nil }
func (r *uploadTestRepo) GetByID(id string) (*form.Form, error) {
	return nil, nil
}
func (r *uploadTestRepo) GetBySlug(contextID, slug string) (*form.Form, error) {
	f, ok := r.bySlug[contextID+"/"+slug]
	if !ok {
		return nil, errNotFound
	}
	return f, nil
}
func (r *uploadTestRepo) List(contextID string) ([]*form.Form, error) { return nil, nil }
func (r *uploadTestRepo) Update(f *form.Form) error                  { return nil }
func (r *uploadTestRepo) Delete(id string) error                      { return nil }
func (r *uploadTestRepo) CreateSubmission(s *form.Submission) error { return nil }
func (r *uploadTestRepo) GetSubmission(id string) (*form.Submission, error) {
	return nil, nil
}
func (r *uploadTestRepo) ListSubmissions(formID string, opts form.ListSubmissionsOptions) ([]*form.Submission, int64, error) {
	return nil, 0, nil
}
func (r *uploadTestRepo) DeleteSubmission(id string) error { return nil }

var errNotFound = errors.New("not found")

func newUploadTestServer(t *testing.T, store *storage.FileStore, repo *uploadTestRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterUploadRoutes(r, NewUploadHandler(store, repo))
	return r
}

func multipartUpload(t *testing.T, field, filename string, content []byte, mime string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	var p interface{ Write([]byte) (int, error) }
	var err error
	if mime != "" {
		h := make(map[string][]string)
		h["Content-Disposition"] = []string{`form-data; name="file"; filename="` + filename + `"`}
		h["Content-Type"] = []string{mime}
		p, err = w.CreatePart(h)
	} else {
		p, err = w.CreateFormFile("file", filename)
	}
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := p.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	req := httptest.NewRequest("POST", "/form/upload/ctx1/contact?field="+field, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestUpload_StoresFileAndReturnsToken(t *testing.T) {
	store, _ := storage.New(t.TempDir())
	repo := &uploadTestRepo{bySlug: map[string]*form.Form{
		"ctx1/contact": {
			ID: "f1", ContextID: "ctx1", Slug: "contact", Name: "Contact",
			Fields: []form.FormField{{Slug: "resume", Type: "file", Label: "Resume"}},
		},
	}}
	r := newUploadTestServer(t, store, repo)

	req := multipartUpload(t, "resume", "resume.pdf", []byte("pdf-bytes"), "application/pdf")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	token, _ := body["token"].(string)
	if token == "" || !strings.HasPrefix(token, "ctx1/") {
		t.Fatalf("unexpected token %q", token)
	}
	if body["name"] != "resume.pdf" || body["type"] != "application/pdf" {
		t.Fatalf("unexpected body %v", body)
	}
	if size, _ := body["size"].(float64); size != float64(len("pdf-bytes")) {
		t.Fatalf("unexpected size %v", body["size"])
	}

	// Stored file must be readable with its meta sidecar.
	meta, err := store.Meta(token)
	if err != nil {
		t.Fatalf("Meta: %v", err)
	}
	if meta.Name != "resume.pdf" || meta.Type != "application/pdf" {
		t.Fatalf("unexpected meta %+v", meta)
	}
	f, err := store.Open(token)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	f.Close()
}

func TestUpload_RejectsDisallowedType(t *testing.T) {
	store, _ := storage.New(t.TempDir())
	repo := &uploadTestRepo{bySlug: map[string]*form.Form{
		"ctx1/contact": {
			ID: "f1", ContextID: "ctx1", Slug: "contact", Name: "Contact",
			Fields: []form.FormField{{
				Slug: "resume", Type: "file", Label: "Resume",
				Validation: map[string]interface{}{"accept": []interface{}{"application/pdf"}},
			}},
		},
	}}
	r := newUploadTestServer(t, store, repo)

	req := multipartUpload(t, "resume", "virus.exe", []byte("x"), "application/octet-stream")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpload_RejectsOversizedFile(t *testing.T) {
	store, _ := storage.New(t.TempDir())
	repo := &uploadTestRepo{bySlug: map[string]*form.Form{
		"ctx1/contact": {
			ID: "f1", ContextID: "ctx1", Slug: "contact", Name: "Contact",
			Fields: []form.FormField{{
				Slug: "resume", Type: "file", Label: "Resume",
				Validation: map[string]interface{}{"max_size": float64(10)},
			}},
		},
	}}
	r := newUploadTestServer(t, store, repo)

	req := multipartUpload(t, "resume", "big.pdf", bytes.Repeat([]byte("a"), 100), "application/pdf")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpload_FormNotFound(t *testing.T) {
	store, _ := storage.New(t.TempDir())
	r := newUploadTestServer(t, store, &uploadTestRepo{bySlug: map[string]*form.Form{}})

	req := multipartUpload(t, "resume", "a.pdf", []byte("x"), "application/pdf")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpload_MissingFilePart(t *testing.T) {
	store, _ := storage.New(t.TempDir())
	repo := &uploadTestRepo{bySlug: map[string]*form.Form{
		"ctx1/contact": {ID: "f1", ContextID: "ctx1", Slug: "contact", Name: "C",
			Fields: []form.FormField{{Slug: "resume", Type: "file"}}},
	}}
	r := newUploadTestServer(t, store, repo)

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("other", "value"); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	w.Close()
	req := httptest.NewRequest("POST", "/form/upload/ctx1/contact?field=resume", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}